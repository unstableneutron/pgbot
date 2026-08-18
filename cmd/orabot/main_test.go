package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/pgrundev/pgbot/internal/model"
)

func TestHelpDescribesSeparateOracleCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := execute(context.Background(), []string{"--help"}, &stdout, &stderr)
	if code != exitClean {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	for _, want := range []string{"orabot", "Oracle Database 19c", "inspect"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("help missing %q: %q", want, stdout.String())
		}
	}
}

func TestInspectRejectsInvalidFlagValuesAsUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"inspect", "oracle://user:secret@db/service", "--format=xml"},
		{"inspect", "oracle://user:secret@db/service", "--fail-on=error"},
		{"inspect", "oracle://user:secret@db/service", "--interval=100ms"},
		{"inspect", "oracle://user:secret@db/service", "--interval=2s", "--timeout=1s"},
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

func TestCommandErrorsRedactOraclePasswords(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := execute(context.Background(), []string{"oracle://monitor:secret@db.example/ORCL"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "secret") || !strings.Contains(stderr.String(), "REDACTED") {
		t.Errorf("unsafe error: %q", stderr.String())
	}
}

func TestInlineIgnoreUsesCommonSuppressionSemantics(t *testing.T) {
	findings := []model.Finding{{
		ID:       "oracle_unusable_indexes",
		Severity: model.SeverityCritical,
		Title:    "2 Oracle indexes unusable",
		Evidence: []string{"APP.A", "APP.B"},
		Objects:  []string{"APP.A", "APP.B"},
	}}
	got := applyIgnores(findings, []string{"oracle_unusable_indexes:APP.A"})
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
