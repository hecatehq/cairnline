package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/hecatehq/cairnline/internal/app"
	"github.com/hecatehq/cairnline/internal/core"
	"github.com/hecatehq/cairnline/internal/sqlitestore"
)

var version = "0.0.0-dev"

func main() {
	dbPath := flag.String("db", "", "path to a SQLite database file; empty uses in-memory state")
	flag.Parse()

	ctx := context.Background()
	store := core.Store(core.NewMemoryStore())
	if *dbPath != "" {
		sqliteStore, err := sqlitestore.Open(ctx, *dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cairnline: %v\n", err)
			os.Exit(1)
		}
		defer sqliteStore.Close()
		store = sqliteStore
	}
	service := core.NewService(store)
	server := app.NewServer(service, version)
	if err := server.Serve(ctx, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "cairnline: %v\n", err)
		os.Exit(1)
	}
}
