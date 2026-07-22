# Agent Host Integration

Cairnline is a coordination server, not an agent runtime. It stores durable
project state and exposes it over MCP so an operator, human worker, or AI agent
host can coordinate work without Cairnline knowing how that host launches,
prompts, sandboxes, or supervises agents.

Core rule:

```text
Assignment is coordination. Execution is capability-dependent.
```

## Host Responsibilities

An integrating host remains responsible for:

- selecting providers, models, local runtimes, and human workers;
- mapping project roles to host-specific agents, subagents, assistants, chats,
  tasks, or manual queues;
- enforcing tools, writes, network, filesystem, approval, and sandbox policy;
- protecting credentials, cookies, login sessions, secrets, and private
  host-memory;
- creating workspaces, branches, and worktrees when code work needs them;
- deciding whether it can launch/supervise an assignment or should leave it
  queued for manual/MCP pull.

Cairnline records intent, context metadata, evidence, reviews, handoffs, and
memory candidates. It never grants runtime authority.

## Concept Mapping

| Cairnline concept | What it means | Host mapping |
| --- | --- | --- |
| Project | Durable identity for a body of work. | Workspace, plan, repo, issue set, launch, research thread, or design effort. |
| Root | Optional filesystem/workspace metadata. | Local folder, checkout, worktree, mounted volume, or no mapping for rootless projects. |
| Project role | Responsibility needed by the project. | Host agent persona, subagent, custom agent, human role, review lane, or queue label. |
| Skill metadata | Capability/instruction metadata and provenance. | Optional hint to load host-native instructions; not a permission grant. |
| Work item | Reviewable unit of work. | Task, ticket, todo, issue, plan step, review item, or human checklist row. |
| Assignment | Work item + role + desired execution record. | Claimable job, task run, chat launch, external process, or manual work card. |
| Desired agent | Portable hint for who/what should claim. | Host-specific agent kind, preset, model family, human worker, or "any". |
| Evidence/review/handoff | Collaboration artifacts. | Run output, PR link, notes, screenshots, review verdicts, or next-agent packet. |
| Memory candidate | Proposed durable memory awaiting approval. | Host-specific memory review queue; explicit promotion stays operator-controlled. |

## Execution Ladder

Cairnline assignments can request one of four execution modes:

```text
manual
→ mcp_pull
→ external_adapter
→ orchestrated
```

- `manual`: a human or external process works from the assignment record.
- `mcp_pull`: a compatible MCP client polls, claims, reads context, records
  evidence, and completes work.
- `external_adapter`: an orchestrator may launch a non-MCP agent adapter.
- `orchestrated`: an orchestrator may supervise execution, retries, approvals,
  and runtime evidence.

The mode is a request, not authorization. A host may downgrade, ignore, or
reject a mode if its policy does not allow it.

Embedding hosts that let operators change a queued assignment's role, root,
execution mode, or desired-agent metadata should use
`Service.UpdateQueuedAssignment` with the exact coordination snapshot and
`updated_at` value that were read:

```go
replacement := assignment.Coordination()
replacement.RoleID = newRoleID
updated, err := service.UpdateQueuedAssignment(ctx, projectID, assignment.ID,
	cairnline.QueuedAssignmentUpdate{
		Expected:          assignment.Coordination(),
		ExpectedUpdatedAt: assignment.UpdatedAt,
		Replacement:       replacement,
	})
```

The update is compare-and-set: it preserves lifecycle/execution fields and
returns `ErrConflict` if another writer changed or claimed the assignment.
This prevents a stale editor from releasing a claim that may already have
started execution. Snapshot import remains an administrative/offline workflow
and is not exposed as a live whole-assignment mutation.

### Portable worker claims and host authority

Portable MCP workers use a fenced pre-start claim rather than relying on the
caller-chosen `claimed_by` label:

```text
queued ── claim ──> claimed (lease active) ── start ──> running (fence retained)
                       │
                       └── expiry + explicit recover ──> queued
```

1. `Service.ClaimAssignmentWithLease` returns an assignment whose `claim`
   contains a server-generated `id`, `acquired_at`, and `expires_at`.
