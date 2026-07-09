package app

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// viewsSourceFS embeds the Project Status view SOURCE set into the TEST binary
// only — embeds in _test.go files are excluded from the production build, so this
// never bloats the shipped cairnline binary. It serves two purposes:
//
//  1. It is the working-tree source the guard recomputes the src hash from, read
//     relative to this test file — the exact same set and scheme the Vite build
//     hashes into the bundle (internal/app/views/vite.config.ts, injectSrcHash).
//  2. Because it is a Go build input, editing ANY of these files rebuilds the test
//     binary and busts the `go test` result cache, so the guard actually re-runs
//     and fails at plain `go test` when a src edit was not followed by a rebuild.
//     A runtime os.ReadFile of the sources would be invisible to the cache and
//     could be silently masked by a cached PASS.
//
// The set mirrors vite.config.ts: everything under views/src plus the four root
// files. Paths are stripped of the leading "views/" so they match the Vite
// relpaths (relative to the views root).
//
//go:embed all:views/src views/index.html views/package.json views/bun.lock views/vite.config.ts
var viewsSourceFS embed.FS

var (
	// viewsSrcHashMeta matches the <meta> the Vite build injects into the head.
	viewsSrcHashMeta = regexp.MustCompile(`<meta[^>]*\bname="cairnline-views-src-sha256"[^>]*>`)
	// viewsSrcHashContent extracts the 64-hex-char hash from that meta tag.
	viewsSrcHashContent = regexp.MustCompile(`content="([0-9a-f]{64})"`)
)

// computeViewsSrcHash recomputes the source-set hash from the embedded
// working-tree source, using the identical scheme as
// internal/app/views/vite.config.ts: outer = sha256 over, for each file in
// POSIX-sorted relpath order (relative to the views root),
// relpath + "\n" + hex(sha256(fileBytes)) + "\n".
func computeViewsSrcHash(t *testing.T) string {
	t.Helper()
	rels := make([]string, 0)
	byRel := make(map[string]string) // views-relative path -> embedded FS path
	err := fs.WalkDir(viewsSourceFS, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, "views/")
		rels = append(rels, rel)
		byRel[rel] = p
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded views source: %v", err)
	}
	sort.Strings(rels)

	outer := sha256.New()
	for _, rel := range rels {
		b, readErr := viewsSourceFS.ReadFile(byRel[rel])
		if readErr != nil {
			t.Fatalf("read embedded views source %s: %v", byRel[rel], readErr)
		}
		inner := sha256.Sum256(b)
		outer.Write([]byte(rel + "\n" + hex.EncodeToString(inner[:]) + "\n"))
	}
	return hex.EncodeToString(outer.Sum(nil))
}

// TestProjectStatusView_BundleBuiltAndFresh is the structural guard: `go test`
// fails loudly if the embedded Project Status bundle is missing (never built —
// only the dist/.gitkeep placeholder + "not built" fallback are present) or stale
// (a src file changed since the last `bun run build`). This is deliberately NOT
// skipped: forgetting to build, or forgetting to rebuild after editing src, is a
// hard, immediate `go test` failure — not merely a CI diff-check.
//
// go build / go install are unaffected: they compile and the app serves the "view
// not built" placeholder until built. Refresh with `bun run build` in
// internal/app/views (or `make views` / `go generate ./...`).
func TestProjectStatusView_BundleBuiltAndFresh(t *testing.T) {
	html := ProjectStatusApp().HTML

	metaTag := viewsSrcHashMeta.FindString(html)
	if metaTag == "" {
		t.Fatal("Project Status view bundle not built — run `bun run build` in internal/app/views (or `make views`)")
	}
	match := viewsSrcHashContent.FindStringSubmatch(metaTag)
	if match == nil {
		t.Fatalf("Project Status view src-hash meta malformed: %s", metaTag)
	}
	embedded := match[1]

	current := computeViewsSrcHash(t)
	if embedded != current {
		t.Fatalf("Project Status view bundle is STALE — src changed since last build; run `bun run build` in internal/app/views. embedded=%s current=%s", embedded, current)
	}
}
