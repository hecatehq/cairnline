package app

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hecatehq/cairnline/internal/core"
	"github.com/hecatehq/cairnline/internal/mcp"
)

// appServerResponse drives one JSON-RPC line through the MCP transport and
// returns the raw response, mirroring the wire-test style in tools_test.go.
func appServerResponse(t *testing.T, server *mcp.Server, line string) string {
	t.Helper()
	var out bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(line+"\n"), &out); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	return strings.TrimSpace(out.String())
}

// (a) The ui:// app resource is listed and read with the mcp-app profile mime
// type and a non-empty HTML body.
func TestRegisterApps_ResourceListedAndReadable(t *testing.T) {
	service := core.NewService(core.NewMemoryStore())
	server := NewServer(service, "test")

	list := appServerResponse(t, server, `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`)
	if !strings.Contains(list, projectStatusAppURI) {
		t.Fatalf("resources/list missing ui:// app resource: %s", list)
	}
	if !strings.Contains(list, `"mimeType":"`+UIAppMimeType+`"`) {
		t.Fatalf("resources/list missing mcp-app mime type: %s", list)
	}

	read := appServerResponse(t, server,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"`+projectStatusAppURI+`"}}`)
	var resp struct {
		Result mcp.ReadResourceResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(read), &resp); err != nil {
		t.Fatalf("resources/read did not unmarshal: %v\n%s", err, read)
	}
	if len(resp.Result.Contents) != 1 {
		t.Fatalf("resources/read contents = %+v, want one entry", resp.Result.Contents)
	}
	content := resp.Result.Contents[0]
	if content.MimeType != UIAppMimeType {
		t.Fatalf("resource mime type = %q, want %q", content.MimeType, UIAppMimeType)
	}
	if !strings.Contains(content.Text, "<!doctype html>") || !strings.Contains(content.Text, "Project Status") {
		t.Fatalf("resource text does not look like the Project Status view (len=%d)", len(content.Text))
	}
}

// (b) A tool descriptor tagged with uiAppMeta carries _meta.ui.resourceUri.
func TestUIAppMeta_TaggedToolDescriptorCarriesResourceURI(t *testing.T) {
	server := mcp.NewServer("test-server", "dev", "test")
	server.RegisterTool(mcp.Tool{
		Name:        "demo.status",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Meta:        uiAppMeta(projectStatusAppURI),
	}, func(context.Context, json.RawMessage) (mcp.CallToolResult, error) {
		return mcp.CallToolResult{Content: mcp.TextContent("ok")}, nil
	})

	out := appServerResponse(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if !strings.Contains(out, `"_meta":{"ui":{"resourceUri":"`+projectStatusAppURI+`"}}`) {
		t.Fatalf("tools/list missing _meta.ui.resourceUri: %s", out)
	}
}

// (c) The io.modelcontextprotocol/ui extension is advertised when an app is
// registered and absent when none are (reactive server behavior).
func TestRegisterApps_ExtensionReactiveToRegistration(t *testing.T) {
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`

	withApp := NewServer(core.NewService(core.NewMemoryStore()), "test")
	got := appServerResponse(t, withApp, initialize)
	if !strings.Contains(got, UIExtensionID) {
		t.Fatalf("initialize should advertise %s when an app is registered: %s", UIExtensionID, got)
	}
	if !strings.Contains(got, `"mimeTypes":["`+UIAppMimeType+`"]`) {
		t.Fatalf("declared extension missing mcp-app mime type: %s", got)
	}

	noApp := mcp.NewServer("test-server", "dev", "test")
	RegisterApps(noApp) // no apps -> nothing declared
	got = appServerResponse(t, noApp, initialize)
	if strings.Contains(got, "extensions") {
		t.Fatalf("initialize should omit extensions when no app is registered: %s", got)
	}
}

// (d) The embedded Project Status view is present and non-empty (go:embed wired).
func TestProjectStatusApp_EmbeddedHTMLNonEmpty(t *testing.T) {
	app := ProjectStatusApp()
	if app.URI != projectStatusAppURI {
		t.Fatalf("app URI = %q, want %q", app.URI, projectStatusAppURI)
	}
	if len(strings.TrimSpace(app.HTML)) == 0 {
		t.Fatal("ProjectStatusApp HTML is empty; go:embed not wired")
	}
	// The built view is a self-contained document with a default-deny CSP.
	if !strings.Contains(app.HTML, "<!doctype html>") {
		t.Fatalf("embedded HTML is not the built document")
	}
	if !strings.Contains(app.HTML, "Content-Security-Policy") {
		t.Fatalf("embedded HTML is missing its CSP meta tag")
	}
}
