package timesten

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

var targetTestDriverID atomic.Int64

type targetOperationLog struct {
	mu  sync.Mutex
	ops []string
}

func (l *targetOperationLog) add(operation string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ops = append(l.ops, operation)
}

func (l *targetOperationLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.ops...)
}

type targetRecordingDriver struct{ log *targetOperationLog }

func (d targetRecordingDriver) Open(string) (driver.Conn, error) {
	return &targetRecordingConn{log: d.log}, nil
}

type targetRecordingConn struct{ log *targetOperationLog }

func (*targetRecordingConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (*targetRecordingConn) Close() error { return nil }
func (*targetRecordingConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are disabled")
}
func (c *targetRecordingConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.log.add(query)
	return targetEmptyRows{}, nil
}

type targetEmptyRows struct{}

func (targetEmptyRows) Columns() []string         { return nil }
func (targetEmptyRows) Close() error              { return nil }
func (targetEmptyRows) Next([]driver.Value) error { return io.EOF }

func openTargetRecorder(t *testing.T) (*Target, *targetOperationLog) {
	t.Helper()
	log := &targetOperationLog{}
	name := "timesten-select-test-" + strconv.FormatInt(targetTestDriverID.Add(1), 10)
	sql.Register(name, targetRecordingDriver{log: log})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return &Target{db: db}, log
}

func TestQueryUsesSelectWithoutTransaction(t *testing.T) {
	target, log := openTargetRecorder(t)
	const query = "SELECT VALUE FROM SYS.SYSTEMSTATS"
	if err := target.query(context.Background(), query, func(*sql.Rows) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if got, want := log.snapshot(), []string{query}; !reflect.DeepEqual(got, want) {
		t.Fatalf("operations = %#v, want %#v", got, want)
	}
}

func TestQueryRejectsWriteBeforeDriver(t *testing.T) {
	target, log := openTargetRecorder(t)
	err := target.query(context.Background(), "DELETE FROM APP.T", func(*sql.Rows) error { return nil })
	if err == nil {
		t.Fatal("write query succeeded")
	}
	if got := log.snapshot(); len(got) != 0 {
		t.Fatalf("driver received operations %#v", got)
	}
}
