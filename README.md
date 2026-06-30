# Cairnline

Cairnline is an experimental local-first coordination server for human and AI
work.

It provides durable project identity, work coordination, context metadata,
collaboration artifacts, evidence, reviews, handoffs, accepted project memory,
and memory-candidate concepts without assuming that any specific agent host can
be launched or supervised.

Initial status: early implementation. Cairnline is usable as an experimental
local MCP server, but its contracts are not stable yet.

## Goals

- Work for rootless planning/research/design projects and workspace-backed code
  projects.
- Expose project coordination over MCP stdio.
- Let agents pull/claim work through MCP instead of requiring push-based
  orchestration.
- Keep Hecate-specific runtime behavior out of the portable core.

## Security Boundaries

Cairnline is local-first and single-operator by default. It stores coordination
state and exposes it over MCP, so clients should treat mutating tools as durable
state changes that need the same care as editing a project database.

Project roots are optional metadata until a feature explicitly needs local
files. Current skill discovery is the main local-read path: it reads bounded
guidance files from active roots, discovers local `SKILL.md` metadata, and skips
absolute paths, parent traversal, remote URLs, and hidden worktree folders. It
stores names, descriptions, provenance, suggested tool names, and nullable
permission hints without storing, injecting, executing, installing, or fetching
skill bodies. Skill metadata is not permission to enable tools, writes, network
access, approvals, or sandbox escapes.

Source locators, evidence locators, and evidence URLs are operator-provided
metadata. Cairnline stores them as-is and does not fetch or render them.
Clients must validate schemes before displaying a locator as a clickable link or
before opening/fetching it.

Assignments are coordination records. Claiming or reading an assignment does not
authorize an agent host to bypass its own sandboxing, approval policy, network
policy, credential handling, or logged-in session boundaries. Secrets, cookies,
provider credentials, and external-agent private memory are outside Cairnline's
core model.

Role references are durable coordination metadata rather than hard ownership.
Creating or updating an assignment validates the role at write time, but deleting
a role does not delete or block historical assignments that still carry that
role id. Context and launch-packet reads surface missing-role warnings so
operators can repair or preserve the historical record deliberately.

## Current Slice

Implemented now:

- portable core types for projects with roots/default root and
  profile/execution-profile default references, context source provenance
  metadata, roles with agent/execution-profile defaults,
  profiles, work items, assignments with lifecycle timestamps, skill metadata,
  generic collaboration artifacts, assignment-scoped evidence with
  source/provider/external-id metadata, Hecate-compatible structured review
  verdict/risk metadata, handoffs with source/target refs and status-transition
  timestamps, accepted memory, and memory candidates
- in-memory service for projects, profiles, roles, work items, assignments,
  assistant proposal records including project-root/default-root actions, and
  collaboration artifacts
- SQLite store for durable projects, profiles, roles, work items, assignments,
  skill metadata, assistant proposal records, and collaboration artifacts
- project skill discovery from `.agents/skills`, Hecate-compatible
  `.hecate/skills`, Cairnline-native `.cairnline/skills`, and enabled
  guidance-linked local skill roots; recognized guidance locators include
  `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.cursor/rules`,
  `.github/instructions`, `.devin/rules`, and `.windsurf/rules`; rediscovery
  refreshes discovered status, provenance, suggested tools, and permission
  hints while preserving operator-edited enabled/title/description and
  trust-label fields
- embeddable Go API for applications that want to use the coordination core
  directly instead of speaking MCP, including assignment metadata updates that
  preserve created time and claim ownership while validating work-item, role,
  root, profile, and execution-profile references, a narrow claimed-assignment
  release path for pre-dispatch retry cleanup, plus source-level context
  metadata create/update/delete helpers that avoid whole-project replacement
- embeddable snapshot export/import for migration rehearsals and bridge seeding;
  snapshots cover profiles, execution profiles, projects, skills, roles, work,
  assignments, artifacts, evidence, reviews, handoffs, memory entries,
  memory candidates, and assistant proposal records
- stdio MCP server with JSON-RPC framing
- MCP read tools return human-readable text plus `structuredContent` where a
  stable data shape exists, including core project/profile/role/work/assignment
  list surfaces, so compatible clients can avoid scraping text output
