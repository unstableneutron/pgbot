package timesten

import (
	"strings"
	"testing"
)

func TestNormalizeDSNRequiresClientServerAndAddsTimeouts(t *testing.T) {
	dsn := "DRIVER={TimesTen 22.1 Client Driver};TTC_SERVER=db/6625;TTC_SERVER_DSN=prod;UID=monitor;PWD={a;b}"
	got, attributes, err := normalizeDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"SQLQUERYTIMEOUT=5;", "TTC_TIMEOUT=10;"} {
		if !strings.Contains(got, want) {
			t.Errorf("normalized DSN %q lacks %q", got, want)
		}
	}
	if attributes["PWD"] != "a;b" || attributes["TTC_SERVER_DSN"] != "prod" {
		t.Errorf("attributes = %#v", attributes)
	}
}

func TestNormalizeDSNRejectsUnsafeModesAndTimeouts(t *testing.T) {
	for _, dsn := range []string{
		"DRIVER={TimesTen};TTC_SERVER_DSN=prod",
		"DRIVER={TimesTen};TTC_SERVER=db/6625",
		"DRIVER={TimesTen};TTC_SERVER=db/6625;TTC_SERVER_DSN=prod;SQLQueryTimeout=0",
		"DRIVER={TimesTen};TTC_SERVER=db/6625;TTC_SERVER_DSN=prod;TTC_TIMEOUT=31",
	} {
		if _, _, err := normalizeDSN(dsn); err == nil {
			t.Errorf("normalizeDSN(%q) succeeded", dsn)
		}
	}
}

func TestRedactDSN(t *testing.T) {
	for _, test := range []struct {
		in   string
		leak string
	}{
		{"DSN=sample;UID=monitor;PWD=secret", "secret"},
		{"connect DRIVER={TimesTen};Password={s;ecret};UID=m failed", "s;ecret"},
	} {
		got := RedactDSN(test.in)
		if strings.Contains(got, test.leak) || !strings.Contains(got, "REDACTED") {
			t.Errorf("RedactDSN(%q) = %q", test.in, got)
		}
	}
}

func TestScrubQueryText(t *testing.T) {
	got := ScrubQueryText("SELECT * FROM APP.ORDERS WHERE EMAIL='alice@example.com' AND ID=42 AND TOKEN=:1")
	for _, leak := range []string{"alice", "example.com", "42"} {
		if strings.Contains(got, leak) {
			t.Errorf("literal %q leaked: %q", leak, got)
		}
	}
	for _, want := range []string{"APP.ORDERS", ":1"} {
		if !strings.Contains(got, want) {
			t.Errorf("query shape lost %q: %q", want, got)
		}
	}
}
