package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hecatehq/cairnline/internal/core"
)

// callToolEnvelope decodes the tools/call response far enough to assert on both
// the human prose and the structured error code.
type callToolEnvelope struct {
	Result struct {
		Content           []Content         `json:"content"`
		StructuredContent *ToolErrorPayload `json:"structuredContent"`
		IsError           bool              `json:"isError"`
	} `json:"result"`
}

func callToolThroughServer(t *testing.T, handler ToolHandler) callToolEnvelope {
	t.Helper()
	server := NewServer("test-server", "dev", "test")
	server.RegisterTool(Tool{
		Name:        "fake.tool",
		Description: "Fake tool.",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, handler)

	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":"call","method":"tools/call","params":{"name":"fake.tool","arguments":{}}}` + "\n",
	)
	var output bytes.Buffer
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	var env callToolEnvelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(output.String())), &env); err != nil {
		t.Fatalf("response did not unmarshal: %v\n%s", err, output.String())
	}
	return env
}

func TestServer_CallToolErrorCodes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
		wantMsg  string
	}{
		{
			name:     "not_found sentinel",
			err:      fmt.Errorf("root %q not found: %w", "x", core.ErrNotFound),
			wantCode: core.ErrorCodeNotFound,
			wantMsg:  `root "x" not found`,
		},
		{
			name:     "duplicate maps to already_exists",
			err:      errors.Join(core.ErrDuplicate, errors.New("project proj_1 already exists")),
			wantCode: core.ErrorCodeAlreadyExists,
			wantMsg:  "already exists",
		},
		{
			name:     "conflict sentinel",
			err:      errors.Join(core.ErrConflict, errors.New("assignment already claimed")),
			wantCode: core.ErrorCodeConflict,
			wantMsg:  "already claimed",
		},
		{
			name:     "invalid sentinel",
			err:      errors.Join(core.ErrInvalid, errors.New("title is required")),
			wantCode: core.ErrorCodeInvalid,
			wantMsg:  "title is required",
		},
		{
			name:     "unclassified defaults to internal",
			err:      errors.New("disk exploded"),
			wantCode: core.ErrorCodeInternal,
			wantMsg:  "disk exploded",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := callToolThroughServer(t, func(ctx context.Context, raw json.RawMessage) (CallToolResult, error) {
				return CallToolResult{}, tc.err
			})
			if !env.Result.IsError {
				t.Fatalf("IsError = false, want true")
			}
			if env.Result.StructuredContent == nil {
				t.Fatalf("StructuredContent = nil, want error payload")
			}
			if got := env.Result.StructuredContent.Error.Code; got != tc.wantCode {
				t.Fatalf("code = %q, want %q", got, tc.wantCode)
			}
			// The structured message and the human prose both carry the full
			// error text unchanged.
			if env.Result.StructuredContent.Error.Message != tc.err.Error() {
				t.Fatalf("structured message = %q, want %q", env.Result.StructuredContent.Error.Message, tc.err.Error())
			}
			if len(env.Result.Content) != 1 || env.Result.Content[0].Text != tc.err.Error() {
				t.Fatalf("prose content = %+v, want text %q", env.Result.Content, tc.err.Error())
			}
			if !strings.Contains(env.Result.Content[0].Text, tc.wantMsg) {
				t.Fatalf("prose content %q missing %q", env.Result.Content[0].Text, tc.wantMsg)
			}
		})
	}
}

func TestServer_CallToolSuccessHasNoErrorEnvelope(t *testing.T) {
	env := callToolThroughServer(t, func(ctx context.Context, raw json.RawMessage) (CallToolResult, error) {
		return CallToolResult{Content: TextContent("ok")}, nil
	})
	if env.Result.IsError {
		t.Fatalf("IsError = true, want false")
	}
	if env.Result.StructuredContent != nil {
		t.Fatalf("StructuredContent = %+v, want nil on success", env.Result.StructuredContent)
	}
	if len(env.Result.Content) != 1 || env.Result.Content[0].Text != "ok" {
		t.Fatalf("content = %+v, want ok", env.Result.Content)
	}
}
