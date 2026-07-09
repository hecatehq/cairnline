# Cairnline MCP Apps views

Self-contained HTML views for Cairnline's MCP Apps (`ui://`) extension
(`io.modelcontextprotocol/ui`, SEP-1865). Each view is built from native Web
Components into a single HTML file with all CSS and JS inlined and embedded into
the Go server with `//go:embed`. **`go build` / `go install` need no JS
toolchain** to compile and run — but **`go test` hard-fails** until the real view
is built and fresh (see the build & embed model below).

## Build & embed model (dist is gitignored, guarded at `go test`)

The built bundle `dist/project-status.html` is **not committed** — it is
gitignored and produced by `bun run build`. This mirrors Hecate's `ui/dist`
convention and removes the committed-artifact footgun. To keep the Go build
compiling on a clean checkout without a JS toolchain:

- `dist/.gitkeep` is committed so the embed directory always exists.
- `status_app.go` uses a **directory** embed, `//go:embed all:views/dist`, into
  an `embed.FS` (a single-file embed of an absent file is a hard compile error;
  the directory + `.gitkeep` placeholder avoids that).
- At use, it reads `views/dist/project-status.html` from the FS. When the bundle
  is absent (only `.gitkeep` present), it falls back to a small committed
  "Project Status view not built" page carrying the **exact same strict CSP**.

### Structural freshness guard (the teeth)

A missing or stale bundle is a **hard, immediate `go test` failure** — not a
convention, not only a CI diff-check:

- The Vite build injects `<meta name="cairnline-views-src-sha256" content="…">`
  into the built HTML head — a sha256 over a deterministic, POSIX-sorted source
  set (`src/**` plus `index.html`, `package.json`, `bun.lock`, `vite.config.ts`),
  scheme: outer sha256 of, per file, `relpath + "\n" + hex(sha256(bytes)) + "\n"`.
- `TestProjectStatusView_BundleBuiltAndFresh` (`internal/app/status_app_bundle_test.go`)
  reads the embedded bundle and:
  - **Fails "not built"** if the meta is absent (the fallback page is embedded).
  - **Fails "STALE"** if the meta hash ≠ the hash recomputed from the working-tree
    source (same set + scheme). The source set is embedded into the **test**
    binary (a `_test.go` embed, excluded from the shipped binary), so editing any
    src file busts the `go test` cache and the guard actually re-runs at plain
    `go test` — a runtime file read would be masked by a cached PASS.

Consequences:

- A clean `go install` / `go build` **compiles and runs**, serving the minimal
  "not built" placeholder until the bundle is built. `go test` does **not** pass
  until the bundle is built and fresh.
- CI (`.github/workflows/ci.yml`) and release (`release.yml`) run
  `bun install --frozen-lockfile && bun run build` in `internal/app/views`
  **before** the Go build, so shipped binaries embed the real view and CI stays
  green.
- Refresh the bundle before `go test` / `verify` with any of:
  `cd internal/app/views && bun run build`, `make views` (top-level), or
  `go generate ./...` (the `//go:generate` on `status_app.go` runs the same
  build).

## Architecture: native Web Components

The Project Status view is a set of native custom elements (no framework). A
small base class owns the shadow root and the DOM-building helpers; a root
element owns the host handshake and per-project state and delegates to one
component per section:

- `src/base-component.ts` — `BaseComponent extends HTMLElement`: attaches the
  shadow root, adopts the shared stylesheet, and exposes the `el` / `badge` /
  `countGrid` / `renderContent` helpers. Every helper writes text with
  `textContent` and classes via the class attribute — **never `innerHTML` with
  payload data** — so untrusted strings render inert.
- `src/components/project-status-app.ts` — `<project-status-app>`: the root
  element. Owns the `@modelcontextprotocol/ext-apps` handshake, the per-project
  state machine (`resetForProject` / `classify`), and dispatch. Its public
  `ingest(structuredContent)` is the single entry point — the host bridge, the
  preview harness, and the tests all call it.
- `src/components/health-section.ts` — `<health-section>`: renders a
  `projects.health` payload (status badge, title, detail, summary grid,
  attention list with inert action badges).
- `src/components/operations-brief-section.ts` — `<operations-brief-section>`:
  renders a `projects.operations_brief` payload (status, title, detail, counts
  grid, item list).
- `src/components/activity-section.ts` — `<activity-section>`: renders a
  `projects.activity` payload (counts grid + active/blocked/completed/other
  buckets).
- `src/styles.ts` — the shared structural stylesheet, adopted by every shadow
  root (falls back to an inline `<style>` where constructable stylesheets are
  unavailable, e.g. the unit-test DOM). The palette lives in `index.html`'s
  `:root` and inherits through the shadow boundary.
- `src/types.ts` — the structuredContent shapes (mirroring
  `internal/core/types.go`).
- `src/define.ts` — registers the custom elements (idempotent).
- `src/main.ts` — runtime entry: registers the elements and, in dev only, loads
  the preview harness.

A single view backs all three tools: it detects which shape arrived by a
distinctive field (`summary` → health, `buckets` → activity, `counts.work_items`
→ operations) and renders that section. Same-`project_id` results accumulate
into one combined view; the first result carrying a different `project_id`
resets every section, so two projects' results never bleed together. All
`action_label` hints render as inert badges — this view never calls tools back.

## Toolchain: Vite + bun

