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

Use this context to build the host-specific prompt or work packet. Skill
metadata and source locators are provenance hints; Cairnline does not inject
`SKILL.md` bodies or fetch source locators.

### 8. Mark It Running

```json
{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"assignments.update_status","arguments":{"project_id":"PROJECT_ID","assignment_id":"ASSIGNMENT_ID","status":"running","execution_ref":"local-run-1"}}}
```

`execution_ref` is an opaque reference owned by the host, such as a local run
id, chat id, shell session id, PR number, or ticket id.

### 9. Record Evidence

```json
{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"evidence.record","arguments":{"project_id":"PROJECT_ID","work_item_id":"WORK_ITEM_ID","assignment_id":"ASSIGNMENT_ID","title":"Checklist review notes","body":"Reviewed the launch checklist and found two missing owner approvals.","locator":"notes://launch-checklist-review","source_kind":"note","external_id":"launch-review-1","provider":"local","trust_label":"agent_reported"}}}
```

Evidence locators are stored as metadata. Clients must validate schemes before
rendering locators as links or opening them.

### 10. Complete The Assignment

```json
{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"assignments.complete","arguments":{"project_id":"PROJECT_ID","assignment_id":"ASSIGNMENT_ID","status":"completed","execution_ref":"local-run-1"}}}
```

Use `status:"failed"` when the agent cannot complete the work. The work item
remains durable coordination state for an operator or another agent to inspect.

### 11. Inspect Closeout Readiness

```json
{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"work_items.closeout_readiness","arguments":{"project_id":"PROJECT_ID","work_item_id":"WORK_ITEM_ID"}}}
```

Closeout readiness is advisory. Cairnline tells the operator whether assignments,
evidence, reviews, and handoffs look clear; the client decides what UI or
approval step to show.

### 12. Discover MCP Resources

Cairnline also exposes read-only resources for clients that prefer resource
pickers over tool calls. Discover parameterized URI shapes:

```json
{"jsonrpc":"2.0","id":12,"method":"resources/templates/list"}
```

Then read concrete resources such as the assignment launch packet:

```json
{"jsonrpc":"2.0","id":13,"method":"resources/read","params":{"uri":"cairnline://projects/PROJECT_ID/assignments/ASSIGNMENT_ID/launch-packet"}}
```

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