2. While status is `claimed`, call `Service.RenewAssignmentClaim` before the
   expiry when provisioning or context preparation may take longer than the
   configured TTL. `WithAssignmentClaimLeaseTTL` changes the default five
   minutes for an embedding server; `coordination.capabilities` reports the
   effective value. Positive custom values are rounded up to whole seconds
   with a one-second minimum.
3. Pass the exact claim id to `PrepareAssignmentWithClaim`,
   `ReleaseAssignmentWithClaim`, `UpdateAssignmentStatusWithClaim`, and
   `CompleteAssignmentWithClaim`. Missing, expired, or superseded ids conflict.
4. When work advances out of `claimed`, Cairnline retires the reservation
   expiry but retains its id as a fencing generation for later worker writes.

Claim ids are concurrency values, not authentication credentials. They may be
stored in portable snapshots and returned in assignment reads. Hosts still
decide which principals may invoke these methods or their MCP tools. For
content-only MCP clients, `assignments.get` and `assignments.list` include
`claim_id` plus `claim_expires_at` while the reservation is active, or
`claim_fence` after work starts.

If a pre-start claim expires, it stays `claimed` until an authorized host or
operator calls `RecoverAssignmentClaim` with the exact expired id. Recovery
requeues it and clears prepared execution/context references. This operation is
explicit so the host can reconcile any resources it created during preparation.
`projects.operations_brief` and `projects.health` expose the stable
`recover_assignment_claim` action hint for this state.
It is never valid for `running`, `awaiting_approval`, or `awaiting_review` work:
Cairnline cannot determine that host execution is dead and never cancels or
requeues it automatically.

The original `ClaimAssignment`, `PrepareAssignment`, `ReleaseAssignment`,
`UpdateAssignmentStatus`, and `CompleteAssignment` methods remain
embedding-host authority surfaces for trusted reconciliation and existing
Hecate integration. They bypass worker fencing by design and must not be
exposed directly as agent tools. Embedders with custom `Store`
implementations must implement the new claim-lease methods before upgrading.

Breaking (alpha): the portable worker claim contract is now fenced.
`assignments.prepare` and `assignments.release` take `claim_id` instead of
`claimed_by`; `assignments.update_status` and `assignments.complete` also
require the current `claim_id`. `assignments.claim` returns that id in both its
text and structured result. The public `Store` interface adds the seven leased
claim/renew/recover and fenced mutation methods, so custom stores must implement
them. Snapshot version 2 persists claim generations; version 1 imports remain
supported only as unleased host-authoritative history.

SQLite migration and v1 snapshot import preserve existing claimed rows as
unleased host-authoritative state; they do not invent an expiry or silently
make old work stealable. Before handing one of those rows to an MCP worker, a
trusted embedding host must reconcile/release it and let the worker claim it
again under the leased contract.

Handoff editors follow the same rule. Use `Service.PatchHandoff`,
`Service.UpdateHandoffStatus`, or `Service.DeleteHandoff` with the exact
handoff `UpdatedAt` value that was read. Cairnline advances the revision
monotonically and returns `ErrConflict` rather than overwriting a newer edit.
Snapshot restore uses a separate store-only path.

This is an intentional alpha contract break: the public Store/Service handoff
mutation signatures and the corresponding MCP tools now require a revision
token. Embedders with custom Store implementations must add the CAS, snapshot
restore, and atomic follow-up methods before upgrading.
Live assistant `update_handoff` proposal actions must likewise carry the exact
`handoff.updated_at` revision; historical proposal ledgers remain importable,
but a tokenless historical action cannot be applied.

When an operator explicitly accepts a handoff and asks for follow-up work, use
`Service.AcceptHandoffWithFollowUp` with the current handoff revision, the
`accept_and_ensure_follow_up` intent, and one stable idempotency key for that
operator action. Cairnline atomically ensures one linked portable assignment
and marks the handoff accepted. When no assignment is linked, Cairnline creates
one queued using defaults from the target work item and role; rootless work
remains rootless. An existing linked assignment is returned in its current
authoritative state, including a state that has already progressed or finished.
The command records coordination only: it never launches newly created work,
and hosts must make a separate policy-controlled launch or claim decision.
An exact-key retry preserves the original outcome and assignment identity while
returning current authoritative records. It conflicts if a later operator
decision closed or relinked the handoff.

