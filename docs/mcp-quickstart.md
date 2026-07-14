# Cairnline MCP Quickstart

This guide exercises Cairnline as a standalone, agent-neutral MCP server. It
uses a rootless project so the flow works for planning, research, writing, ops,
or design work as well as code projects.

Cairnline stores coordination state. It does not launch agents, choose models,
grant tools, create worktrees, read secrets, or bypass any client-side sandbox.
Agents and hosts should treat `execution_mode`, `desired_agent_kind`, and
`skill_ids` as hints that they map to their own runtime policy.

## Start The Server

From a checkout:

```sh
go run ./cmd/cairnline -db /tmp/cairnline-quickstart.db
```

Or install the command and run it from anywhere:

```sh
go install github.com/hecatehq/cairnline/cmd/cairnline@latest
cairnline -version
cairnline -db "$HOME/.local/share/cairnline/cairnline.db"
```

The server speaks MCP over newline-delimited JSON-RPC on stdin/stdout. Most MCP
clients hide the JSON-RPC envelope behind a tool UI. The examples below show
the exact `tools/call` payloads so they can be replayed in tests, scripts, or a
raw stdio session.

## Agent-Neutral Pull Flow

The core workflow is:

```text
operator creates project
operator creates role
operator creates work item
operator queues assignment
compatible agent finds assignment
agent claims assignment
agent reads context
agent records evidence
agent completes assignment
```

Before creating records, hosts can ask Cairnline to describe its portable
contract:

```json
{"jsonrpc":"2.0","id":0,"method":"tools/call","params":{"name":"coordination.capabilities","arguments":{}}}
```

The response includes `structuredContent` with supported `execution_modes`,
`assignment_statuses`, skill metadata paths, the recommended MCP-pull flow, and
the runtime responsibilities that remain owned by the consuming agent host.

Generated IDs are returned in tool response text. Copy the `proj_...`,
`role_...`, `work_...`, and `asgn_...` IDs into later calls.

### 1. Create A Rootless Project

```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"projects.create","arguments":{"name":"Launch planning","description":"Coordinate the next public launch checklist."}}}
```

Optional workspace-backed projects can pass `roots`, but roots are metadata
only. Cairnline does not create folders or Git worktrees:

```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"projects.create","arguments":{"name":"Code review","roots":[{"id":"root_main","path":"/Users/alice/src/example","kind":"git","active":true}],"default_root_id":"root_main"}}}
```

### 2. Create A Project Role

Replace `PROJECT_ID` with the generated project id.

```json
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"roles.create","arguments":{"project_id":"PROJECT_ID","name":"Reviewer","description":"Reviews plans and evidence before closeout.","instructions":"Check that the work is understandable, sourced, and ready for operator approval.","default_execution_mode":"mcp_pull","default_skill_ids":["review"]}}}
```

Roles describe responsibility. They are not agent presets, provider settings,
model settings, or permission grants.

### 3. Create A Work Item

Replace `ROLE_ID` with the generated role id.

```json
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"work_items.create","arguments":{"project_id":"PROJECT_ID","title":"Review launch checklist","brief":"Check the current checklist, identify gaps, and record evidence for operator review.","priority":"normal","owner_role_id":"ROLE_ID","reviewer_role_ids":["ROLE_ID"]}}}
```

### 4. Queue An Assignment

Replace `WORK_ITEM_ID` with the generated work item id.

```json
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"assignments.create","arguments":{"project_id":"PROJECT_ID","work_item_id":"WORK_ITEM_ID","role_id":"ROLE_ID","execution_mode":"mcp_pull","desired_agent_kind":"any","skill_ids":["review"]}}}
```

`execution_mode` describes the desired coordination style:

- `manual`: a human/operator works the assignment outside MCP.
- `mcp_pull`: a compatible MCP client or agent polls, claims, and reports back.
- `external_adapter`: an orchestrator may launch a non-MCP adapter.
- `orchestrated`: an orchestrator may supervise execution.

Cairnline does not perform the launch for any of these modes.

#### Optional: edit the queued assignment safely

Read the assignment with `assignments.get`, then pass its exact coordination
fields and `updated_at` back as the expected snapshot. Only the replacement
fields may differ:

