package app

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/hecatehq/cairnline/internal/core"
	"github.com/hecatehq/cairnline/internal/mcp"
)

// projections stamp a fresh created_at per call, so normalize timestamps before
// comparing two independent calls for structural equality.
var timestampField = regexp.MustCompile(`"(created_at|updated_at)":"[^"]*"`)

func normalizeTimestamps(raw []byte) string {
	return timestampField.ReplaceAllString(string(raw), `"$1":"T"`)
}

// projectStatusToolNames are the projection tools the Project Status app backs.
var projectStatusToolNames = []string{
	"projects.health",
	"projects.operations_brief",
	"projects.activity",
}

func resultBytes(t *testing.T, response string) []byte {
	t.Helper()
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(response), &envelope); err != nil {
		t.Fatalf("response did not unmarshal: %v\n%s", err, response)
	}
	return envelope.Result
}

// Tagging the projection tools with the Project Status app must change only the
// tool descriptor's _meta, never the tool-call result. This compares each
// tagged tool's call result byte-for-byte against a reference server that
// registers the same handlers without the app tag.
func TestProjectStatusApp_TagLeavesToolResultsUnchanged(t *testing.T) {
	service := core.NewService(core.NewMemoryStore())
	project, err := service.CreateProject(context.Background(), core.Project{Name: "Status demo"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	tagged := NewServer(service, "test")

	// Reference server: same handlers over the same service, but no app _meta.
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: mcp.BoolPtr(true)}
	schema := json.RawMessage(`{"type":"object","properties":{"project_id":{"type":"string","minLength":1}},"required":["project_id"]}`)
	reference := mcp.NewServer("cairnline", "test", "")
	reference.RegisterTool(mcp.Tool{Name: "projects.health", InputSchema: schema, Annotations: readOnly}, projectHealth(service))
	reference.RegisterTool(mcp.Tool{Name: "projects.operations_brief", InputSchema: schema, Annotations: readOnly}, projectOperationsBrief(service))
	reference.RegisterTool(mcp.Tool{Name: "projects.activity", InputSchema: schema, Annotations: readOnly}, projectActivity(service))

	for _, name := range projectStatusToolNames {
		call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":{"project_id":"` + project.ID + `"}}}`
		gotResult := resultBytes(t, appServerResponse(t, tagged, call))
		wantResult := resultBytes(t, appServerResponse(t, reference, call))
		if normalizeTimestamps(gotResult) != normalizeTimestamps(wantResult) {
			t.Fatalf("%s call result changed by app tag:\n got=%s\nwant=%s", name, gotResult, wantResult)
		}
		// Guard against the tag leaking into the call result itself.
		if strings.Contains(string(gotResult), "resourceUri") {
			t.Fatalf("%s call result unexpectedly carries the app resourceUri: %s", name, gotResult)
		}
	}
}

// The three projection tool descriptors must carry _meta.ui.resourceUri pointing
// at the Project Status app.
func TestProjectStatusApp_ProjectionToolsCarryAppMeta(t *testing.T) {
	server := NewServer(core.NewService(core.NewMemoryStore()), "test")
	out := appServerResponse(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	var envelope struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("tools/list did not unmarshal: %v\n%s", err, out)
	}
	tagged := map[string]bool{}
	for _, tool := range envelope.Result.Tools {
		ui, ok := tool.Meta["ui"].(map[string]any)
		if !ok {
			continue
		}
		if ui["resourceUri"] == projectStatusAppURI {
			tagged[tool.Name] = true
		}
	}
	for _, name := range projectStatusToolNames {
		if !tagged[name] {
			t.Fatalf("tool %s missing _meta.ui.resourceUri = %s", name, projectStatusAppURI)
		}
	}
}
