package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/store"
)

func main() {
	dbPath := flag.String("db", "data/vanisync.db", "SQLite database path")
	migrationsDir := flag.String("migrations", "migrations", "SQL migrations directory")
	flag.Parse()

	ctx := context.Background()
	st, err := store.Open(ctx, *dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	if err := st.Migrate(ctx, *migrationsDir); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("migrations applied to %s\n", *dbPath)
}
