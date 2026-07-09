package app

import (
	"github.com/hecatehq/cairnline/internal/core"
	"github.com/hecatehq/cairnline/internal/mcp"
)

func NewServer(service *core.Service, version string) *mcp.Server {
	server := mcp.NewServer(
		"cairnline",
		version,
		"Local-first project coordination server for MCP-capable agents.",
	)
	RegisterTools(server, service, version)
	RegisterResources(server, service)
	RegisterApps(server, ProjectStatusApp())
	return server
}
