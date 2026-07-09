// Build the MCP Apps (ui://) views into self-contained HTML files that the Go
// server embeds with //go:embed. Each view TS entry is bundled to a single IIFE
// with no external references, inlined into template.html under a strict
// default-deny CSP, and written to dist/. Runtime and `go test` need no JS
// toolchain: dist/*.html is committed. Rebuild with `bun run build` after
// changing anything under src/ or template.html.

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
    format: "iife",
    minify: true,
    target: "browser",
  });
  if (!result.success) {
    for (const log of result.logs) console.error(log);
    throw new Error(`bundle failed for ${view.entry}`);
  }
  const bundle = await result.outputs[0].text();
  // Neutralize any literal "</script" the minifier may emit inside a string so
  // the inline <script> block cannot be closed early by the HTML parser.
  const safe = bundle.replace(/<\/script/gi, "<\\/script");
  // Replace with a function so "$" runs in the minified bundle are inserted
  // literally instead of being read as replacement patterns ($&, $1, ...).
  const html = template.replace(marker, () => safe);
  await writeFile(join(root, view.out), html, "utf8");
  console.log(`wrote ${view.out} (${html.length} bytes)`);
}
