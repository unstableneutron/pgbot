package oracle

import (
	"strings"
	"testing"
)

func TestValidateDefaultQueryAllowsReadOnlySQL(t *testing.T) {
	queries := []string{
		"SELECT value FROM v$sysstat",
		"WITH totals AS (SELECT value FROM v$sysstat) SELECT value FROM totals",
		identityQuery,
		instanceCountQuery,
	}
	for _, query := range queries {
		if err := ValidateDefaultQuery(query); err != nil {
			t.Errorf("query rejected: %v\n%s", err, query)
		}
	}
}

func TestValidateDefaultQueryRejectsUnsafeSQL(t *testing.T) {
	tests := map[string]string{
		"UPDATE app.users SET enabled = 0":                      "must start",
		"SELECT * FROM app.users FOR UPDATE":                    "mutating",
		"SELECT * FROM app.users; DELETE FROM app.users":        "separator",
		"SELECT * FROM app.users -- conceal a second statement": "comment",
		"SELECT * FROM v$active_session_history":                "ACTIVE_SESSION_HISTORY",
		"SELECT * FROM dba_hist_sqlstat":                        "DBA_HIST_",
		"SELECT DBMS_SQLTUNE.REPORT_SQL_MONITOR('x') FROM dual": "DBMS_SQLTUNE",
	}
	for query, want := range tests {
		err := ValidateDefaultQuery(query)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateDefaultQuery(%q) error = %v, want text %q", query, err, want)
		}
	}
}
