---
name: cairnline-mcp-apps
description: Use when adding or changing a Cairnline MCP Apps (ui://) interactive view — the io.modelcontextprotocol/ui extension, ui:// resources, _meta.ui.resourceUri tool tagging, the view↔host postMessage handshake, the strict-CSP hand-rolled bridge, and the required tests.
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
  runs in a sandboxed iframe and talks to `window.parent`):
  - **View → host, on load:** posts a `ui/initialize` **request** (`id:1`, params
    `appInfo` + `appCapabilities.availableDisplayModes`), then a
    `ui/notifications/initialized` **notification**.
  - **Host → view:** delivers the tool result as a `ui/notifications/tool-result`
    notification whose `params` are a standard `CallToolResult`. The view reads
    `params.structuredContent` from it and renders.
  - The read-only view does not depend on the `ui/initialize` response; it renders
    whenever the first `tool-result` arrives.

## Worked example: the Project Status app (B3)

One stateless view backs three tools. Read these files together:

- `internal/app/status_app.go` — `ProjectStatusApp()` returns the `UIApp`
  (`Name`, `URI` = `ui://cairnline/project-status`, `Title`, `Description`,
  `HTML`). The built HTML is pulled in with `//go:embed views/dist/project-status.html`.
- `internal/app/server.go` — `RegisterApps(server, ProjectStatusApp())` wires it
  in during `NewServer`.
- `internal/app/tools.go` — `projects.health`, `projects.operations_brief`, and
  `projects.activity` each set `Meta: uiAppMeta(projectStatusAppURI)`. No other
  tool is tagged.
- `internal/app/views/src/project-status.ts` — the single view. It detects which
  of the three `structuredContent` shapes arrived by a distinctive field
  (`summary` → health, `buckets` → activity, `counts.work_items` → operations
  brief) and renders that section, keeping sections filled by earlier results.
  `action_kind` / `action_label` hints render as **inert badges** — this batch
  never calls tools back from the view.

## Recipe: add a new app end to end

1. **Write the view** as one stateless TypeScript entry under
   `internal/app/views/src/`. Model the `structuredContent` interfaces on the
   Go types in `internal/core/types.go` (snake_case JSON tags must match
   exactly). Build DOM with `document.createElement` + `textContent` — see the
   `el()` / `badge()` / `countGrid()` helpers. **Never** assign raw
   `structuredContent` into `innerHTML` (see gotchas).
2. **Implement the wire bridge by hand** (do not import the SDK — see gotchas):
   - `window.addEventListener("message", …)`; ignore anything whose
     `jsonrpc !== "2.0"`; on `method === "ui/notifications/tool-result"` read
     `event.data.params?.structuredContent` and render.
   - On load, `window.parent.postMessage(ui/initialize request, "*")` then
     `postMessage(ui/notifications/initialized, "*")`.
3. **Add the entry to the bundler.** Append `{ entry, out }` to the `views`
   array in `internal/app/views/build.ts`. It bundles each entry to a single
   minified IIFE (`Bun.build`, `format: "iife"`, `target: "browser"`), inlines it
   into `template.html` at the `/* __BUNDLE__ */` marker, and writes
   `dist/<view>.html`. It also neutralizes any literal `</script` the minifier
   emits so the inline `<script>` can't be closed early.
4. **Keep the strict CSP.** Reuse `template.html`'s meta tag verbatim:
   `default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'`.
   That means: inline `<script>`/`<style>` only; **no** external domains; `connect-src`
   stays denied by `default-src` (postMessage needs no network origin); **no**
   `eval` / `new Function` (there is no `'unsafe-eval'`).
5. **Build and commit the bundle.** `cd internal/app/views && bun run build`,
   then commit the regenerated `dist/<view>.html`. The runtime and `go test` need
   **no** JS toolchain — only rebuilding after a source edit does.
6. **Register + tag in Go.** Add a `<view>App() UIApp` constructor that
   `//go:embed`s the built HTML (mirror `status_app.go`), pass it to
   `RegisterApps(server, …)` in `server.go`, and tag each backing tool with
   `Meta: uiAppMeta(<viewAppURI>)` in `tools.go`.
7. **Add the required tests** (both are non-negotiable):
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
   - **Headless render check.** Extend `internal/app/views/verify/verify.mjs`
     with representative fixtures for the new shapes and assert the key text
     rendered (see the verification ladder).

## Gotchas (from B2/B3)

- **Do not use the official ext-apps view SDK.** `@modelcontextprotocol/ext-apps`
  does not bundle to a self-contained browser IIFE — a transitive dynamic
  `require` survives bundling and throws `__require is not defined` — and it pulls
  in `new Function` / `eval` code paths that the strict, no-`unsafe-eval` CSP
  forbids. The current pattern is the **hand-rolled** two-message wire bridge in
  `src/project-status.ts`. If you revisit the SDK, prove it bundles clean under
  the real CSP with the headless check before adopting it.
- **Never `innerHTML` raw `structuredContent`.** The payload is host-delivered
  data. Build every node with `createElement` + `textContent` (the `el()` helper)
  so a hostile or malformed field can't inject markup. Keeping it out of
  `innerHTML` is also what lets the view run without `'unsafe-eval'`.
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

View bundle (only needed after editing anything under `internal/app/views/src/`
or `template.html`):

```sh
cd internal/app/views
bun run build           # rebuilds dist/<view>.html; commit the result
```

Headless render check — loads the built HTML in Chromium under its **real** CSP,
delivers representative results over the `ui/notifications/tool-result`
postMessage contract, asserts the key text rendered, screenshots, and fails on
any console/CSP/page error. It is a reproducibility aid, **not** part of
`go test`:

```sh
cd internal/app/views
NODE_PATH=/opt/node22/lib/node_modules \
PLAYWRIGHT_BROWSERS_PATH=/opt/pw-browsers \
node verify/verify.mjs [screenshot-path]
```

Done criteria: `go vet` and `go test` (and `-race` for transport/server work)
pass; if a view source changed, `dist/` was rebuilt and committed and the
headless check runs with zero console/CSP errors; the byte-identical fallback
regression and the resource/extension/descriptor tests cover every newly tagged
tool.
