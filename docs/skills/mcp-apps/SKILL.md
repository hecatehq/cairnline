---
name: cairnline-mcp-apps
description: Use when adding or changing a Cairnline MCP Apps (ui://) interactive view — the io.modelcontextprotocol/ui extension, ui:// resources, _meta.ui.resourceUri tool tagging, the view↔host postMessage handshake driven by the official ext-apps SDK under strict CSP, and the required tests.
---

# Cairnline MCP Apps skill

Use this skill for any work under `internal/app/apps.go`, `internal/app/status_app.go`,
or `internal/app/views/`, or any change that adds, tags, or renders a `ui://`
interactive view.

MCP Apps (SEP-1865, extension id `io.modelcontextprotocol/ui`) let a host render
an interactive HTML view for a tool result. Cairnline's views are **additive and
read-only**: a host that negotiates the extension can render a view; a host that
does not gets the exact same `content` text and `structuredContent` it always
got. The extension is advertised only when at least one app is registered, and
tagging a tool never changes its call result.

The canonical host-facing contract lives in
[`../../agent-host-integration.md`](../../agent-host-integration.md) (the "MCP
Apps (Interactive Views)" section). The view build/verify details live in
[`../../../internal/app/views/README.md`](../../../internal/app/views/README.md).
This skill is the contributor lens: it does not restate those docs, it tells you
how to change the code without breaking the wire contract or the CSP.

## Spec essentials (as wired in Cairnline)

All of the following is enforced in `internal/app/apps.go` and
`internal/mcp/`; keep these invariants when you touch the code.

- **Extension id.** `io.modelcontextprotocol/ui` — the const `UIExtensionID` in
  `internal/app/apps.go`. Negotiated during `initialize`.
- **App resource mime type.** `text/html;profile=mcp-app` — the const
  `UIAppMimeType`. **No space after the `;`.** This exact string marks a resource
  as an MCP Apps HTML view and is what the extension advertises.
- **`ui://` resources.** An app is a self-contained HTML document served over the
  standard `resources/read` surface at a `ui://…` URI, and listed in
  `resources/list` so a host can prefetch it at setup. The reader
  (`uiAppReader`) claims only the `ui://` prefix (`uiResourcePrefix`) so other
  resource readers keep serving their own schemes.
- **`_meta.ui.resourceUri` tool tagging.** A tool opts in by tagging its
  descriptor's `_meta` with the nested path `ui.resourceUri`. Build it with
  `uiAppMeta(resourceURI)`, which returns `{"ui":{"resourceUri":…}}`, and set it
  on `mcp.Tool.Meta`. This is **descriptor-only** — it never appears in the tool
  call result.
- **Capability declared only when apps exist.** `RegisterApps` is reactive: with
  zero apps it returns without touching the server, so the extension is not
  advertised. With one or more apps it registers the `ui://` provider/reader and
  calls `server.DeclareExtension(UIExtensionID, {"mimeTypes":["text/html;profile=mcp-app"]})`.
  `Server.capabilities()` emits the `extensions` map only when it is non-empty,
  so a stock Cairnline server with no apps advertises no extension at all.
- **View↔host wire handshake** (`postMessage`, JSON-RPC 2.0 envelopes; the view
  runs in a sandboxed iframe and talks to `window.parent`). The view drives this
  through the official `@modelcontextprotocol/ext-apps` `App` — you do not hand-roll
  the messages — but the wire shapes it emits are:
  - **View → host, on load (via `app.connect()`):** posts a `ui/initialize`
    **request** (params carry the app info + `availableDisplayModes`), waits for the
    host's `McpUiInitializeResult` **response**, then posts a
    `ui/notifications/initialized` **notification**. The `App` enforces that
    request→response→initialized ordering; `verify.ts` asserts it.
  - **Host → view:** delivers the tool result as a `ui/notifications/tool-result`
    notification whose `params` are a standard `CallToolResult`. The SDK routes it
    to `app.ontoolresult`; the handler reads `result.structuredContent` and renders.
  - The read-only view does not act on the `ui/initialize` response payload; it
    renders whenever the first `tool-result` arrives via `ontoolresult`.

## Worked example: the Project Status app (B3)

One stateless view backs three tools. Read these files together:

- `internal/app/status_app.go` — `ProjectStatusApp()` returns the `UIApp`
  (`Name`, `URI` = `ui://cairnline/project-status`, `Title`, `Description`,
  `HTML`). The built HTML is pulled in with a **directory** embed
  `//go:embed all:views/dist` into an `embed.FS`; `projectStatusHTML()` reads
  `views/dist/project-status.html` from it and falls back to a committed "not
  built" page (same strict CSP) when the bundle is absent. See "Build & embed
  model" below for why a directory embed (not a single-file embed) is used.
- `internal/app/server.go` — `RegisterApps(server, ProjectStatusApp())` wires it
  in during `NewServer`.
- `internal/app/tools.go` — `projects.health`, `projects.operations_brief`, and
  `projects.activity` each set `Meta: uiAppMeta(projectStatusAppURI)`. No other
  tool is tagged.
- `internal/app/views/src/` — the view as native **Web Components** (no
  framework): `<project-status-app>` (root; owns the ext-apps handshake +
  per-project state + dispatch, exposing `ingest(structuredContent)`) delegating
  to `<health-section>`, `<operations-brief-section>`, `<activity-section>` over
  a shared `BaseComponent extends HTMLElement`. The root detects which of the
  three `structuredContent` shapes arrived by a distinctive field (`summary` →
  health, `buckets` → activity, `counts.work_items` → operations brief) and
  renders that section, accumulating same-`project_id` results into one combined
  view. `action_kind` / `action_label` hints render as **inert badges** — this
  batch never calls tools back from the view.

## Build & embed model

The built bundle `internal/app/views/dist/project-status.html` is **gitignored**,
not committed (mirroring Hecate's `ui/dist`). `dist/.gitkeep` is committed and
the Go side uses a **directory** embed `//go:embed all:views/dist` so `go build`
compiles on a clean checkout with no JS toolchain; when only `.gitkeep` is
present, the app serves a minimal "view not built" placeholder (same strict CSP).
A single-file embed of an absent file is a hard compile error — that is why the
directory + placeholder pattern is used. CI and release run `bun run build`
before the Go build so shipped binaries embed the real view.

## Recipe: add a new app end to end

1. **Write the view** as native **Web Components** under
   `internal/app/views/src/` (no framework). Extend the shared `BaseComponent`
   (`src/base-component.ts`) for each element and register it in `src/define.ts`.
   Model the `structuredContent` interfaces (`src/types.ts`) on the Go types in
   `internal/core/types.go` (snake_case JSON tags must match exactly). Build DOM
   with the base helpers `el()` / `badge()` / `countGrid()` (which write
   `textContent` and set attributes only). **Never** assign
   payload-derived data into `innerHTML` (see gotchas) — static, data-free
   markup only may use a template's `innerHTML`.
2. **Wire the bridge with the official SDK** in the root element's
   `connectedCallback`. `import { App } from "@modelcontextprotocol/ext-apps"`,
   construct `new App({ name, version }, { availableDisplayModes: […] })`, set
   `app.ontoolresult = (result) => this.ingest(result.structuredContent)`
   **before** calling `app.connect()`, then call `app.connect()`. The `App`
   drives the whole `ui/initialize` → `McpUiInitializeResult` →
   `ui/notifications/initialized` handshake and routes each
   `ui/notifications/tool-result` to `ontoolresult`; you never post the raw
   envelopes yourself. Expose a public `ingest()` entry point so the host bridge,
   the preview harness, and the tests all drive the view the same way.
3. **Build with Vite.** The toolchain is **Vite + `vite-plugin-singlefile`**
   (managed by bun), which inlines the entry module and all CSS into one HTML
   file — matching the official ext-apps single-file templates and Hecate's
   `ui/` conventions. `index.html` is the Vite entry (CSP shell + the
   `<project-status-app>` mount + `<script type="module" src="/src/main.ts">`); a
   small Vite plugin renames the output to `dist/project-status.html` (the
   `//go:embed` target). The build is configured deterministic (no sourcemaps, no
   module-preload polyfill, fixed internal chunk names). Output is inline module
   script — **no** IIFE, **no** `unsafe-eval` (see gotchas).
4. **Keep the strict CSP.** Reuse `index.html`'s meta tag verbatim:
   `default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'`.
   That means: inline `<script>`/`<style>` (and adopted/constructable stylesheets
   in shadow roots) only; **no** external domains; `connect-src` stays denied by
   `default-src` (postMessage needs no network origin); and **no** `'unsafe-eval'`
   — the SDK runs fine without it because the `App` constructor defaults
   `allowUnsafeEval:false`, which puts zod in jitless mode so its only
   `new Function` (a try/catch feature probe) is never reached at runtime.
5. **Install deps and build (do not commit the bundle).** `cd
   internal/app/views && bun install && bun run build`. The built
   `dist/project-status.html` is **gitignored** — do not commit it. `dist/.gitkeep`
   keeps the embed directory present; CI/release build the real bundle. The view
   has a real dependency graph (the ext-apps SDK + toolchain, pinned in the
   committed `bun.lock`), so `bun install` is required before the build; CI runs
   `bun install --frozen-lockfile`. A clean `go build` / `go test` still needs
   **no** JS toolchain — it compiles and serves the "not built" placeholder until
   the bundle is built.
6. **Register + tag in Go.** Add a `<view>App() UIApp` constructor that reads the
   built HTML from a directory `//go:embed` with a "not built" fallback (mirror
   `status_app.go`), pass it to `RegisterApps(server, …)` in `server.go`, and tag
   each backing tool with `Meta: uiAppMeta(<viewAppURI>)` in `tools.go`.
7. **Add the required tests** (all are non-negotiable):
   - **Byte-identical fallback regression.** Mirror
     `TestProjectStatusApp_TagLeavesToolResultsUnchanged` in
     `internal/app/status_app_test.go`: call each tagged tool through the real
     server and through a reference server that registers the same handlers
     **without** the `_meta` tag, normalize timestamps, and assert the two
     `result` payloads are byte-for-byte equal. Also assert the call result never
     contains `resourceUri`.
   - **Resource + extension + descriptor coverage.** Mirror `apps_test.go`: the
     `ui://` resource is listed and read with the `mcp-app` profile mime type and
     a non-empty HTML body; each backing tool descriptor carries
     `_meta.ui.resourceUri`; the extension is present with an app and absent
     without one; the embedded HTML is non-empty and looks like the built
     document.
   - **Vitest component tests.** Add a `*.test.ts` suite (Vitest + happy-dom)
     per component under `internal/app/views/src/`. Cover: rendering from
     `structuredContent` (reuse the Go golden fixtures in
     `internal/app/testdata/status_app/` as inputs where the shape matches, so
     the view stays in sync with the backend), per-project state reset (project B
     never shows project A's data), and injection inertness (a payload containing
     `<script>…</script>` renders as literal text with **no** `<script>` element
     created in the component subtree). Run with `bun run test` — **never** `bun
     test`. This suite is CI's real view coverage (there is no committed bundle to
     diff for freshness).
   - **Headless render check.** Extend `internal/app/views/verify/verify.ts`
     with representative fixtures for the new shapes and assert the key text
     rendered (crossing shadow boundaries; see the verification ladder). The
     harness renders the view inside a **sandboxed iframe** whose parent plays the
     host and answers `ui/initialize`, because the SDK `App` is a full JSON-RPC
     peer — at top level `window.parent` is the view itself, which would answer
     its own handshake with `-32601`.

## Gotchas (from B2/B3)

- **Ship an inline ESM module script, never IIFE.** The view uses the official
  `@modelcontextprotocol/ext-apps` SDK, and it runs clean under the strict,
  no-`unsafe-eval` CSP. Vite + `vite-plugin-singlefile` inlines the entry as a
  single `<script type="module">` — the same shape the ext-apps
  quickstart/templates ship; nobody ships IIFE. (The historical `__require is not
  defined` crash came from bundling the SDK's mixed ESM/CJS graph to an IIFE — an
  interop bug, not an SDK defect; the ESM module output has no such reference.)
  The SDK's lone `new Function` is **zod's try/catch feature probe**, and the
  `App` constructor defaults `allowUnsafeEval:false` so zod runs jitless — no eval
  is reached, and the strict CSP stands unchanged. Audit the built file: no
  `eval(`, no external script/style/asset host. If you change the build config or
  SDK version, re-run the headless check.
- **Never `innerHTML` payload data.** `structuredContent` is host-delivered
  data. Populate DOM exclusively through `textContent` and attribute setters via
  the `BaseComponent` helpers (`el()` / `badge()` / `countGrid()`), so a hostile
  or malformed field can't inject markup — an XSS defense independent of the CSP.
  Static, data-free structure may use a template's `innerHTML`; anything derived
  from a payload must not. The Vitest suite asserts this per component.
- **Tagging must not touch the fallback output.** The whole point is that
  `content` text and `structuredContent` are identical whether or not the tool is
  tagged. The `_meta.ui.resourceUri` linkage is descriptor-only; if it ever
  leaks into a call result, the byte-identical regression fails — keep it that
  way.
- **Keep the extension reactive.** Advertising `io.modelcontextprotocol/ui` when
  no app is registered would be a false capability. `RegisterApps` returning
  early on empty input, and `capabilities()` omitting an empty `extensions` map,
  are load-bearing — the reactive test guards both.
- **Mime-type string is exact.** `text/html;profile=mcp-app` with no space after
  the `;`. A stray space silently makes hosts stop recognizing the view.
- **Host support is uneven.** Hosts that render MCP Apps — Claude Desktop, VS
  Code Copilot, Goose — display the view. Current coding-agent CLIs do **not**
  render apps; they are unaffected and use the same text/`structuredContent`.
  Design every view so the tool result is fully usable without it.

## Error-code catalog and host-integration doc

- **Normative tool-error-code catalog:** the "Tool Error Codes" section of
  [`../../agent-host-integration.md`](../../agent-host-integration.md), backed by
  `errorcode.go` (public re-exports) and `internal/core/errorcode.go` (canonical
  definitions + `ClassifyErrorCode`). Codes: `not_found`, `invalid`,
  `already_exists`, `conflict`, `internal`. A failed tool call sets
  `isError: true`, keeps human prose in `content`, and adds a `structuredContent`
  `{ "error": { "code", "message" } }` envelope. Views should treat an error
  result as a state to display, not a shape to parse.
- **Host-integration contract:** the "MCP Apps (Interactive Views)" and
  "Mounting The MCP Server" sections of
  [`../../agent-host-integration.md`](../../agent-host-integration.md).

## Verification ladder

Go changes (the floor for any app/registration/tagging change):

```sh
go build ./...
go vet ./...
go test ./...            # includes apps_test.go + status_app_test.go
go test -race ./...      # required for MCP transport / server changes
```

Focused test iteration while editing the app wiring:

```sh
go test ./internal/app/...
```

View toolchain (needed after editing anything under `internal/app/views/src/`
or `index.html`):

```sh
cd internal/app/views
bun install             # installs the pinned ext-apps SDK + toolchain from bun.lock
bun run typecheck       # tsc --noEmit
bun run test            # Vitest component suite (NEVER `bun test`)
bun run build           # rebuilds dist/project-status.html (gitignored; do NOT commit)
bun run preview         # vite dev server + fixture selector, to develop outside a host
```

The built bundle is **gitignored** — do not commit it (`dist/.gitkeep` is the
only tracked file in `dist/`). CI's `views` job runs `bun install
--frozen-lockfile`, `bun run typecheck`, and `bun run test` (the component suite
replaces the old bundle-freshness diff); the Go `test` job runs `bun run build`
before `go test` so it embeds the real view. Keep the lockfile committed and in
sync.

Headless render check — renders the built HTML inside a **sandboxed iframe** host
(the SDK `App` is a full JSON-RPC peer, so it needs a real parent to answer
`ui/initialize`) in Chromium under its **real** CSP, plays the host side of the
handshake, delivers representative results over the `ui/notifications/tool-result`
postMessage contract, asserts the key text rendered (crossing shadow
boundaries), screenshots, and fails on any console/CSP/page error. Build the
bundle first (it reads `dist/project-status.html`). It is a reproducibility aid,
**not** part of `go test`:

```sh
cd internal/app/views
bun run build           # ensure dist/project-status.html exists
NODE_PATH=/opt/node22/lib/node_modules \
PLAYWRIGHT_BROWSERS_PATH=/opt/pw-browsers \
bun verify/verify.ts [screenshot-path]
```

Done criteria: `go vet` and `go test` (and `-race` for transport/server work)
pass; if a view source changed, `bun run typecheck` + `bun run test` pass and the
headless check runs against a fresh `bun run build` with zero console/CSP errors;
the byte-identical fallback regression and the resource/extension/descriptor
tests cover every newly tagged tool.
