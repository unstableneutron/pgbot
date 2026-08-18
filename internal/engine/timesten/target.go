package timesten

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

const supportedVersion = "22.1-compatible"

// Capabilities describes the TimesTen target established during connection
// setup. The initial implementation supports Classic client/server mode only.
type Capabilities struct {
	Database  string
	Server    string
	User      string
	StartedAt string
	TypeMode  int
	Version   string
}

// Identity is stable when the client host or server address changes.
func (c Capabilities) Identity() string {
	return "timesten:" + c.Database
}

// Target owns the bounded ODBC connection pool for one TimesTen database.
type Target struct {
	db   *sql.DB
	caps Capabilities
}

// Capabilities returns a copy of the probed target identity.
func (t *Target) Capabilities() Capabilities { return t.caps }

// Close releases every pooled TimesTen connection.
func (t *Target) Close() error {
	if t == nil || t.db == nil {
		return nil
	}
	return t.db.Close()
}

func (t *Target) query(ctx context.Context, query string, scan func(*sql.Rows) error) error {
	if t == nil || t.db == nil {
		return fmt.Errorf("TimesTen target is not open")
	}
	if err := ValidateDefaultQuery(query); err != nil {
		return err
	}
	rows, err := t.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	if err := scan(rows); err != nil {
		return err
	}
	return rows.Err()
}

func (t *Target) queryRow(ctx context.Context, query string, scan func(*sql.Row) error) error {
	if t == nil || t.db == nil {
		return fmt.Errorf("TimesTen target is not open")
	}
	if err := ValidateDefaultQuery(query); err != nil {
		return err
	}
	return scan(t.db.QueryRowContext(ctx, query))
}

const identityQuery = `SELECT CURRENT_USER,
       RTRIM(TIME_OF_1ST_CONNECT),
       TYPE_MODE
FROM SYS.MONITOR`

const versionFeatureQuery = `SELECT COUNT(*)
FROM SYS.SYSTEMSTATS
WHERE NAME = 'zzinternal.db.index.range.btree.balance'`

func (t *Target) probe(ctx context.Context, attributes map[string]string) (Capabilities, error) {
	caps := Capabilities{
		Database: attributes["TTC_SERVER_DSN"],
		Server:   attributes["TTC_SERVER"],
		User:     attributes["UID"],
		Version:  supportedVersion,
	}
	if err := t.queryRow(ctx, identityQuery, func(row *sql.Row) error {
		return row.Scan(&caps.User, &caps.StartedAt, &caps.TypeMode)
	}); err != nil {
		return Capabilities{}, fmt.Errorf("probe TimesTen identity: %w", err)
	}

	var featureCount int
	if err := t.queryRow(ctx, versionFeatureQuery, func(row *sql.Row) error {
		return row.Scan(&featureCount)
	}); err != nil {
		return Capabilities{}, fmt.Errorf("probe TimesTen 22.1 catalog: %w", err)
	}
	if featureCount != 1 {
		return Capabilities{}, fmt.Errorf("TimesTen target is not compatible with 22.1 Classic: required B-tree statistic is absent")
	}
	return caps, nil
}

func normalizeDSN(dsn string) (string, map[string]string, error) {
	attributes, err := parseODBCAttributes(dsn)
	if err != nil {
		return "", nil, err
	}
	for _, name := range []string{"TTC_SERVER", "TTC_SERVER_DSN"} {
		if strings.TrimSpace(attributes[name]) == "" {
			return "", nil, fmt.Errorf("TimesTen connection string requires %s for Classic client/server mode", name)
		}
	}
	if attributes["DRIVER"] == "" && attributes["DSN"] == "" {
		return "", nil, fmt.Errorf("TimesTen connection string requires DRIVER or DSN")
	}

	normalized := strings.TrimSpace(dsn)
	if normalized != "" && !strings.HasSuffix(normalized, ";") {
		normalized += ";"
	}
	for _, timeout := range []struct {
		name         string
		defaultValue int
	}{
		{"SQLQUERYTIMEOUT", 5},
		{"TTC_TIMEOUT", 10},
	} {
		value := attributes[timeout.name]
		if value == "" {
			normalized += timeout.name + "=" + strconv.Itoa(timeout.defaultValue) + ";"
			attributes[timeout.name] = strconv.Itoa(timeout.defaultValue)
			continue
		}
		seconds, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || seconds < 1 || seconds > 30 {
			return "", nil, fmt.Errorf("TimesTen %s must be from 1 to 30 seconds", timeout.name)
		}
	}
	return normalized, attributes, nil
}

func parseODBCAttributes(dsn string) (map[string]string, error) {
	attributes := make(map[string]string)
	for offset := 0; offset < len(dsn); {
		for offset < len(dsn) && (dsn[offset] == ';' || dsn[offset] == ' ' || dsn[offset] == '\t' || dsn[offset] == '\n' || dsn[offset] == '\r') {
			offset++
		}
		if offset == len(dsn) {
			break
		}
		equals := strings.IndexByte(dsn[offset:], '=')
		if equals < 0 {
			return nil, fmt.Errorf("TimesTen connection string contains an attribute without =")
		}
		equals += offset
		key := strings.ToUpper(strings.TrimSpace(dsn[offset:equals]))
		if key == "" {
			return nil, fmt.Errorf("TimesTen connection string contains an empty attribute name")
		}
		offset = equals + 1
		start := offset
		if offset < len(dsn) && dsn[offset] == '{' {
			offset++
			for offset < len(dsn) {
				if dsn[offset] != '}' {
					offset++
					continue
				}
				if offset+1 < len(dsn) && dsn[offset+1] == '}' {
					offset += 2
					continue
				}
				offset++
				break
			}
			if offset > len(dsn) || dsn[offset-1] != '}' {
				return nil, fmt.Errorf("TimesTen connection string has an unterminated braced value for %s", key)
			}
			for offset < len(dsn) && dsn[offset] != ';' {
				if dsn[offset] != ' ' && dsn[offset] != '\t' && dsn[offset] != '\n' && dsn[offset] != '\r' {
					return nil, fmt.Errorf("TimesTen connection string has text after braced value for %s", key)
				}
				offset++
			}
		} else {
			for offset < len(dsn) && dsn[offset] != ';' {
				offset++
			}
		}
		if _, duplicate := attributes[key]; duplicate {
			return nil, fmt.Errorf("TimesTen connection string repeats attribute %s", key)
		}
		value := strings.TrimSpace(dsn[start:offset])
		if len(value) >= 2 && value[0] == '{' && value[len(value)-1] == '}' {
			value = strings.ReplaceAll(value[1:len(value)-1], "}}", "}")
		}
		attributes[key] = value
	}
	return attributes, nil
}
