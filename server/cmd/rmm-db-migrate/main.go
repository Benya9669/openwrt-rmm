package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"rmm-openwrt/server/internal/dbmigrate"
)

func main() {
	source := flag.String("source-sqlite", "", "path to a stopped, consistent SQLite snapshot")
	expectedHash := flag.String("expected-source-sha256", "", "expected SHA-256 of the SQLite snapshot")
	dryRun := flag.Bool("dry-run", false, "validate source data without writing to PostgreSQL")
	confirm := flag.Bool("confirm-cutover", false, "confirm import into an empty PostgreSQL database")
	dataKey := flag.String("data-encryption-key-file", "", "existing data encryption key file")
	signingKey := flag.String("command-signing-key-file", "", "existing command signing private key file")
	insecureLocal := flag.Bool("insecure-local-postgres", false, "permit sslmode=disable only for a local PostgreSQL test instance")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	report, err := dbmigrate.Run(ctx, dbmigrate.Options{
		SourceSQLite:         *source,
		TargetPostgres:       os.Getenv("RMM_DATABASE_URL"),
		ExpectedSourceSHA256: *expectedHash,
		DryRun:               *dryRun,
		ConfirmCutover:       *confirm,
		DataEncryptionKey:    *dataKey,
		CommandSigningKey:    *signingKey,
		AllowInsecureLocal:   *insecureLocal,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "migration failed:", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, "write migration report:", err)
		os.Exit(1)
	}
}
