import { existsSync, renameSync, rmSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig, type Plugin } from "vite";
import { viteSingleFile } from "vite-plugin-singlefile";

const here = dirname(fileURLToPath(import.meta.url));

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
