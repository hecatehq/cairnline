# Cairnline MCP Apps views

Self-contained HTML views for Cairnline's MCP Apps (`ui://`) extension
(`io.modelcontextprotocol/ui`, SEP-1865). Each view is bundled to a single HTML
file with all CSS and JS inlined, committed under `dist/`, and embedded into the
Go server with `//go:embed`. **The runtime and `go test` need no JS toolchain** —
only rebuilding a view after editing its source does.

## Build

```sh
cd internal/app/views
bun install
bun run build
```

This bundles each entry in `build.ts` to a single ESM module (`bun build
--format=esm --minify`) — the view source **plus** the
`@modelcontextprotocol/ext-apps` SDK it imports — inlines it into
`template.html`'s `<script type="module">` under the strict CSP, and writes
`dist/<view>.html`. Commit the regenerated `dist/` file.

The output is ESM, not IIFE, on purpose: bundling the SDK's mixed ESM/CJS-interop
graph to an IIFE makes Bun emit an **undefined `__require`** reference that throws
`__require is not defined` at load — a Bun IIFE-interop bug, not an SDK defect.
The ESM bundle has no such reference. It also needs no `unsafe-eval`: the SDK's
`App` constructor sets zod to jitless mode, so no `eval` / `new Function` path is
taken at runtime (zod's sole `new Function` is a feature-probe wrapped in
`try/catch` that degrades to the interpreter when CSP blocks it).

## Reproducibility (Bun pin, lockfile, CI guard)

The Bun version is pinned in `.bun-version` and `package.json`
(`packageManager` / `engines`) so every build uses the same toolchain. The view's
one dependency (`@modelcontextprotocol/ext-apps`, with its transitive
`@modelcontextprotocol/sdk` / `zod` peers) is pinned in the committed `bun.lock`.
With pinned Bun + frozen lockfile the minified bundle is byte-reproducible.

CI enforces bundle freshness: the `Views bundle freshness` job installs the
pinned Bun, runs `bun install --frozen-lockfile` then `bun run build`, and
`git diff --exit-code -- dist` fails the build if the committed bundle drifted
from source. A contributor who edits `src/` without rebuilding is caught in CI.

## Files

- `template.html` — HTML shell with the default-deny CSP meta tag and a
  `/* __BUNDLE__ */` placeholder the build replaces with the bundled script.
- `src/project-status.ts` — the Project Status view (renders `ProjectHealth`,
  `ProjectOperationsBrief`, `ProjectActivity`).
- `build.ts` — the bundler.
- `dist/` — committed, embedded output.
- `verify/verify.mjs` — headless Chromium render check (see below).

## Host <-> view bridge

The view uses the official `@modelcontextprotocol/ext-apps` SDK. `new App(...)`:

- performs the handshake — posts a `ui/initialize` request to `window.parent`,
  waits for the host's `McpUiInitializeResult` response, then posts the
  `ui/notifications/initialized` readiness notification (spec ordering);
- delivers each host `ui/notifications/tool-result` to `app.ontoolresult`, whose
  `CallToolResult` carries `structuredContent`. The view renders it keyed on
  `project_id`, so a result for a new project resets the view instead of merging
  into the prior project's sections.

Because the SDK is a full JSON-RPC peer, the view must run in an iframe whose
parent is the host (as real hosts render it): at top level `window.parent` is the
view itself, which would answer its own `ui/initialize` with `-32601`. The
`verify/verify.mjs` harness embeds the view in a sandboxed iframe accordingly.

## CSP / sandbox posture

`template.html` sets:

```
default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'
```

No external origins are granted; `connect-src` stays denied (postMessage needs no
network origin). Hosts render the view in a sandboxed iframe.

## Verify (headless render check)

```sh
NODE_PATH=/opt/node22/lib/node_modules \
PLAYWRIGHT_BROWSERS_PATH=/opt/pw-browsers \
node verify/verify.mjs [screenshot-path]
```

Renders `dist/project-status.html` in a sandboxed iframe under its real CSP,
plays the host side of the `ui/initialize` handshake, delivers representative
health/operations/activity results over the tool-result contract, asserts the
key text rendered, and writes a screenshot. Requires the `playwright` package and
its Chromium build; it is a reproducibility aid, not part of `go test`.