```json
{"jsonrpc":"2.0","id":"4-edit","method":"tools/call","params":{"name":"assignments.update","arguments":{"project_id":"PROJECT_ID","assignment_id":"ASSIGNMENT_ID","expected":{"work_item_id":"WORK_ITEM_ID","role_id":"ROLE_ID","root_id":"","execution_mode":"mcp_pull","desired_agent":{"kind":"any","skill_ids":["review"]}},"expected_updated_at":"ASSIGNMENT_UPDATED_AT","replacement":{"work_item_id":"WORK_ITEM_ID","role_id":"NEW_ROLE_ID","root_id":"","execution_mode":"manual","desired_agent":{"kind":"human","skill_ids":["review"]}}}}}
```

If the assignment was edited, claimed, or prepared after the read, the update
returns a conflict. Re-read before deciding whether the new state should be
changed. Breaking (alpha): `assignments.update` now uses this nested
compare-and-set shape and no longer changes lifecycle or execution fields.

#### Optional: accept a handoff and queue its follow-up atomically

Handoff edits, status changes, and deletes also require the exact
`updated_at` returned by `handoffs.get`. To accept a handoff and ensure one
follow-up assignment in a single durable operation, call:

Breaking (alpha): `handoffs.update`, `handoffs.update_status`, and
`handoffs.delete` now require `expected_updated_at`; stale or tokenless writes
fail closed instead of replacing newer handoff state.
The same guard applies to live assistant `update_handoff` actions: the embedded
handoff must carry the exact `updated_at` revision that was read.

```json
{"jsonrpc":"2.0","id":"handoff-follow-up","method":"tools/call","params":{"name":"handoffs.accept_with_follow_up","arguments":{"project_id":"PROJECT_ID","work_item_id":"WORK_ITEM_ID","handoff_id":"HANDOFF_ID","expected_updated_at":"HANDOFF_UPDATED_AT","idempotency_key":"ONE-KEY-PER-OPERATOR-ACTION","intent":"accept_and_ensure_follow_up"}}}
```

The handoff must name a target role. Cairnline derives the target work item,
root, execution mode, and role skills from portable project state. When no
assignment is linked, it creates one pristine and queued, links it, and marks
the handoff accepted in one transaction; it never claims or launches that new
assignment. An existing linked assignment is reused and returned in its current
authoritative state, even if it has already progressed or finished. Retry an
uncertain request with the same idempotency key; Cairnline returns the original
outcome and assignment identity with their current authoritative state instead
of creating another assignment. If an operator later closes or changes that
link, the replay conflicts rather than overriding the newer decision. Reusing a
key for a different request, or writing from a stale `updated_at`, also
conflicts.

### 5. Let A Compatible Agent Find Work

An agent can ask for queued assignments matching its kind and skills:

```json
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"assignments.next","arguments":{"project_id":"PROJECT_ID","agent_kind":"any","skill_ids":["review"]}}}
```

The result includes compatible queued assignments. The agent host decides
whether it is allowed to do the work.

### 6. Claim The Assignment

Replace `ASSIGNMENT_ID` with the generated assignment id. `claimed_by` should be
a stable local label for the claiming agent or host session.

```json
{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"assignments.claim","arguments":{"project_id":"PROJECT_ID","assignment_id":"ASSIGNMENT_ID","claimed_by":"local-agent-reviewer"}}}
```

Claiming prevents another compatible agent from picking the same queued work. It
does not grant tools, writes, network access, credentials, or model access.

### 7. Read Assignment Context

```json
{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"assignments.context","arguments":{"project_id":"PROJECT_ID","assignment_id":"ASSIGNMENT_ID"}}}
```

Use this context to build the host-specific prompt or work packet. The context
includes the project's enabled durable memory entries (`memory`), so a portable
host does not need a side channel to recover promoted project memory. Skill
metadata and source locators are provenance hints; Cairnline does not inject
`SKILL.md` bodies or fetch source locators.

After claiming, a host may attach its execution reference or generated context
snapshot without marking work as started:

```json
{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"assignments.prepare","arguments":{"project_id":"PROJECT_ID","assignment_id":"ASSIGNMENT_ID","claimed_by":"local-agent-reviewer","execution_ref":{"kind":"task_run","run_id":"local-run-1"},"context_snapshot_id":"HOST_CONTEXT_SNAPSHOT_ID"}}}
```

Preparation returns a conflict if the assignment is no longer claimed by that
exact worker.

### 8. Mark It Running

```json
{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"assignments.update_status","arguments":{"project_id":"PROJECT_ID","assignment_id":"ASSIGNMENT_ID","status":"running","execution_ref":{"kind":"task_run","run_id":"local-run-1"}}}}
```

