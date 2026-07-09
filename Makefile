# Cairnline build helpers.
#
# The Project Status MCP Apps view (internal/app/views) is built with bun and
# embedded into the Go binary. Its built bundle is gitignored, so it must be
# built before the Go build; `go test` hard-fails via a guard test if the bundle
# is missing or stale (see internal/app/status_app_bundle_test.go).

VIEWS_DIR := internal/app/views

.PHONY: views build test

## views: install pinned deps and build the embedded Project Status view bundle
views:
	cd $(VIEWS_DIR) && bun install --frozen-lockfile && bun run build

## build: build the view bundle, then the Go binaries
build: views
	go build ./...

## test: build the view bundle, then run the Go tests
test: views
	go test ./...
