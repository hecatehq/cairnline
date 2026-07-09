package app

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	cairnline "github.com/hecatehq/cairnline"
	"github.com/hecatehq/cairnline/internal/core"
	"github.com/hecatehq/cairnline/internal/mcp"
)

// TestMCPTools_ErrorCodesAcrossCentralPath drives real registered handlers and
// asserts the structured tool-error code the central path attaches for each
// failure class, including argument-decode failures that must classify as
// invalid.
func TestMCPTools_ErrorCodesAcrossCentralPath(t *testing.T) {
	ctx := context.Background()
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "dev")

	project, err := service.CreateProject(ctx, core.Project{Name: "Codes"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	tests := []struct {
		name      string
		request   string
		wantCode  string
		wantProse string
	}{
		{
			name:      "argument decode failure is invalid",
			request:   `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"projects.get","arguments":{"id":{"nested":true}}}}`,
			wantCode:  cairnline.ErrorCodeInvalid,
			wantProse: "invalid arguments",
		},
		{
			name:      "missing entity is not_found",
			request:   `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"projects.get","arguments":{"id":"proj_missing"}}}`,
			wantCode:  cairnline.ErrorCodeNotFound,
			wantProse: "not found",
		},
		{
			name:      "validation failure is invalid",
			request:   `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"work_items.create","arguments":{"project_id":"` + project.ID + `","title":""}}}`,
			wantCode:  cairnline.ErrorCodeInvalid,
			wantProse: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := server.Serve(ctx, strings.NewReader(tc.request+"\n"), &output); err != nil {
				t.Fatalf("Serve() error = %v", err)
			}
			var env struct {
				Result struct {
					Content           []mcp.Content         `json:"content"`
					StructuredContent *mcp.ToolErrorPayload `json:"structuredContent"`
					IsError           bool                  `json:"isError"`
				} `json:"result"`
			}
			if err := json.Unmarshal(output.Bytes(), &env); err != nil {
				t.Fatalf("decode response: %v\n%s", err, output.String())
			}
			if !env.Result.IsError {
				t.Fatalf("IsError = false, want true: %s", output.String())
			}
			if env.Result.StructuredContent == nil {
				t.Fatalf("StructuredContent = nil, want error payload: %s", output.String())
			}
			if got := env.Result.StructuredContent.Error.Code; got != tc.wantCode {
				t.Fatalf("code = %q, want %q (%s)", got, tc.wantCode, output.String())
			}
			if len(env.Result.Content) != 1 {
				t.Fatalf("content = %+v, want single prose block", env.Result.Content)
			}
			if env.Result.StructuredContent.Error.Message != env.Result.Content[0].Text {
				t.Fatalf("structured message %q != prose %q", env.Result.StructuredContent.Error.Message, env.Result.Content[0].Text)
			}
			if tc.wantProse != "" && !strings.Contains(env.Result.Content[0].Text, tc.wantProse) {
				t.Fatalf("prose %q missing %q", env.Result.Content[0].Text, tc.wantProse)
			}
		})
	}
}
