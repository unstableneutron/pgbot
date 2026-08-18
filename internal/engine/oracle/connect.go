// Package oracle implements read-only Oracle Database inspection.
package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/sijms/go-ora/v3"
)

const maxConnections = 4

// Capabilities describes the Oracle identity and topology established during
// connection setup. The initial implementation supports one instance and one
// CDB or PDB target; InstanceCount records RAC without claiming RAC support.
type Capabilities struct {
	DBID          int64
	Database      string
	UniqueName    string
	DatabaseRole  string
	OpenMode      string
	CDB           bool
	Container     string
	ContainerID   int
	Instance      string
	InstanceID    int
	InstanceCount int
	Version       string
}

// Identity is stable across host changes and distinguishes PDB containers.
func (c Capabilities) Identity() string {
	return "dbid:" + strconv.FormatInt(c.DBID, 10) + "/con:" + strconv.Itoa(c.ContainerID)
}

// Target owns the bounded connection pool for one Oracle database target.
type Target struct {
	db   *sql.DB
	caps Capabilities
}

// Open connects to Oracle, establishes database identity, and probes topology.
// The DSN uses go-ora's oracle:// URL form.
func Open(ctx context.Context, dsn string) (*Target, error) {
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, fmt.Errorf("open Oracle connection: %w", err)
	}
	db.SetMaxOpenConns(maxConnections)
	db.SetMaxIdleConns(maxConnections)
	db.SetConnMaxLifetime(5 * time.Minute)

	target := &Target{db: db}
	caps, err := target.probe(ctx)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("probe Oracle database: %w", err)
	}
	target.caps = caps
	return target, nil
}

// Capabilities returns a copy of the probed identity and topology.
func (t *Target) Capabilities() Capabilities { return t.caps }

// Close releases every pooled Oracle connection.
func (t *Target) Close() error {
	if t == nil || t.db == nil {
		return nil
	}
	return t.db.Close()
}

// readOnly makes SET TRANSACTION READ ONLY the first statement after BeginTx.
// go-ora rejects sql.TxOptions{ReadOnly:true}, so the adapter establishes the
// equivalent Oracle transaction explicitly and commits only SELECT work.
func (t *Target) readOnly(ctx context.Context, fn func(*sql.Tx) error) error {
	if t == nil || t.db == nil {
		return fmt.Errorf("Oracle target is not open")
	}
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Oracle transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "SET TRANSACTION READ ONLY"); err != nil {
		return fmt.Errorf("set Oracle transaction read only: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Oracle read-only transaction: %w", err)
	}
	return nil
}

const identityQuery = `
SELECT d.dbid,
       d.name,
       d.db_unique_name,
       d.database_role,
       d.open_mode,
       d.cdb,
       i.instance_name,
       i.instance_number,
       i.version,
       SYS_CONTEXT('USERENV', 'CON_NAME'),
       TO_NUMBER(SYS_CONTEXT('USERENV', 'CON_ID'))
FROM v$database d
CROSS JOIN v$instance i`

const instanceCountQuery = `SELECT COUNT(*) FROM gv$instance`

func (t *Target) probe(ctx context.Context) (Capabilities, error) {
	var caps Capabilities
	var cdb string
	if err := t.queryRow(ctx, identityQuery, nil, func(row *sql.Row) error {
		return row.Scan(
			&caps.DBID,
			&caps.Database,
			&caps.UniqueName,
			&caps.DatabaseRole,
			&caps.OpenMode,
			&cdb,
			&caps.Instance,
			&caps.InstanceID,
			&caps.Version,
			&caps.Container,
			&caps.ContainerID,
		)
	}); err != nil {
		return Capabilities{}, err
	}
	caps.CDB = strings.EqualFold(cdb, "YES")
	caps.InstanceCount = 1
	// GV$INSTANCE can require an additional catalog grant. Identity remains
	// usable when that optional topology probe is unavailable.
	_ = t.queryRow(ctx, instanceCountQuery, nil, func(row *sql.Row) error {
		return row.Scan(&caps.InstanceCount)
	})
	return caps, nil
}

func (t *Target) queryRow(ctx context.Context, query string, args []any, scan func(*sql.Row) error) error {
	if err := ValidateDefaultQuery(query); err != nil {
		return err
	}
	return t.readOnly(ctx, func(tx *sql.Tx) error {
		return scan(tx.QueryRowContext(ctx, query, args...))
	})
}

var easyConnectPassword = regexp.MustCompile(`(?i)(^|\s)([^\s/@]+)/(?:[^\s@]+)@`)

// RedactDSN removes passwords from Oracle URL and Easy Connect strings.
func RedactDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	if parsed, err := url.Parse(dsn); err == nil && parsed.Scheme != "" && parsed.User != nil {
		if _, ok := parsed.User.Password(); ok {
			parsed.User = url.UserPassword(parsed.User.Username(), "REDACTED")
		}
		return parsed.String()
	}
	return easyConnectPassword.ReplaceAllString(dsn, `${1}${2}/REDACTED@`)
}
