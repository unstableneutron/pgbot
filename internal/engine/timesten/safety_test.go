package timesten

import (
	"strings"
	"testing"
)

func TestValidateDefaultQueryAllowsSelect(t *testing.T) {
	for _, query := range []string{
		"SELECT VALUE FROM SYS.SYSTEMSTATS",
		"\nSELECT COUNT(*)\nFROM SYS.MONITOR",
		identityQuery,
		versionFeatureQuery,
	} {
		if err := ValidateDefaultQuery(query); err != nil {
			t.Errorf("query rejected: %v\n%s", err, query)
		}
	}
}

func TestValidateDefaultQueryRejectsUnsafeSQL(t *testing.T) {
	tests := map[string]string{
		"WITH S AS (SELECT 1 FROM SYS.DUAL) SELECT * FROM S":    "must start",
		"UPDATE APP.T SET C = 1":                                "must start",
		"SELECT * FROM APP.T FOR UPDATE":                        "non-SELECT",
		"SELECT * FROM APP.T; DELETE FROM APP.T":                "separator",
		"SELECT * FROM APP.T -- hidden text":                    "comment",
		"SELECT CALL ttConfiguration('PermSize') FROM SYS.DUAL": "non-SELECT",
	}
	for query, want := range tests {
		err := ValidateDefaultQuery(query)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateDefaultQuery(%q) error = %v, want text %q", query, err, want)
		}
	}
}
