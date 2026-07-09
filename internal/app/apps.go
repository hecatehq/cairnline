package app

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/hecatehq/cairnline/internal/mcp"
)

// MCP Apps (SEP-1865, extension id io.modelcontextprotocol/ui) let a host render
// an interactive HTML view for a tool's result. A UIApp bundles the view's
// ui:// resource URI with the self-contained HTML the host loads into a
// sandboxed iframe. The HTML is served over the standard resources/read surface
// with the mcp-app profile mime type, and tools opt in by tagging their
// descriptor with the app's resource URI (see uiAppMeta).
const (
	// UIExtensionID is the MCP Apps extension identifier a host negotiates.
	UIExtensionID = "io.modelcontextprotocol/ui"
	// UIAppMimeType is the exact media type (note: no space after ";") that
	// marks a resource as an MCP Apps HTML view.
	UIAppMimeType = "text/html;profile=mcp-app"
	// uiResourcePrefix guards the ui:// reader so it only claims app URIs and
	// leaves every other scheme to the other resource readers.
	uiResourcePrefix = "ui://"
)

// UIApp is a registered MCP Apps view: an HTML bundle served at URI, plus the
// list metadata a host sees before reading it.
type UIApp struct {
	// Name is a short, stable identifier for the app (resources/list "name").
	Name string
	// URI is the ui:// resource URI tools reference and hosts read.
	URI string
	// Title and Description are human-readable list metadata.
	Title       string
	Description string
	// HTML is the self-contained view document (all CSS/JS inlined).
	HTML string
}

// RegisterApps wires MCP Apps onto the server. It is reactive per the spec: with
// no apps it changes nothing, so the io.modelcontextprotocol/ui extension is not
// advertised. With one or more apps it registers a ui:// resource provider and
// reader that serve the app HTML, and declares the extension advertising the
// mcp-app html mime type so hosts can negotiate it during initialize.
func RegisterApps(server *mcp.Server, apps ...UIApp) {
	if len(apps) == 0 {
		return
	}
	ordered := make([]UIApp, len(apps))
	copy(ordered, apps)
	index := make(map[string]UIApp, len(apps))
	for _, app := range ordered {
		index[app.URI] = app
	}
	server.RegisterResourceProvider(uiAppProvider(ordered), uiAppReader(index))
	server.DeclareExtension(UIExtensionID, json.RawMessage(`{"mimeTypes":["`+UIAppMimeType+`"]}`))
}

// uiAppMeta builds the tool-descriptor _meta that links a tool to its app view.
// The nested path is _meta.ui.resourceUri per the MCP Apps spec; a host that
// negotiated the extension renders the referenced view for the tool's result,
// while other hosts ignore it and use the tool's text/structuredContent.
func uiAppMeta(resourceURI string) map[string]any {
	return map[string]any{"ui": map[string]any{"resourceUri": resourceURI}}
}

func uiAppProvider(apps []UIApp) mcp.ResourceProvider {
	return func(context.Context) ([]mcp.Resource, error) {
		resources := make([]mcp.Resource, 0, len(apps))
		for _, app := range apps {
			resources = append(resources, mcp.Resource{
				URI:         app.URI,
				Name:        app.Name,
				Title:       app.Title,
				Description: app.Description,
				MimeType:    UIAppMimeType,
			})
		}
		return resources, nil
	}
}

func uiAppReader(index map[string]UIApp) mcp.ResourceReader {
	return func(_ context.Context, uri string) (mcp.ReadResourceResult, bool, error) {
		// Claim only ui:// URIs so other readers keep serving their schemes.
		if !strings.HasPrefix(uri, uiResourcePrefix) {
			return mcp.ReadResourceResult{}, false, nil
		}
		app, ok := index[uri]
		if !ok {
			return mcp.ReadResourceResult{}, false, nil
		}
		return mcp.ReadResourceResult{
			Contents: []mcp.ResourceContent{{
				URI:      app.URI,
				MimeType: UIAppMimeType,
				Text:     app.HTML,
			}},
		}, true, nil
	}
}
