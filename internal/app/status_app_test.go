package app

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hecatehq/cairnline/internal/core"
	"github.com/hecatehq/cairnline/internal/mcp"
)

// updateGolden regenerates the committed status-app golden fixtures from the
// current projection output: `go test ./internal/app -run TestProjectStatusApp -update`.
var updateGolden = flag.Bool("update", false, "update status-app golden fixtures")

// projections stamp a fresh created_at per call, so normalize timestamps before
// comparing against the golden fixtures for structural equality.
var timestampField = regexp.MustCompile(`"(created_at|updated_at)":"[^"]*"`)

func normalizeTimestamps(raw []byte) string {
	return timestampField.ReplaceAllString(string(raw), `"$1":"T"`)
}

// normalizeResult stamps the projection output into a deterministic form:
// per-call timestamps become "T" and the generated project id becomes the
// PROJECT_ID placeholder the golden fixtures use.
func normalizeResult(raw []byte, projectID string) string {
	return strings.ReplaceAll(normalizeTimestamps(raw), projectID, "PROJECT_ID")
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

// goldenFiles maps each projection tool to its committed golden fixture. The
// fixtures capture the exact text + structuredContent a call returns for a
// freshly created project, normalized (timestamps -> "T", project id ->
// PROJECT_ID). They were generated from the projection output that predates the
// Project Status app tag (PR #80 only added tool-descriptor _meta, never touched
// the call result), so any future drift in a result body is caught here — not
// just the presence or absence of the app tag.
var goldenFiles = map[string]string{
	"projects.health":           "testdata/status_app/health.golden.json",
	"projects.operations_brief": "testdata/status_app/operations_brief.golden.json",
	"projects.activity":         "testdata/status_app/activity.golden.json",
}

// Tagging the projection tools with the Project Status app must change only the
// tool descriptor's _meta, never the tool-call result. This asserts each tagged
// tool's call result equals its pre-#80 golden fixture byte-for-byte (after
// deterministic normalization), so a change to the result body is caught, not
// only a change to the tag.
func TestProjectStatusApp_TagLeavesToolResultsUnchanged(t *testing.T) {
	service := core.NewService(core.NewMemoryStore())
	project, err := service.CreateProject(context.Background(), core.Project{Name: "Status demo"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	tagged := NewServer(service, "test")

	for _, name := range projectStatusToolNames {
		call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":{"project_id":"` + project.ID + `"}}}`
		gotResult := resultBytes(t, appServerResponse(t, tagged, call))
		got := normalizeResult(gotResult, project.ID)

		path := goldenFiles[name]
		if *updateGolden {
			if err := os.WriteFile(filepath.FromSlash(path), []byte(got+"\n"), 0o644); err != nil {
				t.Fatalf("update golden %s: %v", path, err)
			}
		}
		wantBytes, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil {
			t.Fatalf("read golden %s: %v", path, err)
		}
		want := strings.TrimRight(string(wantBytes), "\n")
		if got != want {
			t.Fatalf("%s call result drifted from golden %s:\n got=%s\nwant=%s\n(regenerate with -update after an intended change)", name, path, got, want)
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
