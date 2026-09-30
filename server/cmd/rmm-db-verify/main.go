package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"rmm-openwrt/server/internal/dbmigrate"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	encryption := flag.String("encryption-key", "", "existing data encryption key file")
	signing := flag.String("signing-key", "", "existing command signing key file")
	expected := flag.String("expected", "", "optional report from the same frozen source snapshot")
	flag.Parse()
	dsn := os.Getenv("RMM_RECOVERY_DATABASE_URL")
	if dsn == "" || *encryption == "" || *signing == "" || flag.NArg() != 0 {
		return fmt.Errorf("set RMM_RECOVERY_DATABASE_URL and supply --encryption-key and --signing-key")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open recovery database failed")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	report, err := dbmigrate.VerifyRecovery(ctx, db, *encryption, *signing)
	if err != nil {
		return err
	}
	if *expected != "" {
		data, err := os.ReadFile(*expected)
		if err != nil {
			return fmt.Errorf("read expected recovery report failed")
		}
		var want dbmigrate.RecoveryReport
		if err := json.Unmarshal(data, &want); err != nil {
			return fmt.Errorf("invalid expected recovery report")
		}
		if !reflect.DeepEqual(want, report) {
			return fmt.Errorf("restored counts or hashes differ from the expected snapshot")
		}
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
