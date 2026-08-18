//go:build timesten && cgo

package timesten

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/alexbrainman/odbc"
)

const maxConnections = 4

// Open connects through unixODBC, verifies Classic client/server mode, and
// probes the TimesTen 22.1 catalog. The native driver is available only with
// the timesten build tag and cgo.
func Open(ctx context.Context, dsn string) (*Target, error) {
	normalized, attributes, err := normalizeDSN(dsn)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("odbc", normalized)
	if err != nil {
		return nil, fmt.Errorf("open TimesTen ODBC connection: %w", err)
	}
	db.SetMaxOpenConns(maxConnections)
	db.SetMaxIdleConns(maxConnections)
	db.SetConnMaxLifetime(5 * time.Minute)

	target := &Target{db: db}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping TimesTen database: %w", err)
	}
	caps, err := target.probe(ctx, attributes)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	target.caps = caps
	return target, nil
}