- MCP resources:
  - `cairnline://projects/{project_id}`
  - `cairnline://projects/{project_id}/work-items/{work_item_id}`
  - `cairnline://projects/{project_id}/work-items/{work_item_id}/closeout-readiness`
  - `cairnline://projects/{project_id}/assignments/{assignment_id}`
  - `cairnline://projects/{project_id}/assignments/{assignment_id}/launch-packet`
  - `cairnline://projects/{project_id}/memory-candidates/{memory_candidate_id}`
- MCP tools:
  - `projects.list`
  - `projects.get`
  - `projects.create`
  - `projects.update`
  - `projects.delete`
  - `projects.activity`
  - `projects.health`
  - `projects.operations_brief`
  - `projects.setup_readiness`
  - `roots.list`
  - `roots.create`
  - `roots.update`
  - `roots.delete`
  - `context_sources.list`
  - `context_sources.create`
  - `context_sources.update`
  - `context_sources.delete`
  - `assistant.propose`
  - `assistant.proposals.list`
  - `assistant.proposals.get`
  - `assistant.apply`
  - `profiles.list`
  - `profiles.create`
  - `profiles.update`
  - `profiles.delete`
  - `execution_profiles.list`
  - `execution_profiles.create`
  - `execution_profiles.update`
  - `execution_profiles.delete`
  - `skills.list`
  - `skills.create`
  - `skills.update`
  - `skills.discover`
  - `roles.list`
  - `roles.create`
  - `roles.update`
  - `roles.delete`
  - `work_items.list`
  - `work_items.get`
  - `work_items.create`
  - `work_items.update`
  - `work_items.delete`
  - `work_items.closeout_readiness`
  - `assignments.list`
  - `assignments.get`
  - `assignments.next`
  - `assignments.create`
  - `assignments.update`
  - `assignments.claim`
  - `assignments.release`
  - `assignments.update_status`
  - `assignments.context`
  - `assignments.launch_packet`
  - `assignments.complete`
  - `assignments.delete`
  - `artifacts.list`
  - `artifacts.get`
  - `artifacts.create`
  - `evidence.list`
  - `evidence.get`
  - `evidence.record`
  - `reviews.list`
  - `reviews.get`
  - `reviews.record`
  - `handoffs.create`
  - `handoffs.list`
  - `handoffs.get`
  - `handoffs.update`
  - `handoffs.update_status`
  - `handoffs.delete`
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
  generic-artifact, evidence, review, handoff, accepted-memory, and
  memory-candidate metadata
- read-only work-item closeout readiness summaries derived from assignment,
  evidence, review, and handoff metadata
- read-only project operations briefs for attention routing across active
  assignments, blocked closeout, review follow-up, memory candidates, and open
  work
- read-only project activity projections grouped by active, blocked, completed,
  and recent assignment state; queued assignments are attention items until
  claimed, while claimed/running/review assignments are active
- read-only project setup-readiness and health summaries for onboarding,
  context/profile/skill gaps, and bounded operator attention
- deterministic assistant proposal/apply tools with durable proposal records,
  proposal warnings, apply attempts, latest-result state, and repeat-apply
  protection for confirmed project-state mutations; applying a proposal can
  create queued assignment coordination records, but it does not launch or
  supervise agents
- snapshot and proposal-record imports preserve assistant ledger state without
  replaying proposal actions

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

Embedded hosts can rehearse migration through snapshots:

```go
snapshot, err := source.ExportSnapshot(ctx)
if err != nil {
	log.Fatal(err)
}

_, err = target.ImportSnapshot(ctx, snapshot)
if err != nil {
	log.Fatal(err)
}
```

Snapshot import is additive/upsert. It does not delete records that are absent
from the snapshot, does not replay assistant proposal actions, and is not
exposed as an MCP bulk mutation tool.

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

Hecate is one integration client, not the defining host. Hecate may provide a
native operator UI/UX and may embed Cairnline while the contracts settle, but
the stable distribution target remains a standalone `cairnline` binary/MCP
server that can be installed as an additional local tool by any compatible
agent host.

Before Hecate can replace its current Projects backend with Cairnline, the
following gates should be closed:

- stable API/resource contracts for the coordination model
- Hecate Projects API parity for current operator workflows
- Hecate-specific migration mapping from its current local store into
  Cairnline's portable snapshot format, plus rehearsal/rollback evidence
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
