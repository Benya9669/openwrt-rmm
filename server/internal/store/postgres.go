package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"errors"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// OpenPostgres opens a database whose schema has already been checked by the
// startup migration runner. It does not change database schema or import data.
func OpenPostgres(ctx context.Context, dsn string, maxOpen, maxIdle int) (*Store, error) {
	if dsn == "" {
		return nil, errors.New("PostgreSQL connection URL is required")
	}
	rawDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, errors.New("open PostgreSQL database failed")
	}
	if maxOpen <= 0 {
		maxOpen = 16
	}
	if maxIdle < 0 || maxIdle > maxOpen {
		maxIdle = 4
	}
	rawDB.SetMaxOpenConns(maxOpen)
	rawDB.SetMaxIdleConns(maxIdle)
	if err := rawDB.PingContext(ctx); err != nil {
		rawDB.Close()
		return nil, errors.New("connect to PostgreSQL database failed")
	}
	_, ephemeralKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		rawDB.Close()
		return nil, err
	}
	return &Store{db: &sqliteDB{db: rawDB, postgres: true}, postgresDSN: dsn, commandSigningKey: ephemeralKey}, nil
}