Progress and completion transitions are also atomic, and terminal completion is
first-writer-wins. Once an assignment is completed, failed, or cancelled, later
progress or terminal writes return `ErrConflict`. Completion preserves an
existing `started_at`. A direct queued completion or failure records the
transition as both start and finish for compatibility; cancelling queued work
records only `completed_at`, so portable readers do not report that unstarted
work began.

## Recommended MCP Pull Flow

Hosts that want agent-neutral interoperability should start here:

```text
coordination.capabilities
assignments.next
assignments.claim
assignments.renew_claim (only while still claimed and nearing expiry)
assignments.context
assignments.launch_packet
evidence.record
assignments.complete
```

1. Call `coordination.capabilities` once during setup or health checks to learn
   the server contract and boundaries.
2. Poll `assignments.next` with the host's available kind and skill ids.
3. Claim one assignment with `assignments.claim` and retain the returned
   `structuredContent.claim.id` fencing value.
4. Read `assignments.context` and/or `assignments.launch_packet`. Both include
   the project's enabled durable memory entries.
5. Renew with `assignments.renew_claim` if the assignment remains `claimed` as
   expiry approaches. Pass the claim id to prepare, progress, and completion
   mutations.
6. Build the host-native prompt/run packet from the structured metadata.
7. Record evidence as the run produces useful proof.
8. Complete, fail, or cancel the assignment explicitly.

If the host crashes before work starts, another authorized host may explicitly
call `assignments.recover_claim` once the reservation expires, then claim the
queued assignment again. If it crashes after work starts, the executing host
must reconcile or resume through its `execution_ref`; claim expiry never makes
runtime work stealable.

## Execution Ref And Approval Signal

`execution_ref` is a structured, host-neutral record of the execution a host
attached to an assignment: `kind`, `task_id`, `run_id`, `session_id`,
`trace_id`, and `pending_approvals`. Cairnline never dereferences these values;
they exist so a coordination row survives migration between hosts without
losing which execution it referred to. A host maps its own identifiers onto
these slots — a task orchestrator fills `task_id`/`run_id`, a chat-first host
fills `session_id`, and any host can attach `trace_id` for observability.

Breaking (alpha): the pre-structured bare-string `execution_ref` is not
accepted anywhere — MCP arguments fail with an invalid-arguments error, and
stored rows or snapshots that still hold string refs fail decode with an error
telling the operator to rebuild or re-seed the store from the host's
authoritative data. Cairnline contracts are unstable alpha; no compatibility
shim is kept.

When execution pauses on a host-side human approval gate, set the assignment
status to `awaiting_approval` (optionally with `pending_approvals` in the ref)
and return it to `running` once the gate resolves. The status is portable
coordination state; the approval object, its policy, and its resolution stay
host-owned. Portable readers treat `awaiting_approval` as blocked-on-operator,
not as active execution.

## Tool Error Codes

Every MCP tool failure carries a stable, machine-readable code so a host can map
a coordination failure onto its own error taxonomy (for example an HTTP status)
without parsing human prose. On failure the tool result sets `isError: true`,
keeps the human-readable message in `content`, and adds a `structuredContent`
error envelope:

```json
{
  "error": {
    "code": "not_found",
    "message": "root \"x\" not found"
  }
}
```

`error.message` repeats the human-readable text; `error.code` is one of the
fixed values below. The catalog derives from Cairnline's typed store sentinels
plus one default. Hosts should branch on `error.code`, treat unknown codes as
`internal`, and never parse `error.message`.

| Code             | Meaning                                            | Suggested host HTTP status |
| ---------------- | -------------------------------------------------- | -------------------------- |
| `not_found`      | Referenced entity does not exist                   | 404                        |
| `invalid`        | Bad or missing input, including argument-decode and validation failures | 400 |
| `already_exists` | Id or uniqueness collision                         | 409                        |
| `conflict`       | Invalid state transition or claim race             | 409                        |
| `internal`       | Unexpected, unclassified server-side error         | 500                        |

