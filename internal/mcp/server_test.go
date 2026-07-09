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
	server.RegisterResourceTemplates([]ResourceTemplate{{
		URITemplate: "cairnline://projects/{project_id}",
		Name:        "project",
		MimeType:    "application/json",
	}})

	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n" +
			`{"jsonrpc":"2.0","id":3,"method":"resources/list"}` + "\n" +
			`{"jsonrpc":"2.0","id":4,"method":"resources/templates/list"}` + "\n" +
			`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"cairnline://projects/proj_1"}}` + "\n",
	)
	var output bytes.Buffer
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("responses = %q, want 5 responses", output.String())
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
	if !strings.Contains(responses["4"], `"uriTemplate":"cairnline://projects/{project_id}"`) {
		t.Fatalf("resources/templates/list response = %s", responses["4"])
	}
	var readResponse struct {
		Result ReadResourceResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(responses["5"]), &readResponse); err != nil {
		t.Fatalf("resources/read response did not unmarshal: %v\n%s", err, responses["5"])
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

func TestServer_InitializeDeclaresExtensions(t *testing.T) {
	server := NewServer("test-server", "dev", "test")
	server.DeclareExtension("io.modelcontextprotocol/ui", json.RawMessage(`{"version":"1"}`))

	out := singleResponse(t, server, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{"extensions":{"io.modelcontextprotocol/ui":{}}}}}`)
	if !strings.Contains(out, `"extensions":{`) || !strings.Contains(out, `"io.modelcontextprotocol/ui"`) {
		t.Fatalf("initialize result missing declared extension: %s", out)
	}
	if !strings.Contains(out, `"version":"1"`) {
		t.Fatalf("initialize result dropped extension config: %s", out)
	}
}

func TestServer_InitializeOmitsExtensionsByDefault(t *testing.T) {
	server := NewServer("test-server", "dev", "test")
	out := singleResponse(t, server, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`)
	if strings.Contains(out, `"extensions"`) {
		t.Fatalf("initialize result should omit extensions when none declared: %s", out)
	}
	if !strings.Contains(out, `"protocolVersion":"2025-11-25"`) {
		t.Fatalf("initialize result = %s", out)
	}
}

func TestServer_ToolDescriptorCarriesOutputSchemaAndMeta(t *testing.T) {
	server := NewServer("test-server", "dev", "test")
	server.RegisterTool(Tool{
		Name:         "demo.tool",
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`),
		Meta:         map[string]any{"category": "demo"},
	}, func(ctx context.Context, raw json.RawMessage) (CallToolResult, error) {
		return CallToolResult{Content: TextContent("ok")}, nil
	})

	out := singleResponse(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if !strings.Contains(out, `"outputSchema"`) {
		t.Fatalf("tools/list missing outputSchema: %s", out)
	}
	if !strings.Contains(out, `"_meta":{"category":"demo"}`) {
		t.Fatalf("tools/list missing tool _meta: %s", out)
	}
}

func TestServer_CallToolEmbeddedResourceAndBlob(t *testing.T) {
	server := NewServer("test-server", "dev", "test")
	server.RegisterTool(Tool{
		Name:        "demo.blob",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, func(ctx context.Context, raw json.RawMessage) (CallToolResult, error) {
		return CallToolResult{
			Content: []Content{EmbeddedResource(ResourceContent{
				URI:      "cairnline://demo/blob",
				MimeType: "application/octet-stream",
				Blob:     "aGVsbG8=",
			})},
			Meta: map[string]any{"trace": "abc123"},
		}, nil
	})

	out := singleResponse(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"demo.blob","arguments":{}}}`)
	if !strings.Contains(out, `"type":"resource"`) {
		t.Fatalf("tools/call missing embedded resource content: %s", out)
	}
	if !strings.Contains(out, `"blob":"aGVsbG8="`) {
		t.Fatalf("tools/call missing blob payload: %s", out)
	}
	if !strings.Contains(out, `"_meta":{"trace":"abc123"}`) {
		t.Fatalf("tools/call missing result _meta: %s", out)
	}
}

func TestServer_HandleMessage(t *testing.T) {
	server := NewServer("test-server", "dev", "test")

	resp, ok := server.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":7,"method":"ping"}`))
	if !ok {
		t.Fatal("ping should produce a response")
	}
	if !strings.Contains(string(resp), `"id":7`) {
		t.Fatalf("ping response = %s", resp)
	}

	if _, ok := server.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); ok {
		t.Fatal("notification should not produce a response")
	}
}

func singleResponse(t *testing.T, server *Server, line string) string {
	t.Helper()
	input := strings.NewReader(line + "\n")
	var output bytes.Buffer
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	return strings.TrimSpace(output.String())
}
