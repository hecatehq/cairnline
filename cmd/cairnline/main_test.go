package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestCommand_VersionFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "default", args: []string{"run", ".", "-version"}, want: "cairnline 0.0.0-dev\n"},
		{name: "stamped", args: []string{"run", "-ldflags", "-X main.version=v0.1.2-test", ".", "-version"}, want: "cairnline v0.1.2-test\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "go", tt.args...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			output, err := cmd.Output()
			if err != nil {
				t.Fatalf("go %s error = %v stderr=%s", strings.Join(tt.args, " "), err, stderr.String())
			}
			if got := string(output); got != tt.want {
				t.Fatalf("version output = %q, want %q", got, tt.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestCommand_StandaloneMCPPullSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", ".", "-db", filepath.Join(t.TempDir(), "cairnline.db"))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe() error = %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() error = %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	waited := false
	t.Cleanup(func() {
		if waited {
			return
		}
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	request := newMCPCommandClient(t, stdin, scanner, &stderr)

	request.raw(1, "initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"clientInfo": map[string]any{
			"name":    "cairnline-command-smoke",
			"version": "test",
		},
	})
	projectText := request.toolText(2, "projects.create", map[string]any{
		"name":        "Standalone MCP smoke",
		"description": "Prove the command can coordinate a pull assignment.",
	})
	projectID := mustExtractMCPID(t, projectText, `Created project (proj_[^:\s]+)`)
	roleText := request.toolText(3, "roles.create", map[string]any{
		"project_id":             projectID,
		"name":                   "Reviewer",
		"description":            "Reviews evidence before closeout.",
		"instructions":           "Check that the handoff is understandable.",
		"default_execution_mode": "mcp_pull",
		"default_skill_ids":      []string{"review"},
	})
	roleID := mustExtractMCPID(t, roleText, `Created role (role_[^:\s]+)`)
	workText := request.toolText(4, "work_items.create", map[string]any{
		"project_id":        projectID,
		"title":             "Review standalone MCP flow",
		"brief":             "Claim the assignment, inspect context, record evidence, and complete it.",
		"owner_role_id":     roleID,
		"reviewer_role_ids": []string{roleID},
	})
	workItemID := mustExtractMCPID(t, workText, `Created work item (work_[^:\s]+)`)
	assignmentText := request.toolText(5, "assignments.create", map[string]any{
		"project_id":         projectID,
		"work_item_id":       workItemID,
		"role_id":            roleID,
		"execution_mode":     "mcp_pull",
		"desired_agent_kind": "any",
		"skill_ids":          []string{"review"},
	})
	assignmentID := mustExtractMCPID(t, assignmentText, `Created assignment (asgn_[^:\s]+)`)

	nextText := request.toolText(6, "assignments.next", map[string]any{
		"project_id": projectID,
		"agent_kind": "any",
		"skill_ids":  []string{"review"},
	})
	if !strings.Contains(nextText, assignmentID) {
		t.Fatalf("assignments.next text = %q, want assignment %s", nextText, assignmentID)
	}
	claimResponse := request.raw(7, "tools/call", map[string]any{
		"name": "assignments.claim",
		"arguments": map[string]any{
			"project_id":    projectID,
			"assignment_id": assignmentID,
			"claimed_by":    "standalone-smoke-agent",
		},
	})
	var claimResult struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent struct {
			Claim *struct {
				ID string `json:"id"`
			} `json:"claim"`
		} `json:"structuredContent"`
	}
	if err := json.Unmarshal(claimResponse.Result, &claimResult); err != nil {
		t.Fatalf("claim response did not unmarshal: %v\n%s", err, string(claimResponse.Result))
	}
	if len(claimResult.Content) == 0 || claimResult.StructuredContent.Claim == nil {
		t.Fatalf("claim response missing content or lease: %s", string(claimResponse.Result))
	}
	claimText := claimResult.Content[0].Text
	claimID := claimResult.StructuredContent.Claim.ID
	if !strings.Contains(claimText, "Claimed assignment "+assignmentID+" by standalone-smoke-agent") {
		t.Fatalf("claim text = %q, want claimed assignment", claimText)
	}
	contextText := request.toolText(8, "assignments.context", map[string]any{
		"project_id":    projectID,
		"assignment_id": assignmentID,
	})
	if !strings.Contains(contextText, "Work item: Review standalone MCP flow") || !strings.Contains(contextText, "Role: Reviewer") {
		t.Fatalf("assignment context text = %q, want work item and role context", contextText)
	}
	evidenceText := request.toolText(9, "evidence.record", map[string]any{
		"project_id":    projectID,
		"work_item_id":  workItemID,
		"assignment_id": assignmentID,
		"title":         "Smoke evidence",
		"body":          "The standalone command accepted the pull assignment lifecycle.",
		"locator":       "notes://standalone-mcp-smoke",
		"source_kind":   "note",
		"external_id":   "standalone-mcp-smoke",
		"provider":      "local",
		"trust_label":   "test_evidence",
	})
	mustExtractMCPID(t, evidenceText, `Recorded evidence (ev_[^:\s]+)`)
	completeText := request.toolText(10, "assignments.complete", map[string]any{
		"project_id":    projectID,
		"assignment_id": assignmentID,
		"claim_id":      claimID,
		"status":        "completed",
		"execution_ref": map[string]any{"run_id": "standalone-smoke-run"},
	})
	if !strings.Contains(completeText, "Updated assignment "+assignmentID+": completed") {
		t.Fatalf("complete text = %q, want completed assignment", completeText)
	}
	readinessText := request.toolText(11, "work_items.closeout_readiness", map[string]any{
		"project_id":   projectID,
		"work_item_id": workItemID,
	})
	if !strings.Contains(readinessText, "Closeout readiness") || !strings.Contains(readinessText, "ready") || !strings.Contains(readinessText, "Assignments: 1/1 complete") {
		t.Fatalf("closeout readiness text = %q, want ready closeout", readinessText)
	}

	if err := stdin.Close(); err != nil {
		t.Fatalf("Close(stdin) error = %v", err)
	}
	err = cmd.Wait()
	waited = true
	if err != nil {
		t.Fatalf("Wait() error = %v stderr=%s", err, stderr.String())
	}
}

type mcpCommandClient struct {
	t       *testing.T
	stdin   io.Writer
	scanner *bufio.Scanner
	stderr  *bytes.Buffer
}

func newMCPCommandClient(t *testing.T, stdin io.Writer, scanner *bufio.Scanner, stderr *bytes.Buffer) mcpCommandClient {
	t.Helper()
	return mcpCommandClient{t: t, stdin: stdin, scanner: scanner, stderr: stderr}
}

func (c mcpCommandClient) raw(id int, method string, params any) mcpSmokeResponse {
	c.t.Helper()
	request := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		request["params"] = params
	}
	body, err := json.Marshal(request)
	if err != nil {
		c.t.Fatalf("Marshal request %d error = %v", id, err)
	}
	if _, err := fmt.Fprintln(c.stdin, string(body)); err != nil {
		c.t.Fatalf("write request %d error = %v stderr=%s", id, err, c.stderr.String())
	}
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			c.t.Fatalf("read response %d error = %v stderr=%s", id, err, c.stderr.String())
		}
		c.t.Fatalf("read response %d got EOF stderr=%s", id, c.stderr.String())
	}
	line := c.scanner.Text()
	var response mcpSmokeResponse
	if err := json.Unmarshal([]byte(line), &response); err != nil {
		c.t.Fatalf("response %d did not unmarshal: %v\n%s", id, err, line)
	}
	if response.Error != nil {
		c.t.Fatalf("response %d error = %+v stderr=%s", id, *response.Error, c.stderr.String())
	}
	return response
}

func (c mcpCommandClient) toolText(id int, name string, args map[string]any) string {
	c.t.Helper()
	response := c.raw(id, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		c.t.Fatalf("tool response %d did not unmarshal: %v\n%s", id, err, string(response.Result))
	}
	if len(result.Content) == 0 {
		c.t.Fatalf("tool response %d has no text content: %s", id, string(response.Result))
	}
	return result.Content[0].Text
}

type mcpSmokeResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func mustExtractMCPID(t *testing.T, text, pattern string) string {
	t.Helper()
	match := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if len(match) != 2 {
		t.Fatalf("text %q did not match %s", text, pattern)
	}
	return match[1]
}
