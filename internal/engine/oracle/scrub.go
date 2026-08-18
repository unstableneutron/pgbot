package oracle

import (
	"regexp"
	"strings"
)

var (
	oracleSingleQuoted = regexp.MustCompile(`'(?:[^']|'')*'`)
	oracleEmail        = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	oracleUUID         = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	oracleNumberOrBind = regexp.MustCompile(`:\d+|\b\d+(?:\.\d+)?\b`)
)

// ScrubQueryText removes values from Oracle cursor text before it enters a
// report. Oracle alternative quoting must be removed before ordinary quoted
// strings because its body can contain unmatched single quotes.
func ScrubQueryText(query string) string {
	if query == "" {
		return query
	}
	query = strings.ReplaceAll(query, "\x00", "")
	scrubbed := scrubAlternativeQuotes(query)
	scrubbed = oracleSingleQuoted.ReplaceAllLiteralString(scrubbed, "'?'")
	scrubbed = oracleEmail.ReplaceAllLiteralString(scrubbed, "?")
	scrubbed = oracleUUID.ReplaceAllLiteralString(scrubbed, "?")
	scrubbed = oracleNumberOrBind.ReplaceAllStringFunc(scrubbed, func(value string) string {
		if value[0] == ':' {
			return value
		}
		return "?"
	})
	return scrubbed
}

func scrubAlternativeQuotes(query string) string {
	out := make([]byte, 0, len(query))
	for i := 0; i < len(query); {
		if i+2 >= len(query) || (query[i] != 'q' && query[i] != 'Q') || query[i+1] != '\'' {
			out = append(out, query[i])
			i++
			continue
		}

		open := query[i+2]
		close := open
		switch open {
		case '[':
			close = ']'
		case '{':
			close = '}'
		case '(':
			close = ')'
		case '<':
			close = '>'
		}

		end := i + 3
		for end+1 < len(query) && (query[end] != close || query[end+1] != '\'') {
			end++
		}
		out = append(out, query[i], '\'', '?', '\'')
		if end+1 >= len(query) {
			return string(out)
		}
		i = end + 2
	}
	return string(out)
}
