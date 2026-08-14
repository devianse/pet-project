package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// pingTimeout bounds the startup connectivity check below — every other
// external call in this codebase (TMDb, Telegram) has an explicit
// timeout; a reachable-but-slow Postgres shouldn't be able to hang
// startup indefinitely.
const pingTimeout = 5 * time.Second

// Open connects to Postgres via the pgx stdlib driver and verifies the
// connection with a Ping before returning, so callers don't have to
// guess whether a bad DSN failed at parse time or connection time.
func Open(databaseURL string) (*sql.DB, error) {
	conn, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("pinging db: %w", err)
	}
	return conn, nil
}