`execution_ref` is a structured, host-neutral reference to the execution the
host attached to this assignment. All fields are optional and opaque to
Cairnline:

- `kind`: host execution shape, for example `task_run`, `chat_session`, or
  `external_adapter`;
- `task_id`: host-scoped id of the queued/background unit of work;
- `run_id`: host-scoped id of a single execution attempt;
- `session_id`: host-scoped id of an interactive session driving the work;
- `trace_id`: host observability link, for example an OpenTelemetry trace id;
- `pending_approvals`: count of host-side approval gates currently blocking the
  execution.

Breaking (alpha): `execution_ref` must be an object. The pre-structured
bare-string form is rejected with an invalid-arguments error, and stores that
still hold string refs must be rebuilt or re-seeded from the host's
authoritative data.

When the host pauses the execution on a human approval gate, report it:

```json
{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"assignments.update_status","arguments":{"project_id":"PROJECT_ID","assignment_id":"ASSIGNMENT_ID","status":"awaiting_approval","execution_ref":{"kind":"task_run","run_id":"local-run-1","pending_approvals":1}}}}
```

`awaiting_approval` is a first-class assignment status so portable readers can
distinguish "blocked on a human decision" from "actively executing" without
host-specific overlays. Set the status back to `running` once the approval
resolves. The approval object itself stays host-owned.

### 9. Record Evidence

```json
{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"evidence.record","arguments":{"project_id":"PROJECT_ID","work_item_id":"WORK_ITEM_ID","assignment_id":"ASSIGNMENT_ID","title":"Checklist review notes","body":"Reviewed the launch checklist and found two missing owner approvals.","locator":"notes://launch-checklist-review","source_kind":"note","external_id":"launch-review-1","provider":"local","trust_label":"agent_reported"}}}
```

Evidence locators are stored as metadata. Clients must validate schemes before
rendering locators as links or opening them.

### 10. Complete The Assignment

```json
{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"assignments.complete","arguments":{"project_id":"PROJECT_ID","assignment_id":"ASSIGNMENT_ID","status":"completed","execution_ref":{"kind":"task_run","run_id":"local-run-1"}}}}
```

Use `status:"failed"` when the agent cannot complete the work. The work item
remains durable coordination state for an operator or another agent to inspect.

### 11. Inspect Closeout Readiness

```json
{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"work_items.closeout_readiness","arguments":{"project_id":"PROJECT_ID","work_item_id":"WORK_ITEM_ID"}}}
```

Closeout readiness is advisory. Cairnline tells the operator whether assignments,
evidence, reviews, and handoffs look clear; the client decides what UI or
approval step to show.

### 12. Discover MCP Resources

Cairnline also exposes read-only resources for clients that prefer resource
pickers over tool calls. Discover parameterized URI shapes:

```json
{"jsonrpc":"2.0","id":14,"method":"resources/templates/list"}
```

Then read concrete resources such as the assignment launch packet:

```json
{"jsonrpc":"2.0","id":15,"method":"resources/read","params":{"uri":"cairnline://projects/PROJECT_ID/assignments/ASSIGNMENT_ID/launch-packet"}}
```

## Tool Error Shape

When a tool call fails, the result sets `isError: true`, keeps the human message
in `content`, and adds a machine-readable code in `structuredContent`:

```json
{
  "content": [{"type": "text", "text": "root \"x\" not found"}],
  "structuredContent": {"error": {"code": "not_found", "message": "root \"x\" not found"}},
  "isError": true
}
```

Branch on `structuredContent.error.code` rather than parsing the message. The
codes are `not_found`, `invalid`, `already_exists`, `conflict`, and `internal`;
see the catalog and suggested host HTTP mapping in
[Tool Error Codes](agent-host-integration.md#tool-error-codes).

## MCP Client Configuration

Use a durable database for normal local use:

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
      "args": ["run", "./cmd/cairnline", "-db", "/tmp/cairnline-dev.db"],
      "cwd": "/path/to/cairnline"
    }
  }
}
```

## What Hosts Must Still Enforce

Cairnline records intent and provenance. The consuming host still owns:

- model/provider selection;
- tools, writes, network, and filesystem policy;
- approval prompts and user confirmation;
- credentials, cookies, secrets, and login sessions;
- workspace checkout, branch, and worktree creation;
- conversion from desired-agent and skill hints into host-specific agent
  settings.
