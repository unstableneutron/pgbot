//go:build integration

package oracle

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

func TestLiveOracleSafetyBoundary(t *testing.T) {
	dsn := os.Getenv("ORACLE_TEST_DSN")
	if dsn == "" {
		t.Skip("ORACLE_TEST_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	target, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(RedactDSN(err.Error()))
	}
	defer target.Close()

	caps := target.Capabilities()
	if caps.Database == "" || caps.Container == "" || caps.Instance == "" || caps.Version == "" || caps.InstanceCount != 1 {
		t.Fatalf("capabilities = %#v", caps)
	}
	if got := target.db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("max open connections = %d, want 1", got)
	}

	report, err := Inspect(ctx, target, InspectOptions{
		Interval: 500 * time.Millisecond, Deadline: 35 * time.Second, Concurrency: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := report.Data.(*Data)
	if !ok || data == nil {
		t.Fatalf("report data = %T", report.Data)
	}
	sections := map[string]string{
		"health": data.Health.Exactness, "sessions": data.Sessions.Exactness,
		"locks": data.Locks.Exactness, "top_sql": data.TopSQL.Exactness,
		"storage": data.Storage.Exactness, "memory": data.Memory.Exactness,
		"resources": data.Resources.Exactness, "tables": data.Tables.Exactness,
		"indexes": data.Indexes.Exactness, "configuration": data.Configuration.Exactness,
		"recovery": data.Recovery.Exactness,
	}
	for name, exactness := range sections {
		if exactness == "" || exactness == model.ExactnessUnavailable || exactness == model.ExactnessReset {
			t.Errorf("%s exactness = %q", name, exactness)
		}
	}

	wantTable := os.Getenv("ORACLE_TEST_SCHEMA_TABLE")
	if wantTable != "" {
		found := false
		for _, table := range data.Tables.Rows {
			if strings.EqualFold(table.Owner+"."+table.Name, wantTable) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("schema table %q is absent", wantTable)
		}
	}
	for _, statement := range data.TopSQL.Statements {
		if strings.Contains(statement.SampleText, "live-secret-token") || strings.ContainsRune(statement.SampleText, '\x00') {
			t.Errorf("unsafe SQL text %q", statement.SampleText)
		}
	}

	if table := os.Getenv("ORACLE_TEST_WRITE_TABLE"); table != "" {
		attempts := []string{
			"INSERT INTO " + table + " (NOTE) VALUES ('forbidden')",
			"UPDATE " + table + " SET NOTE = 'forbidden' WHERE 1 = 0",
			"CREATE TABLE ORABOT_MONITOR.FORBIDDEN_WRITE (ID NUMBER)",
		}
		for _, statement := range attempts {
			if _, err := target.db.ExecContext(ctx, statement); err == nil {
				t.Errorf("monitor account executed %q", statement)
			}
		}
	}

	if query := os.Getenv("ORACLE_TEST_CANCEL_QUERY"); query != "" {
		cancelCtx, stop := context.WithTimeout(context.Background(), 300*time.Millisecond)
		started := time.Now()
		err := target.query(cancelCtx, query, func(rows *sql.Rows) error {
			if !rows.Next() {
				return rows.Err()
			}
			var count int64
			return rows.Scan(&count)
		})
		stop()
		if err == nil || (!errors.Is(err, context.DeadlineExceeded) && !errors.Is(cancelCtx.Err(), context.DeadlineExceeded)) {
			t.Fatalf("cancellation error after %s = %v", time.Since(started), err)
		}
		if elapsed := time.Since(started); elapsed > 10*time.Second {
			t.Fatalf("Oracle cancellation returned after %s", elapsed)
		}
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()
		if err := target.db.PingContext(pingCtx); err != nil {
			t.Fatalf("ping after cancellation: %v", err)
		}
	}
}
