package app

import _ "embed"

// projectStatusHTML is the self-contained Project Status view, built from
// internal/app/views (bun run build) and committed to dist/. Embedding keeps the
// server pure Go at runtime: no JS toolchain is needed for `go build` or tests.
//
//go:embed views/dist/project-status.html
var projectStatusHTML string

// projectStatusAppURI is the ui:// resource URI for the Project Status view. The
// projects.health, projects.operations_brief, and projects.activity tools tag
// their descriptors with it via uiAppMeta.
const projectStatusAppURI = "ui://cairnline/project-status"

// ProjectStatusApp returns the read-only Project Status MCP Apps view. It renders
// ProjectHealth, ProjectOperationsBrief, and ProjectActivity results; a single
// stateless view backs all three tools.
func ProjectStatusApp() UIApp {
	return UIApp{
		Name:        "project_status",
		URI:         projectStatusAppURI,
		Title:       "Project Status",
		Description: "Read-only project health, operations brief, and activity view.",
		HTML:        projectStatusHTML,
	}
}
