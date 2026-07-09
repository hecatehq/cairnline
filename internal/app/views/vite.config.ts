import { createHash } from "node:crypto";
import { existsSync, readdirSync, readFileSync, renameSync, rmSync } from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig, type Plugin } from "vite";
import { viteSingleFile } from "vite-plugin-singlefile";

const here = dirname(fileURLToPath(import.meta.url));

// Source set hashed into the built HTML so the Go guard test can detect a missing
// or stale bundle. Relative to the views root, POSIX-sorted: everything under
// src/** plus these four root files. Kept in lockstep with computeViewsSrcHash in
// internal/app/status_app_bundle_test.go — same files, same scheme.
const srcHashRootFiles = ["index.html", "package.json", "bun.lock", "vite.config.ts"];

function collectSourceRelPaths(root: string): string[] {
  const rels: string[] = [];
  const walk = (dir: string): void => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const abs = join(dir, entry.name);
      if (entry.isDirectory()) walk(abs);
      else rels.push(relative(root, abs).split(sep).join("/"));
    }
  };
  walk(join(root, "src"));
  rels.push(...srcHashRootFiles);
  rels.sort();
  return rels;
}

// computeSrcHash: outer = sha256 over, for each file in POSIX-sorted relpath
// order, `relpath + "\n" + hex(sha256(fileBytes)) + "\n"`. index.html is hashed
// from its source bytes (pre-injection). Deterministic given source, so the build
// stays byte-reproducible.
function computeSrcHash(root: string): string {
  const outer = createHash("sha256");
  for (const rel of collectSourceRelPaths(root)) {
    const inner = createHash("sha256")
      .update(readFileSync(join(root, rel)))
      .digest("hex");
    outer.update(`${rel}\n${inner}\n`);
  }
  return outer.digest("hex");
}

// injectSrcHash writes the source-set hash into a <meta> in the built HTML head.
// The Go guard test (TestProjectStatusView_BundleBuiltAndFresh) fails loudly at
// `go test` when the meta is absent (bundle not built) or its hash does not match
// the working-tree source (bundle stale). The meta is static, data-free markup,
// so it is CSP-safe; it is injected only into the OUTPUT, never the source file.
function injectSrcHash(): Plugin {
  return {
    name: "cairnline-inject-src-hash",
    apply: "build",
    transformIndexHtml: {
      order: "post",
      handler() {
        return [
          {
            tag: "meta",
            attrs: { name: "cairnline-views-src-sha256", content: computeSrcHash(here) },
            injectTo: "head",
          },
        ];
      },
    },
  };
}

// preserveDistGitkeep re-emits dist/.gitkeep on every build. The committed
// placeholder is what keeps `//go:embed all:views/dist` compiling on a clean
// source checkout (the built bundle is gitignored). Vite's emptyOutDir wipes the
// directory before writing, so a tracked .gitkeep would vanish on each build and
// the worktree would show a phantom deletion; emitting it through Rollup's
// generateBundle (which runs after empty-out-dir) makes it land in dist
// regardless of who invoked the build. Mirrors Hecate's ui/ vite config.
function preserveDistGitkeep(): Plugin {
  return {
    name: "cairnline-preserve-dist-gitkeep",
    apply: "build",
    generateBundle() {
      this.emitFile({ type: "asset", fileName: ".gitkeep", source: "" });
    },
  };
}

// renameSingleFileOutput renames Vite's index.html output to the //go:embed
// target name (dist/project-status.html) after vite-plugin-singlefile has
// inlined every asset. The embed path in status_app.go must stay stable; the
// rename runs after the bundle is written and is deterministic.
function renameSingleFileOutput(): Plugin {
  return {
    name: "cairnline-rename-single-file-output",
    apply: "build",
    enforce: "post",
    closeBundle() {
      const from = resolve(here, "dist/index.html");
      const to = resolve(here, "dist/project-status.html");
      if (existsSync(from)) {
        if (existsSync(to)) rmSync(to);
        renameSync(from, to);
      }
    },
  };
}

// The build inlines the entry module and all CSS into a single HTML file under
// the view's strict, no-unsafe-eval CSP. Output is intentionally deterministic
// (no sourcemaps, no module-preload polyfill, fixed internal chunk name) so the
// bundle is byte-reproducible. The bundle is gitignored and built by CI/release
// before the Go build (it is not committed).
export default defineConfig({
  root: here,
  plugins: [
    viteSingleFile({ removeViteModuleLoader: true }),
    injectSrcHash(),
    preserveDistGitkeep(),
    renameSingleFileOutput(),
  ],
  build: {
    target: "es2022",
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: false,
    cssCodeSplit: false,
    modulePreload: { polyfill: false },
    assetsInlineLimit: Number.MAX_SAFE_INTEGER,
    rollupOptions: {
      input: resolve(here, "index.html"),
      output: {
        entryFileNames: "app.js",
        assetFileNames: "app.[ext]",
      },
    },
  },
});