A Go host embedding Cairnline can classify errors it raises itself and reuse the
same code constants from the public package
(`github.com/hecatehq/cairnline`): `ErrorCodeNotFound`, `ErrorCodeInvalid`,
`ErrorCodeAlreadyExists`, `ErrorCodeConflict`, `ErrorCodeInternal`, and the
`ClassifyErrorCode(err error) string` helper (which returns `""` for a nil
error and `internal` for anything unclassified).

## Role And Desired-Agent Mapping

Cairnline roles answer "what responsibility does this project need?" They are
not agent presets or permission bundles.

Good role names:

- Architect
- Implementer
- Reviewer
- Researcher
- Designer
- Release Manager
- Operator

The host maps role plus desired-agent metadata to its own runtime:

```text
Cairnline role + desired_agent + skill metadata
        ↓
host policy and operator settings
        ↓
host-native agent/task/chat/manual work
```

Examples:

- A coding agent host may map `role=Reviewer`, `skill_ids=["review"]` to its
  review subagent.
- A human-first host may show the same assignment in a "Review" queue.
- An orchestrator may map `desired_agent.kind="external_adapter"` to one of its
  installed adapters only after its own approvals pass.
- A rootless planning host may ignore roots entirely and use the work item
  brief plus memory/evidence metadata.

## Skill Metadata

Skill ids are hints. Cairnline discovery stores metadata such as title,
description, path, source refs, suggested tool names, and nullable permission
preferences. Cairnline does not read skill bodies into prompts, execute skill
code, install remote skills, or convert suggested permissions into authority.

Host behavior should be explicit:

- Load host-native skill bodies only if the host already trusts that skill
  source and its own policy allows it.
- Treat unknown skill ids as warnings or routing misses, not hard failures.
- Treat suggested tool/write/network permissions as review metadata.
- Keep local host instruction formats host-owned.

## Launch Packet Use

`assignments.launch_packet` is the best portable packet for a host preparing a
run. It should include:

- project identity and optional roots;
- work item and assignment metadata;
- role instructions;
- desired-agent and skill metadata;
- relevant evidence, reviews, handoffs, accepted memory, and memory candidates;
- warnings for missing, disabled, or conflicting coordination references.

Hosts should translate that packet into their own prompt format and enforce
their own token, privacy, and egress policy before sending anything to a model
or external process.

## Mounting The MCP Server

A host that embeds Cairnline in-process does not have to shell out to the stdio
binary to speak MCP. The root package builds the fully-registered server and
exposes a single per-message entry point, so the host can mount Cairnline's tool
and resource surface on whatever transport it already runs.

```go
service := cairnline.NewMemoryService()          // or NewSQLiteService(ctx, path)
server := cairnline.NewMCPServer(service, version)
// Custom transport: feed each inbound JSON-RPC message through HandleMessage.
resp, ok := server.HandleMessage(ctx, msg)       // ok == false for notifications (no response)
// Or run the built-in stdio loop:
// server.Serve(ctx, os.Stdin, os.Stdout)
// Optionally advertise a protocol extension during initialize:
// server.DeclareExtension("io.modelcontextprotocol/ui", nil)
```

`HandleMessage` processes one JSON-RPC message and returns the encoded response;
`ok` is false for notifications, which produce no reply. `Serve` owns the stdio
read/write loop and dispatches through the same path, so stdio behavior is
identical whichever entry point the host uses. `DeclareExtension` advertises an
optional protocol extension in the `initialize` capabilities under its extension
id; Cairnline declares none by default, so a host opts in explicitly.

## Manual And Non-Agent Clients

Cairnline should still be useful without an AI agent. A simple UI or script can:

- create rootless projects;
- create roles and work items;
- queue assignments for humans;
- record evidence and reviews;
- inspect closeout readiness;
- approve or reject memory candidates in a separate operator flow.

The same coordination state can later be claimed by an MCP-capable agent host
without changing the project model.

## MCP Apps (Interactive Views)

Cairnline ships optional interactive views using the MCP Apps extension
(`io.modelcontextprotocol/ui`, SEP-1865). A view is a self-contained HTML
document served as a `ui://` resource. A host that negotiates the extension can
render the view for a tool result; a host that does not simply uses the tool's
existing text and `structuredContent`. Views are additive and never change tool
call results.

### What a `ui://` app is

