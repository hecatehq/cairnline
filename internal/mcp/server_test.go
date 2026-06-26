package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestServer_InitializeAndListTools(t *testing.T) {
	server := NewServer("test-server", "dev", "test")
	server.RegisterTool(Tool{
		Name:        "projects.list",
		Description: "List projects.",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, func(ctx context.Context, raw json.RawMessage) (CallToolResult, error) {
		return CallToolResult{Content: TextContent("ok")}, nil
	})
	server.RegisterResourceProvider(func(ctx context.Context) ([]Resource, error) {
		return []Resource{{
			URI:      "cairnline://projects/proj_1",
			Name:     "project/proj_1",
			MimeType: "application/json",
		}}, nil
	}, func(ctx context.Context, uri string) (ReadResourceResult, bool, error) {
		if uri != "cairnline://projects/proj_1" {
			return ReadResourceResult{}, false, nil
		}
		return ReadResourceResult{Contents: []ResourceContent{{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"id":"proj_1"}`,
		}}}, true, nil
	})

	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n" +
			`{"jsonrpc":"2.0","id":3,"method":"resources/list"}` + "\n" +
			`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"cairnline://projects/proj_1"}}` + "\n",
	)
	var output bytes.Buffer
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("responses = %q, want 4 responses", output.String())
	}
	responses := responsesByID(t, lines)
	if !strings.Contains(responses["1"], `"protocolVersion":"2025-11-25"`) {
		t.Fatalf("initialize response = %s", responses["1"])
	}
	if !strings.Contains(responses["1"], `"resources":{`) {
		t.Fatalf("initialize response = %s, want resources capability", responses["1"])
	}
	if !strings.Contains(responses["2"], `"projects.list"`) {
		t.Fatalf("tools/list response = %s", responses["2"])
	}
	if !strings.Contains(responses["3"], "cairnline://projects/proj_1") {
		t.Fatalf("resources/list response = %s", responses["3"])
	}
	var readResponse struct {
		Result ReadResourceResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(responses["4"]), &readResponse); err != nil {
		t.Fatalf("resources/read response did not unmarshal: %v\n%s", err, responses["4"])
	}
	if len(readResponse.Result.Contents) != 1 || readResponse.Result.Contents[0].Text != `{"id":"proj_1"}` {
		t.Fatalf("resources/read response = %+v, want project text", readResponse.Result.Contents)
	}
}

func TestServer_CallTool(t *testing.T) {
	server := NewServer("test-server", "dev", "test")
	server.RegisterTool(Tool{
		Name:        "projects.list",
		Description: "List projects.",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, func(ctx context.Context, raw json.RawMessage) (CallToolResult, error) {
		return CallToolResult{Content: TextContent("No projects yet.")}, nil
	})

	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":"call","method":"tools/call","params":{"name":"projects.list","arguments":{}}}` + "\n",
	)
	var output bytes.Buffer
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !strings.Contains(output.String(), "No projects yet.") {
		t.Fatalf("tools/call response = %s", output.String())
	}
}

func responsesByID(t *testing.T, lines []string) map[string]string {
	t.Helper()
	responses := make(map[string]string, len(lines))
	for _, line := range lines {
		var response Response
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatalf("response %q did not unmarshal: %v", line, err)
		}
		if response.ID == nil {
			t.Fatalf("response %q has no id", line)
		}
		responses[string(*response.ID)] = line
	}
	return responses
}
