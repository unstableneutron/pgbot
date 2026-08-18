package findings

import (
	"testing"

	"github.com/pgrundev/pgbot/internal/engine"
	"github.com/pgrundev/pgbot/internal/engine/oracle"
	"github.com/pgrundev/pgbot/internal/model"
)

func TestComputeFindsOracleRisksAndSortsBySeverity(t *testing.T) {
	data := &oracle.Data{
		Health: &oracle.Health{
			Section:          engine.Section{Exactness: model.ExactnessSampled},
			ParsesPerSec:     100,
			HardParsesPerSec: 40,
			HardParseRatio:   0.4,
		},
		Sessions: &oracle.Sessions{
			Section:     engine.Section{Exactness: model.ExactnessScraped},
			LongRunning: 2,
		},
		Locks: &oracle.Locks{
			Section: engine.Section{Exactness: model.ExactnessScraped},
			Blocked: []oracle.BlockedSession{{Instance: 1, SID: 8, BlockingSession: 9, SecondsInWait: 600}},
		},
		Resources: &oracle.Resources{
			Section: engine.Section{Exactness: model.ExactnessScraped},
			Limits:  []oracle.ResourceLimit{{Name: "sessions", CurrentUtilization: 97, Limit: 100}},
		},
		Storage: &oracle.Storage{
			Section:     engine.Section{Exactness: model.ExactnessScraped},
			Tablespaces: []oracle.Tablespace{{Name: "USERS", TotalBytes: 1000, MaxBytes: 1000, UsedBytes: 960}},
		},
		Tables: &oracle.Tables{
			Section: engine.Section{Exactness: model.ExactnessScraped},
			Rows:    []oracle.Table{{Owner: "APP", Name: "ORDERS", Rows: 1000, StaleStats: true}},
		},
		Indexes: &oracle.Indexes{
			Section: engine.Section{Exactness: model.ExactnessScraped},
			Rows:    []oracle.Index{{Owner: "APP", Name: "ORDERS_I", Status: "UNUSABLE"}},
		},
		Configuration: &oracle.Configuration{
			Section:    engine.Section{Exactness: model.ExactnessScraped},
			Parameters: map[string]oracle.Parameter{"statistics_level": {Value: "BASIC"}},
		},
		Recovery: &oracle.Recovery{
			Section: engine.Section{Exactness: model.ExactnessScraped},
			ArchiveDestinations: []oracle.ArchiveDestination{{
				ID: 2, Name: "LOG_ARCHIVE_DEST_2", Status: "ERROR", Target: "STANDBY", Error: "network error",
			}},
			DataGuardStats: map[string]string{"apply lag": "+00 00:10:00 day(2) to second(0) interval"},
		},
	}

	got := Compute(data)
	if len(got) != 10 {
		t.Fatalf("finding count = %d, want 10: %#v", len(got), got)
	}
	for i, finding := range got {
		if i > 0 && severityRank(got[i-1].Severity) < severityRank(finding.Severity) {
			t.Fatalf("findings are not severity sorted: %#v", got)
		}
		if finding.ID == "" || finding.Title == "" || finding.Detail == "" || finding.Confidence == 0 {
			t.Errorf("incomplete finding: %#v", finding)
		}
	}
	if got[0].Severity != model.SeverityCritical {
		t.Errorf("first severity = %q", got[0].Severity)
	}
}

func TestTablespacePressureUsesConfiguredMaximum(t *testing.T) {
	data := &oracle.Data{Storage: &oracle.Storage{
		Section: engine.Section{Exactness: model.ExactnessScraped},
		Tablespaces: []oracle.Tablespace{{
			Name: "USERS", TotalBytes: 100, MaxBytes: 1000, UsedBytes: 95, UsedRatio: 0.95,
		}},
	}}
	if got := Compute(data); len(got) != 0 {
		t.Fatalf("autoextend headroom should avoid pressure finding: %#v", got)
	}
}

func TestUnavailableSectionsDoNotProduceFindings(t *testing.T) {
	data := &oracle.Data{
		Locks: &oracle.Locks{
			Section: engine.Section{Exactness: model.ExactnessUnavailable},
			Blocked: []oracle.BlockedSession{{SID: 8}},
		},
		Indexes: &oracle.Indexes{
			Section: engine.Section{Exactness: model.ExactnessUnavailable},
			Rows:    []oracle.Index{{Status: "UNUSABLE"}},
		},
	}
	if got := Compute(data); len(got) != 0 {
		t.Fatalf("unavailable data produced findings: %#v", got)
	}
}

func TestParseIntervalSeconds(t *testing.T) {
	for _, test := range []struct {
		value string
		want  float64
		ok    bool
	}{
		{"+00 00:01:30 day(2) to second(0) interval", 90, true},
		{"+02 03:04:05.5", 183845.5, true},
		{"-00 00:00:01", -1, true},
		{"UNKNOWN", 0, false},
	} {
		got, ok := parseIntervalSeconds(test.value)
		if got != test.want || ok != test.ok {
			t.Errorf("parseIntervalSeconds(%q) = (%v, %v), want (%v, %v)", test.value, got, ok, test.want, test.ok)
		}
	}
}

func TestComputeNilReturnsNonNilSlice(t *testing.T) {
	if got := Compute(nil); got == nil || len(got) != 0 {
		t.Fatalf("Compute(nil) = %#v", got)
	}
}
