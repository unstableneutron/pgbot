//go:build timesten && integration && cgo

package timesten

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pgrundev/pgbot/internal/model"
)

func TestLiveTimesTenSafetyBoundary(t *testing.T) {
	dsn := os.Getenv("TIMESTEN_TEST_DSN")
	if dsn == "" {
		t.Skip("TIMESTEN_TEST_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	target, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(RedactDSN(err.Error()))
	}
	defer target.Close()

	caps := target.Capabilities()
	if caps.Database == "" || caps.Server == "" || caps.User == "" || caps.Version != supportedVersion {
		t.Fatalf("capabilities = %#v", caps)
	}

	var (
		number    int64
		text      string
		nullText  sql.NullString
		timestamp time.Time
	)
	const scanQuery = `SELECT CAST(42 AS TT_INTEGER),
       CAST('monitor' AS VARCHAR2(20)),
       CAST(NULL AS VARCHAR2(20)),
       TIMESTAMP '2026-08-17 12:34:56'
FROM SYS.DUAL`
	if err := target.queryRow(ctx, scanQuery, func(row *sql.Row) error {
		return row.Scan(&number, &text, &nullText, &timestamp)
	}); err != nil {
		t.Fatal(err)
	}
	if number != 42 || text != "monitor" || nullText.Valid || timestamp.UTC().Format(time.RFC3339) != "2026-08-17T12:34:56Z" {
		t.Fatalf("scan values = %d, %q, %#v, %s", number, text, nullText, timestamp)
	}

	tx, err := target.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err == nil {
		_ = tx.Rollback()
		t.Fatal("ODBC driver accepted unsupported read-only transaction options")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "read-only") {
		t.Fatalf("read-only transaction error = %v", err)
	}

	if table := os.Getenv("TIMESTEN_TEST_WRITE_TABLE"); table != "" {
		attempts := []string{
			"INSERT INTO " + table + " (PK, C1) VALUES (900001, 'forbidden')",
			"UPDATE " + table + " SET C1 = 'forbidden' WHERE PK = 1",
			"CREATE TABLE PGBOT_MONITOR.FORBIDDEN_WRITE (ID TT_INTEGER)",
		}
		for _, statement := range attempts {
			if _, err := target.db.ExecContext(ctx, statement); err == nil {
				t.Errorf("monitor account executed %q", statement)
			}
		}
	}

	if query := os.Getenv("TIMESTEN_TEST_CANCEL_QUERY"); query != "" {
		cancelCtx, stop := context.WithTimeout(context.Background(), 300*time.Millisecond)
		started := time.Now()
		var count int64
		err := target.db.QueryRowContext(cancelCtx, query).Scan(&count)
		stop()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("cancellation error after %s = %v", time.Since(started), err)
		}
		if elapsed := time.Since(started); elapsed > 10*time.Second {
			t.Fatalf("SQLCancel returned after %s", elapsed)
		}
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()
		if err := target.db.PingContext(pingCtx); err != nil {
			t.Fatalf("ping after cancellation: %v", err)
		}
	}

	report, err := Inspect(ctx, target, InspectOptions{
		Interval: 500 * time.Millisecond, Deadline: 20 * time.Second, Concurrency: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := report.Data.(*Data)
	if !ok || data == nil {
		t.Fatalf("report data = %T", report.Data)
	}
	sections := map[string]string{
		"health":      data.Health.Exactness,
		"connections": data.Connections.Exactness,
		"locks":       data.Locks.Exactness,
		"space":       data.Space.Exactness,
		"persistence": data.Persistence.Exactness,
		"tables":      data.Tables.Exactness,
		"indexes":     data.Indexes.Exactness,
		"replication": data.Replication.Exactness,
	}
	for name, exactness := range sections {
		if exactness == model.ExactnessUnavailable || exactness == model.ExactnessReset || exactness == "" {
			t.Errorf("%s exactness = %q", name, exactness)
		}
	}
	if len(data.Tables.Rows) == 0 {
		t.Error("table inventory is empty")
	}
	if data.TopSQL == nil || data.TopSQL.Exactness != model.ExactnessUnavailable || !strings.Contains(data.TopSQL.Reason, "TTStats") {
		t.Errorf("top SQL = %#v", data.TopSQL)
	}
	if data.Configuration == nil || data.Configuration.Exactness != model.ExactnessUnavailable || !strings.Contains(data.Configuration.Reason, "SELECT-only") {
		t.Errorf("configuration = %#v", data.Configuration)
	}
}
