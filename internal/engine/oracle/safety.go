package oracle

import (
	"fmt"
	"regexp"
	"strings"
)

var mutatingSQL = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|MERGE|CREATE|ALTER|DROP|TRUNCATE|GRANT|REVOKE|BEGIN|DECLARE|CALL)\b`)

var licensedObjects = []string{
	"ACTIVE_SESSION_HISTORY",
	"DBA_HIST_",
	"DBMS_ADVISOR",
	"DBMS_AUTO_SQLTUNE",
	"DBMS_SQLPA",
	"DBMS_SQLTUNE",
	"DBMS_WORKLOAD_REPOSITORY",
	"SQL_MONITOR",
}

// ValidateDefaultQuery enforces the default Oracle collector boundary. It
// rejects writes, multiple statements, concealed SQL, and Diagnostics Pack or
// Tuning Pack objects before a query reaches the driver.
func ValidateDefaultQuery(query string) error {
	trimmed := strings.TrimSpace(query)
	upper := strings.ToUpper(trimmed)
	if trimmed == "" {
		return fmt.Errorf("Oracle collector query is empty")
	}
	if !strings.HasPrefix(upper, "SELECT ") && !strings.HasPrefix(upper, "SELECT\n") &&
		!strings.HasPrefix(upper, "WITH ") && !strings.HasPrefix(upper, "WITH\n") {
		return fmt.Errorf("Oracle collector query must start with SELECT or WITH")
	}
	if strings.Contains(trimmed, ";") {
		return fmt.Errorf("Oracle collector query contains a statement separator")
	}
	if strings.Contains(trimmed, "--") || strings.Contains(trimmed, "/*") {
		return fmt.Errorf("Oracle collector query contains a SQL comment")
	}
	if mutatingSQL.MatchString(upper) || strings.Contains(upper, "FOR UPDATE") {
		return fmt.Errorf("Oracle collector query contains a mutating operation")
	}
	for _, object := range licensedObjects {
		if strings.Contains(upper, object) {
			return fmt.Errorf("Oracle collector query uses licensed object %s", object)
		}
	}
	return nil
}