- A resource served over standard `resources/read` with mime type
  `text/html;profile=mcp-app` (note: no space after `;`). Its `text` is a single
  HTML document with all CSS and JS inlined — no external requests.
- It also appears in `resources/list`, so a host can prefetch it at connection
  setup.

Cairnline's first app is **Project Status** (`ui://cairnline/project-status`), a
read-only view that renders `projects.health`, `projects.operations_brief`, and
`projects.activity` results.

### How a tool opts in

A tool references its view through the tool descriptor's `_meta`:

```json
{
  "name": "projects.health",
  "_meta": { "ui": { "resourceUri": "ui://cairnline/project-status" } }
}
```

The nested `_meta.ui.resourceUri` path is the MCP Apps linkage. When the host
calls the tool, it may render the referenced view and deliver the tool result to
it. A host that ignores `_meta` still gets the unchanged text/`structuredContent`
response.

### Capability negotiation

The server is reactive: it advertises the extension in `initialize` capabilities
only when at least one app is registered, declaring the supported content type:

```json
{ "capabilities": { "extensions": {
  "io.modelcontextprotocol/ui": { "mimeTypes": ["text/html;profile=mcp-app"] }
} } }
```

### Host <-> view bridge

The host renders the view in a sandboxed iframe and drives it over JSON-RPC on
`window.postMessage`:

- The view posts `ui/initialize` and then `ui/notifications/initialized` on load.
- The host delivers the tool result as a `ui/notifications/tool-result`
  notification whose params are a standard `CallToolResult`; the view reads
  `params.structuredContent`.

### Sandbox / CSP posture

Cairnline's views declare a strict, default-deny CSP
(`default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'`) and
grant no external origins; `connect-src` stays denied because postMessage needs
no network. Hosts should still render every view in a sandboxed iframe and must
not loosen the declared origins.

### Single view / per-project rendering

A single view backs all three projection tools. Per the MCP Apps maturity
guidance the view holds no durable cross-turn state; it keeps only light
per-project state within a session so the health, operations-brief, and activity
results for one project accumulate into a single combined view. That state is
keyed on `project_id`: the first result carrying a different `project_id` resets
every section, so one project's results can never bleed into another's. The view
renders whatever tool result the host delivers and never persists beyond the
session.

### Authoring a view (contributors)

Views live in `internal/app/views` and are bundled — together with the official
`@modelcontextprotocol/ext-apps` SDK they use for the host bridge — to a single
self-contained HTML file (`bun install` then `bun run build`, ESM inlined into a
`<script type="module">`) that is committed under `dist/` and embedded with
`//go:embed`; the runtime and `go test` need no JS toolchain. Register a built
view with `app.RegisterApps` and tag its tools with the `_meta.ui.resourceUri`
helper. See `internal/app/views/README.md` for the build (including why the
bundle targets ESM rather than IIFE) and the headless render check.

## Security Checklist For Hosts

Before exposing Cairnline tools to an agent, a host should decide:

- Which mutating tools the agent may call.
- Whether assignments can be claimed automatically or require operator review.
- Which component renews pre-start claims, how it retains the non-secret claim
  id, and how it stops stale workers after a conflict.
- Whether evidence locators are only stored, rendered as text, or opened as
  links after scheme validation.
- Whether local roots are readable by the host, and under what path boundary.
- Whether skill metadata may trigger host-native instruction loading.
- How to reconcile prepared host resources before recovering an expired claim,
  and how to handle running work separately through host supervision.
- How to show operator confirmation for memory promotion and destructive
  project changes.
- Whether to render `ui://` app views, and if so, only inside a sandboxed
  iframe under the view's declared default-deny CSP without loosening origins.

## Minimal Integration Target

A minimal useful integration does not need orchestration. It only needs:

- `coordination.capabilities`
- `projects.list`
- `assignments.next`
- `assignments.claim`
- `assignments.renew_claim` when pre-start work approaches expiry
- `assignments.recover_claim` for explicit expired-claim recovery
- `assignments.context`
- `assignments.launch_packet`
- `evidence.record`
- `assignments.complete`

That is enough for an MCP-capable host to pull one queued assignment, do work
under its own policy, record proof, and hand control back to the operator.
