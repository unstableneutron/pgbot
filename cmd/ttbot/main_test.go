//go:build timesten && cgo

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/pgrundev/pgbot/internal/model"
)

func TestHelpDescribesSeparateTimesTenCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := execute(context.Background(), []string{"--help"}, &stdout, &stderr)
	if code != exitClean {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	for _, want := range []string{"ttbot", "TimesTen 22.1 Classic", "inspect"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("help missing %q: %q", want, stdout.String())
		}
	}
}

func TestInspectRejectsInvalidFlagsAsUsageErrors(t *testing.T) {
	dsn := "DRIVER={TimesTen};TTC_SERVER=db/6625;TTC_SERVER_DSN=sample;UID=monitor;PWD=secret"
	for _, args := range [][]string{
		{"inspect", dsn, "--format=xml"},
		{"inspect", dsn, "--fail-on=error"},
		{"inspect", dsn, "--interval=100ms"},
		{"inspect", dsn, "--interval=2s", "--timeout=1s"},
	} {
		var stdout, stderr bytes.Buffer
		if code := execute(context.Background(), args, &stdout, &stderr); code != exitUsage {
			t.Errorf("execute(%q) exit = %d, stderr = %q", args, code, stderr.String())
		}
		if strings.Contains(stderr.String(), "secret") {
			t.Errorf("password leaked for %q: %q", args, stderr.String())
		}
	}
}

func TestCommandErrorsRedactTimesTenPasswords(t *testing.T) {
	var stdout, stderr bytes.Buffer
	dsn := "DRIVER={TimesTen};TTC_SERVER=db/6625;TTC_SERVER_DSN=sample;PWD=secret"
	code := execute(context.Background(), []string{dsn}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "secret") || !strings.Contains(stderr.String(), "REDACTED") {
		t.Errorf("unsafe error: %q", stderr.String())
	}
}

func TestInlineIgnoreUsesCommonSuppressionSemantics(t *testing.T) {
	findings := []model.Finding{{
		ID:       "timesten_missing_table_statistics",
		Severity: model.SeverityWarn,
		Title:    "2 TimesTen tables lack optimizer statistics",
		Evidence: []string{"APP.A", "APP.B"},
		Objects:  []string{"APP.A", "APP.B"},
	}}
	got := applyIgnores(findings, []string{"timesten_missing_table_statistics:APP.A"})
	if got[0].Suppressed || len(got[0].Objects) != 1 || got[0].Objects[0] != "APP.B" {
		t.Fatalf("partial suppression = %#v", got[0])
	}
	if !strings.HasPrefix(got[0].Title, "1 ") {
		t.Errorf("partial suppression title = %q", got[0].Title)
	}
}

func TestFindingExitCode(t *testing.T) {
	critical := model.Finding{Severity: model.SeverityCritical}
	warning := model.Finding{Severity: model.SeverityWarn}
	suppressedCritical := model.Finding{Severity: model.SeverityCritical, Suppressed: true}

	for _, test := range []struct {
		findings []model.Finding
		failOn   string
		want     int
	}{
		{nil, "warn", exitClean},
		{[]model.Finding{warning}, "critical", exitClean},
		{[]model.Finding{warning}, "warn", exitWarn},
		{[]model.Finding{critical}, "warn", exitCritical},
		{[]model.Finding{suppressedCritical}, "warn", exitClean},
		{[]model.Finding{critical}, "none", exitClean},
	} {
		if got := findingExitCode(test.findings, test.failOn); got != test.want {
			t.Errorf("findingExitCode(%#v, %q) = %d, want %d", test.findings, test.failOn, got, test.want)
		}
	}
}