The package manager is **bun** (pinned in `.bun-version` and `package.json`); the
bundler is **Vite** with **`vite-plugin-singlefile`**, which inlines the entry
module and all CSS into one HTML file (matching the official ext-apps single-file
templates and the Hecate `ui/` conventions).

```sh
cd internal/app/views
bun install
bun run build      # vite build -> dist/project-status.html (single inlined file)
bun run typecheck  # tsc --noEmit
bun run test       # vitest run (component tests; NEVER `bun test`)
bun run lint       # oxlint
bun run format     # oxfmt --write  (format:check for CI-style verification)
bun run verify     # bun verify/verify.ts (headless render check, see below)
bun run preview    # vite dev server with the fixture selector (see below)
```

`bun run build` runs `vite build`. A tiny Vite plugin renames Vite's
`dist/index.html` output to `dist/project-status.html` — the exact
`//go:embed` target in `status_app.go`. The build is configured to be
deterministic (no sourcemaps, no module-preload polyfill, fixed internal chunk
names), so the committed `dist/project-status.html` is byte-reproducible.

## Preview (develop outside an MCP host)

```sh
cd internal/app/views
bun run preview    # http://localhost:5173
```

The preview harness (`src/preview.ts`, loaded only under `import.meta.env.DEV`
and dead-code-eliminated from the production bundle) renders a fixture selector
above the view. Choosing a fixture feeds the same `structuredContent` a host
would deliver — straight into `<project-status-app>.ingest` — so every section,
the project-switch reset, and the injection-safety behavior can be exercised
with a click, no MCP host required. Fixtures live in `src/fixtures.ts` and are
shared with the component tests.

## Tests

`bun run test` runs Vitest with a `happy-dom` DOM. Each component has a suite
covering: rendering from `structuredContent` (including the three committed
Go-side golden fixtures under `internal/app/testdata/status_app/`, read directly
so the view stays in sync with the backend contract), the per-project state
reset (project B never shows project A's data), and injection-safety (a payload
containing `<script>…</script>` renders as literal text with **no** `<script>`
element created in the component subtree). Never run `bun test` — it bypasses the
Vitest environment.

## Reproducibility & CI

The Bun version is pinned in `.bun-version` and `package.json`
(`packageManager` / `engines`). Dependencies (`@modelcontextprotocol/ext-apps`
plus the Vite/Vitest/oxlint toolchain) are pinned in the committed `bun.lock`.
With pinned Bun + frozen lockfile the Vite single-file output is byte-stable.

Because the bundle is no longer committed, the old "bundle freshness" diff guard
is gone; the **`go test` freshness guard** above replaces it (a missing/stale
bundle hard-fails `go test`, locally and in CI). CI's `views` job runs `bun
install --frozen-lockfile`, `bun run typecheck`, and `bun run test` (the Vitest
component suite) — that suite is the view's behavioral coverage. The Go `test`
job builds the bundle before running `go test` so it embeds a fresh real view.

## CSP / injection-safety invariants

`index.html` sets, byte-for-byte:

```
default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'
```

No external origins are granted; `connect-src` stays denied by `default-src`
(postMessage needs no network origin). `script-src` / `style-src` carry only
`'unsafe-inline'` (no `'unsafe-eval'`): the inlined module script and every
component `<style>` / adopted stylesheet run inline, and nothing takes an
`eval` / `new Function` path at runtime — the ext-apps `App` constructor sets zod
to jitless mode (its sole `new Function` is a feature-probe wrapped in
`try/catch` that degrades to the interpreter when CSP blocks it). The built file
references no external script/style/asset hosts. Hosts render the view in a
sandboxed iframe.

Injection-safety is a load-bearing property: **no component ever assigns
`innerHTML` from payload data.** Static, data-free structure may use a template's
`innerHTML`, but everything derived from a tool result is written through
`textContent` / attribute setters via the `BaseComponent` helpers. The Vitest
suite asserts this for each component.

## Host <-> view bridge

The view uses the official `@modelcontextprotocol/ext-apps` SDK. `new App(...)`
(constructed in `<project-status-app>`'s `connectedCallback`):

- performs the handshake — posts a `ui/initialize` request to `window.parent`,
  waits for the host's `McpUiInitializeResult` response, then posts the
  `ui/notifications/initialized` readiness notification (spec ordering);
- delivers each host `ui/notifications/tool-result` to `app.ontoolresult`, whose
  `CallToolResult` carries `structuredContent`, which is handed to `ingest`.

Because the SDK is a full JSON-RPC peer, the view must run in an iframe whose
parent is the host (as real hosts render it): at top level `window.parent` is the
view itself, which would answer its own `ui/initialize` with `-32601`. The
`verify/verify.ts` harness embeds the view in a sandboxed iframe accordingly.

## Verify (headless render check)

```sh
NODE_PATH=/opt/node22/lib/node_modules \
PLAYWRIGHT_BROWSERS_PATH=/opt/pw-browsers \
bun verify/verify.ts [screenshot-path]
```

Renders `dist/project-status.html` in a sandboxed iframe under its real CSP,
plays the host side of the `ui/initialize` handshake, delivers representative
health/operations/activity results over the tool-result contract, asserts the
key text rendered (crossing shadow boundaries), checks the no-cross-project-bleed
behavior, and writes a screenshot. Requires the `playwright` package and its
Chromium build; it is a reproducibility aid, not part of `go test`. Screenshots
(`verify/*.png`) are gitignored.
