# Cairnline

Cairnline is an experimental local-first coordination server for human and AI
work.

It provides durable project identity, work coordination, context metadata,
evidence, reviews, handoffs, and memory-candidate concepts without assuming
that any specific agent host can be launched or supervised.

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
  skill metadata, evidence, handoffs, and memory candidates
- in-memory service for projects, profiles, roles, work items, assignments, and
  collaboration artifacts
- SQLite store for durable projects, profiles, roles, work items, assignments,
  skill metadata, and collaboration artifacts
- stdio MCP server with JSON-RPC framing
- MCP resources:
  - `cairnline://projects/{project_id}`
  - `cairnline://projects/{project_id}/work-items/{work_item_id}`
  - `cairnline://projects/{project_id}/assignments/{assignment_id}`
  - `cairnline://projects/{project_id}/assignments/{assignment_id}/launch-packet`
- MCP tools:
  - `projects.list`
  - `projects.create`
  - `projects.update`
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
  - `memory_candidates.create`
- assignment launch packets with resolved profile, execution-profile, skill,
  artifact, handoff, and memory-candidate metadata

Planned next:

- resource templates once the MCP transport grows that surface
- richer assignment context resources for memory candidates and review follow-up

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

Before Hecate can use Cairnline as its Projects backend, Cairnline needs stable
API/resource contracts, Hecate Projects API parity, migration/import-export
from Hecate's current store, permission and path-boundary review, and dogfood
coverage for at least one real Hecate project flow.

## Test

```sh
go test ./...
```

The public CI also runs:

```sh
go vet ./...
go test -race ./...
```
