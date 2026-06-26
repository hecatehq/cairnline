# Cairnline

Portable local-first project coordination server exposed over MCP.

This repo is intentionally independent from Hecate runtime internals. Keep the
core model agent-neutral and avoid importing Hecate packages. Hecate may later
embed or connect to this server, but this repo should stay useful for any
MCP-capable agent host.

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
