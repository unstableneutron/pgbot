package oracle

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

var testDriverID atomic.Int64

type operationLog struct {
	mu  sync.Mutex
	ops []string
}

func (l *operationLog) add(op string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ops = append(l.ops, op)
}

func (l *operationLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.ops...)
}

type recordingDriver struct{ log *operationLog }

func (d recordingDriver) Open(string) (driver.Conn, error) { return &recordingConn{log: d.log}, nil }

type recordingConn struct{ log *operationLog }

func (c *recordingConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c *recordingConn) Close() error { return nil }
func (c *recordingConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *recordingConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.log.add("BEGIN")
	return recordingTx{log: c.log}, nil
}
func (c *recordingConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.log.add(query)
	return driver.RowsAffected(0), nil
}
func (c *recordingConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.log.add(query)
	return emptyRows{}, nil
}

type recordingTx struct{ log *operationLog }

func (t recordingTx) Commit() error   { t.log.add("COMMIT"); return nil }
func (t recordingTx) Rollback() error { t.log.add("ROLLBACK"); return nil }

type emptyRows struct{}

func (emptyRows) Columns() []string         { return nil }
func (emptyRows) Close() error              { return nil }
func (emptyRows) Next([]driver.Value) error { return io.EOF }

func openRecordingTarget(t *testing.T) (*Target, *operationLog) {
	t.Helper()
	log := &operationLog{}
	name := "oracle-readonly-test-" + strconv.FormatInt(testDriverID.Add(1), 10)
	sql.Register(name, recordingDriver{log: log})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return &Target{db: db}, log
}

func TestReadOnlyMakesSafetyStatementFirst(t *testing.T) {
	target, log := openRecordingTarget(t)
	err := target.readOnly(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(context.Background(), "SELECT 1 FROM dual")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"BEGIN", "SET TRANSACTION READ ONLY", "SELECT 1 FROM dual", "COMMIT"}
	if got := log.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("operations = %#v, want %#v", got, want)
	}
}

func TestReadOnlyRollsBackAfterCollectorError(t *testing.T) {
	target, log := openRecordingTarget(t)
	wantErr := errors.New("collector failed")
	err := target.readOnly(context.Background(), func(*sql.Tx) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	want := []string{"BEGIN", "SET TRANSACTION READ ONLY", "ROLLBACK"}
	if got := log.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("operations = %#v, want %#v", got, want)
	}
}

func TestRedactDSN(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"oracle://scott:tiger@db.example/ORCL", "oracle://scott:REDACTED@db.example/ORCL"},
		{"scott/tiger@db.example:1521/ORCL", "scott/REDACTED@db.example:1521/ORCL"},
	} {
		if got := RedactDSN(test.in); got != test.want {
			t.Errorf("RedactDSN(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}
