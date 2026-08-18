package oracle

import (
	"strings"
	"testing"
)

func TestScrubQueryTextRemovesOracleLiterals(t *testing.T) {
	tests := []struct {
		name  string
		query string
		leaks []string
	}{
		{
			name:  "ordinary literal",
			query: "SELECT * FROM customers WHERE email = 'alice@example.com' AND id = 42",
			leaks: []string{"alice", "example.com", "42"},
		},
		{
			name:  "paired alternative quote",
			query: "SELECT q'[O'Brien alice@example.com 8675309]' FROM dual",
			leaks: []string{"O'Brien", "alice", "example.com", "8675309"},
		},
		{
			name:  "custom alternative quote",
			query: "SELECT Q'!token=very-secret!' FROM dual",
			leaks: []string{"very-secret"},
		},
		{
			name:  "unterminated alternative quote",
			query: "SELECT q'[private-data 99",
			leaks: []string{"private-data", "99"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ScrubQueryText(test.query)
			for _, leak := range test.leaks {
				if strings.Contains(got, leak) {
					t.Errorf("literal %q leaked: %q", leak, got)
				}
			}
		})
	}
}

func TestScrubQueryTextPreservesQueryShapeAndBinds(t *testing.T) {
	got := ScrubQueryText("SELECT col1 FROM orders WHERE id = :1 AND status = :status AND amount > 120.50")
	for _, want := range []string{"SELECT col1", "orders", ":1", ":status"} {
		if !strings.Contains(got, want) {
			t.Errorf("query shape lost %q: %q", want, got)
		}
	}
	if strings.Contains(got, "120.50") {
		t.Errorf("numeric literal leaked: %q", got)
	}
}
