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

## Recommended MCP Pull Flow

Hosts that want agent-neutral interoperability should start here:

```text
coordination.capabilities
assignments.next
assignments.claim
assignments.context
assignments.launch_packet
evidence.record
assignments.complete
```

1. Call `coordination.capabilities` once during setup or health checks to learn
   the server contract and boundaries.
2. Poll `assignments.next` with the host's available kind and skill ids.
3. Claim one assignment with `assignments.claim`.
4. Read `assignments.context` and/or `assignments.launch_packet`. Both include
   the project's enabled durable memory entries.
5. Build the host-native prompt/run packet from the structured metadata.
6. Record evidence as the run produces useful proof.
7. Complete, fail, or cancel the assignment explicitly.

If the host crashes after claim, it should either resume by `execution_ref` or
release the claim when it knows work will not continue.

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

## Security Checklist For Hosts

Before exposing Cairnline tools to an agent, a host should decide:

- Which mutating tools the agent may call.
- Whether assignments can be claimed automatically or require operator review.
- Whether evidence locators are only stored, rendered as text, or opened as
  links after scheme validation.
- Whether local roots are readable by the host, and under what path boundary.
- Whether skill metadata may trigger host-native instruction loading.
- How to recover or release claimed assignments after crashes.
- How to show operator confirmation for memory promotion and destructive
  project changes.

## Minimal Integration Target

A minimal useful integration does not need orchestration. It only needs:

- `coordination.capabilities`
- `projects.list`
- `assignments.next`
- `assignments.claim`
- `assignments.context`
- `assignments.launch_packet`
- `evidence.record`
- `assignments.complete`

That is enough for an MCP-capable host to pull one queued assignment, do work
under its own policy, record proof, and hand control back to the operator.
