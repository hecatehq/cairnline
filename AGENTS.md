# Cairnline

Portable local-first project coordination server exposed over MCP.

This repo is intentionally independent from Hecate runtime internals. Keep the
core model agent-neutral and avoid importing Hecate packages. Hecate already
embeds this server live (pinned via `go.mod`, run in-process, with an opt-in
sidecar connector mode) and live-mirrors its portable coordination writes into
Cairnline; an opt-in, off-by-default replacement mode lets Cairnline become
authoritative while Hecate keeps a runtime overlay. Even so, this repo must stay
useful for any MCP-capable agent host, and the integration contracts are still
alpha and not stable.

## Conventions

- Keep `internal/core` free of MCP, JSON-RPC, and host-specific adapter details.
- Keep MCP transport code in `internal/mcp`.
- Keep tool registration/application wiring in `internal/app`.
- Assignment records are coordination state; execution and launch authority stay
  outside core.
- Skill metadata never grants tools, writes, network, or approval bypass.

## Verification

Run:

```sh
go test ./...
go vet ./...
```

For storage, MCP transport, or assignment-claim changes, also run:

```sh
go test -race ./...
```
