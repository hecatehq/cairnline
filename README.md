# Cairnline

Cairnline is an experimental local-first coordination server for human and AI
work.

It provides durable project identity, work coordination, context metadata,
evidence, reviews, handoffs, accepted project memory, and memory-candidate
concepts without assuming that any specific agent host can be launched or
supervised.

Initial status: early implementation. Cairnline is usable as an experimental
local MCP server, but its contracts are not stable yet.

## Goals

- Work for rootless planning/research/design projects and workspace-backed code
  projects.
- Expose project coordination over MCP stdio.
- Let agents pull/claim work through MCP instead of requiring push-based
  orchestration.
- Keep Hecate-specific runtime behavior out of the portable core.

## Current Slice

Implemented now:

- portable core types for projects, roles, profiles, work items, assignments,
  skill metadata, assignment-scoped evidence, reviews, handoffs with
  source/target refs, accepted memory, and memory candidates
- in-memory service for projects, profiles, roles, work items, assignments, and
  collaboration artifacts
- SQLite store for durable projects, profiles, roles, work items, assignments,
  skill metadata, and collaboration artifacts
- embeddable Go API for applications that want to use the coordination core
  directly instead of speaking MCP
- stdio MCP server with JSON-RPC framing
- MCP resources:
  - `cairnline://projects/{project_id}`
  - `cairnline://projects/{project_id}/work-items/{work_item_id}`
  - `cairnline://projects/{project_id}/work-items/{work_item_id}/closeout-readiness`
  - `cairnline://projects/{project_id}/assignments/{assignment_id}`
  - `cairnline://projects/{project_id}/assignments/{assignment_id}/launch-packet`
  - `cairnline://projects/{project_id}/memory-candidates/{memory_candidate_id}`
- MCP tools:
  - `projects.list`
  - `projects.create`
  - `projects.update`
  - `projects.activity`
  - `projects.operations_brief`
  - `profiles.list`
  - `profiles.create`
  - `profiles.update`
  - `execution_profiles.list`
  - `execution_profiles.create`
  - `execution_profiles.update`
  - `skills.list`
  - `skills.create`
  - `skills.update`
  - `skills.discover`
  - `roles.list`
  - `roles.create`
  - `roles.update`
  - `work_items.list`
  - `work_items.create`
  - `work_items.update`
  - `work_items.closeout_readiness`
  - `assignments.list`
  - `assignments.next`
  - `assignments.create`
  - `assignments.claim`
  - `assignments.update_status`
  - `assignments.context`
  - `assignments.launch_packet`
  - `assignments.complete`
  - `evidence.record`
  - `reviews.record`
  - `handoffs.create`
  - `memory_entries.list`
  - `memory_entries.get`
  - `memory_entries.create`
  - `memory_entries.update`
  - `memory_entries.delete`
  - `memory_candidates.list`
  - `memory_candidates.get`
  - `memory_candidates.create`
  - `memory_candidates.promote`
  - `memory_candidates.reject`
  - `memory_candidates.delete`
- assignment launch packets with resolved profile, execution-profile, skill,
  artifact, handoff, accepted-memory, and memory-candidate metadata
- read-only work-item closeout readiness summaries derived from assignment,
  evidence, review, and handoff metadata
- read-only project operations briefs for attention routing across active
  assignments, blocked closeout, review follow-up, memory candidates, and open
  work
- read-only project activity projections grouped by active, blocked, completed,
  and recent assignment state

Planned next:

- resource templates once the MCP transport grows that surface

## Run

Install the command from source:

```sh
go install github.com/hecatehq/cairnline/cmd/cairnline@latest
```

Ephemeral in-memory state:

```sh
go run ./cmd/cairnline
```

Durable SQLite state:

```sh
go run ./cmd/cairnline -db ./cairnline.db
```

The server speaks MCP over newline-delimited JSON-RPC on stdin/stdout.

## Embedded Go API

Applications can embed Cairnline directly through the root Go package. Do not
import `internal/*` packages; they are private implementation details.

```go
package main

import (
	"context"
	"log"

	"github.com/hecatehq/cairnline"
)

func main() {
	ctx := context.Background()

	service, store, err := cairnline.NewSQLiteService(ctx, "cairnline.db")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	project, err := service.CreateProject(ctx, cairnline.Project{
		Name: "Example project",
	})
	if err != nil {
		log.Fatal(err)
	}

	_, err = service.CreateWorkItem(ctx, cairnline.WorkItem{
		ProjectID: project.ID,
		Title:     "Coordinate the next reviewable task",
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

For tests or short-lived tools, use the in-memory service:

```go
service := cairnline.NewMemoryService()
```

## MCP Client Config

Use a durable SQLite database for normal local use:

```json
{
  "mcpServers": {
    "cairnline": {
      "command": "cairnline",
      "args": ["-db", "/Users/alice/.local/share/cairnline/cairnline.db"]
    }
  }
}
```

For development from a checkout:

```json
{
  "mcpServers": {
    "cairnline-dev": {
      "command": "go",
      "args": [
        "run",
        "./cmd/cairnline",
        "-db",
        "/tmp/cairnline-dev.db"
      ],
      "cwd": "/path/to/cairnline"
    }
  }
}
```

## Hecate Integration Status

Cairnline is the intended portable extraction path for Hecate's Projects
coordination substrate, not a drop-in replacement yet.

Cairnline now has a public embeddable Go API, so Hecate can start integration
experiments without going through MCP. MCP remains the interoperability surface
for external agents and other hosts.

Before Hecate can replace its current Projects backend with Cairnline, the
following gates should be closed:

- stable API/resource contracts for the coordination model
- Hecate Projects API parity for current operator workflows
- migration/import-export from Hecate's current local store
- permission and path-boundary review for workspace-backed projects
- adapter between Hecate task/external-agent execution records and Cairnline
  assignment coordination records
- dogfood coverage for at least one real Hecate project flow, including work
  creation, assignment, evidence, review, handoff, memory candidate, and closeout

## Test

```sh
go test ./...
```

The public CI also runs:

```sh
go vet ./...
go test -race ./...
```
