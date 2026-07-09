// Build the MCP Apps (ui://) views into self-contained HTML files that the Go
// server embeds with //go:embed. Each view TS entry — including the official
// @modelcontextprotocol/ext-apps SDK it imports — is bundled to a single ESM
// module, inlined into template.html's <script type="module"> under a strict
// default-deny CSP, and written to dist/. Runtime and `go test` need no JS
// toolchain: dist/*.html is committed. Rebuild with `bun install` then
// `bun run build` after changing anything under src/ or template.html.
//
// The output format is ESM, not IIFE: bundling the SDK's ESM+CJS-interop graph
// to an IIFE makes Bun emit an undefined __require reference (a Bun interop bug)
// that throws at load. ESM output has no such reference and runs under the
// no-unsafe-eval CSP; the SDK sets zod to jitless mode so no eval path is taken.

import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const root = dirname(fileURLToPath(import.meta.url));

const views = [{ entry: "src/project-status.ts", out: "dist/project-status.html" }];

const template = await readFile(join(root, "template.html"), "utf8");
const marker = "/* __BUNDLE__ */";
if (!template.includes(marker)) {
  throw new Error(`template.html is missing the ${marker} placeholder`);
}

for (const view of views) {
  const result = await Bun.build({
    entrypoints: [join(root, view.entry)],
    format: "esm",
    minify: true,
    target: "browser",
  });
  if (!result.success) {
    for (const log of result.logs) console.error(log);
    throw new Error(`bundle failed for ${view.entry}`);
  }
  // The view must inline as one script. A second output chunk (e.g. from a
  // surviving dynamic import) would be silently dropped by inlining only
  // outputs[0], so fail loudly instead of shipping a broken bundle.
  const chunks = result.outputs.filter((o) => o.kind !== "sourcemap");
  if (chunks.length !== 1) {
    throw new Error(`expected a single bundle chunk for ${view.entry}, got ${chunks.length}`);
  }
  const bundle = await chunks[0].text();
  // Neutralize any literal "</script" the minifier may emit inside a string so
  // the inline <script> block cannot be closed early by the HTML parser.
  const safe = bundle.replace(/<\/script/gi, "<\\/script");
  // Replace with a function so "$" runs in the minified bundle are inserted
  // literally instead of being read as replacement patterns ($&, $1, ...).
  const html = template.replace(marker, () => safe);
  await writeFile(join(root, view.out), html, "utf8");
  console.log(`wrote ${view.out} (${html.length} bytes)`);
}
