# Cairnline MCP Apps views

Self-contained HTML views for Cairnline's MCP Apps (`ui://`) extension
(`io.modelcontextprotocol/ui`, SEP-1865). Each view is bundled to a single HTML
file with all CSS and JS inlined, committed under `dist/`, and embedded into the
Go server with `//go:embed`. **The runtime and `go test` need no JS toolchain** —
only rebuilding a view after editing its source does.

## Build

```sh
cd internal/app/views
bun run build
```

This bundles each entry in `build.ts` to a single IIFE (`bun build --format=iife
--minify`), inlines it into `template.html` under the strict CSP, and writes
`dist/<view>.html`. Commit the regenerated `dist/` file. The build has no runtime
dependencies; `package.json` declares none.

## Reproducibility (Bun pin, lockfile, CI guard)

The Bun version is pinned in `.bun-version` and `package.json`
(`packageManager` / `engines`) so every build uses the same toolchain. The build
has **no third-party dependencies** (`build.ts` uses only Bun built-ins), so
there is no `bun.lock` to commit — `bun install` deletes the empty lockfile. The
lockfile is no longer git-ignored, so it is committed automatically if a
dependency is ever added.

CI enforces bundle freshness: the `Views bundle freshness` job installs the
pinned Bun, runs `bun run build`, and `git diff --exit-code -- dist` fails the
build if the committed bundle drifted from source. A contributor who edits
`src/` without rebuilding is caught in CI. The job needs no network beyond
installing Bun itself, since the build pulls in no packages.

## Files

- `template.html` — HTML shell with the default-deny CSP meta tag and a
  `/* __BUNDLE__ */` placeholder the build replaces with the bundled script.
- `src/project-status.ts` — the Project Status view (renders `ProjectHealth`,
  `ProjectOperationsBrief`, `ProjectActivity`).
- `build.ts` — the bundler.
- `dist/` — committed, embedded output.
- `verify/verify.mjs` — headless Chromium render check (see below).

## Host <-> view bridge

The view implements the MCP Apps postMessage contract directly rather than using
`@modelcontextprotocol/ext-apps`: that SDK does not bundle to a self-contained
browser IIFE (a transitive dynamic `require` survives bundling) and pulls in
`eval` / `new Function` code paths that this view's strict, no-`unsafe-eval` CSP
forbids. The hand-rolled bridge:

- on load, posts a `ui/initialize` request to `window.parent`, then posts the
  `ui/notifications/initialized` readiness notification only after the host's
  `McpUiInitializeResult` response arrives (spec request/response ordering);
- listens for `ui/notifications/tool-result` messages and renders
  `params.structuredContent`, keyed on `project_id` so a result for a new
  project resets the view instead of merging into the prior project's sections.

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

Loads `dist/project-status.html` under its real CSP, delivers representative
health/operations/activity results over the tool-result postMessage contract,
asserts the key text rendered, and writes a screenshot. Requires the `playwright`
package and its Chromium build; it is a reproducibility aid, not part of
`go test`.
