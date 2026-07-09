package app

import (
	"bytes"
	"embed"
)

// go:generate refreshes the embedded bundle from source. `go generate ./...`
// runs it from this package dir, so it enters views/ and rebuilds dist/. This is
// a convenience entrypoint; the hard gate is the guard test in
// status_app_bundle_test.go (`go test` fails if the bundle is missing or stale).
//
//go:generate bash -c "cd views && bun install --frozen-lockfile && bun run build"

// projectStatusViewFS embeds the built Project Status view directory. This is a
// directory embed (all:views/dist), not a single named file, so `go build`
// compiles on a clean source checkout where only dist/.gitkeep is present and
// the JS bundle has not been built: the committed placeholder satisfies the
// embed pattern. The built dist/project-status.html is gitignored and produced
// by `bun run build` (internal/app/views); CI and release run that build before
// the Go build so the real bundle is embedded. A single-file embed of an absent
// file is a hard compile error, which is why the directory pattern is used.
//
//go:embed all:views/dist
var projectStatusViewFS embed.FS

// projectStatusViewPath is the built single-file view inside the embedded dir.
const projectStatusViewPath = "views/dist/project-status.html"

// projectStatusAppURI is the ui:// resource URI for the Project Status view. The
// projects.health, projects.operations_brief, and projects.activity tools tag
// their descriptors with it via uiAppMeta.
const projectStatusAppURI = "ui://cairnline/project-status"

// projectStatusFallbackHTML is served when the JS bundle has not been built
// (only dist/.gitkeep is embedded, e.g. after a clean `go install`). It is a
// self-contained, valid MCP-app document under the EXACT same strict, no-eval
// CSP as the built view, telling the operator to build it. Keeping it non-empty
// and CSP-tagged also keeps the embed-wired tests green on a source-only
// checkout with no JS toolchain.
const projectStatusFallbackHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta
      http-equiv="Content-Security-Policy"
      content="default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'"
    />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Project Status</title>
    <style>
      body {
        margin: 0;
        padding: 24px;
        font: 14px/1.5 system-ui, -apple-system, Segoe UI, Roboto, sans-serif;
        color: #1b1f24;
        background: #ffffff;
      }
      @media (prefers-color-scheme: dark) {
        body {
          color: #e6e9ee;
          background: #14171c;
        }
      }
      code {
        font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      }
    </style>
  </head>
  <body>
    <h1>Project Status</h1>
    <p>This view has not been built yet.</p>
    <p>Run <code>bun run build</code> in <code>internal/app/views</code> to build the bundle.</p>
  </body>
</html>
`

// projectStatusHTML returns the embedded Project Status view, or the built-in
// "not built" fallback when the bundle is absent (source checkout without a JS
// build). Reading from the embedded FS keeps the runtime pure Go: no JS
// toolchain is needed for `go build` or `go test`.
func projectStatusHTML() string {
	data, err := projectStatusViewFS.ReadFile(projectStatusViewPath)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return projectStatusFallbackHTML
	}
	return string(data)
}

// ProjectStatusApp returns the read-only Project Status MCP Apps view. It renders
// ProjectHealth, ProjectOperationsBrief, and ProjectActivity results; a single
// stateless view backs all three tools.
func ProjectStatusApp() UIApp {
	return UIApp{
		Name:        "project_status",
		URI:         projectStatusAppURI,
		Title:       "Project Status",
		Description: "Read-only project health, operations brief, and activity view.",
		HTML:        projectStatusHTML(),
	}
}
