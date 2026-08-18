package timesten

import (
	"regexp"
	"strings"
)

var passwordAttribute = regexp.MustCompile(`(?i)(\b(?:PWD|PASSWORD)\s*=\s*)(\{(?:[^}]|}})*\}|[^;\s]*)`)

// RedactDSN removes password attributes from ODBC connection strings and
// errors before they reach terminal or machine output.
func RedactDSN(value string) string {
	return passwordAttribute.ReplaceAllString(value, `${1}REDACTED`)
}

var (
	timesTenSingleQuoted = regexp.MustCompile(`'(?:[^']|'')*'`)
	timesTenEmail        = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	timesTenUUID         = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	timesTenNumberOrBind = regexp.MustCompile(`:\d+|\b\d+(?:\.\d+)?\b`)
)

// ScrubQueryText removes literals from TTStats SQL text before reporting it.
func ScrubQueryText(query string) string {
	scrubbed := timesTenSingleQuoted.ReplaceAllLiteralString(query, "'?'")
	scrubbed = timesTenEmail.ReplaceAllLiteralString(scrubbed, "?")
	scrubbed = timesTenUUID.ReplaceAllLiteralString(scrubbed, "?")
	return timesTenNumberOrBind.ReplaceAllStringFunc(scrubbed, func(value string) string {
		if strings.HasPrefix(value, ":") {
			return value
		}
		return "?"
	})
}
