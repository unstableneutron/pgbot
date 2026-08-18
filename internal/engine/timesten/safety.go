package timesten

import (
	"fmt"
	"regexp"
	"strings"
)

var unsafeSQL = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|MERGE|CREATE|ALTER|DROP|TRUNCATE|GRANT|REVOKE|BEGIN|DECLARE|CALL|EXECUTE)\b`)

// ValidateDefaultQuery enforces the TimesTen collector boundary. It permits
// one plain SELECT statement and rejects comments, calls, and write syntax.
func ValidateDefaultQuery(query string) error {
	trimmed := strings.TrimSpace(query)
	upper := strings.ToUpper(trimmed)
	if trimmed == "" {
		return fmt.Errorf("TimesTen collector query is empty")
	}
	if !strings.HasPrefix(upper, "SELECT ") && !strings.HasPrefix(upper, "SELECT\n") && !strings.HasPrefix(upper, "SELECT\t") {
		return fmt.Errorf("TimesTen collector query must start with SELECT")
	}
	if strings.Contains(trimmed, ";") {
		return fmt.Errorf("TimesTen collector query contains a statement separator")
	}
	if strings.Contains(trimmed, "--") || strings.Contains(trimmed, "/*") {
		return fmt.Errorf("TimesTen collector query contains a SQL comment")
	}
	if unsafeSQL.MatchString(upper) || strings.Contains(upper, "FOR UPDATE") {
		return fmt.Errorf("TimesTen collector query contains a non-SELECT operation")
	}
	return nil
}
