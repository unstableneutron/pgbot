package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
	"github.com/pgrundev/pgbot/internal/engine/oracle"
	"github.com/pgrundev/pgbot/internal/model"
)

func sampleReport() *engine.Report {
	data := &oracle.Data{
		Health: &oracle.Health{
			Section:            engine.Section{Exactness: model.ExactnessSampled},
			TransactionsPerSec: 10,
			ExecutionsPerSec:   50,
			ParsesPerSec:       5,
			HardParseRatio:     0.1,
			RedoBytesPerSec:    1024,
		},
		Sessions: &oracle.Sessions{
			Section: engine.Section{Exactness: model.ExactnessScraped}, Total: 12, Active: 2, Inactive: 10,
		},
		Locks: &oracle.Locks{
			Section: engine.Section{Exactness: model.ExactnessScraped},
			Blocked: []oracle.BlockedSession{{Instance: 1, SID: 8, BlockingSession: 9, SecondsInWait: 12, Event: "enq: TX"}},
		},
		TopSQL: &oracle.TopSQL{
			Section: engine.Section{Exactness: model.ExactnessCumulative},
			Statements: []oracle.SQLStatement{{
				SQLID: "abc123", Executions: 8, ElapsedSeconds: 2, CPUSeconds: 1, SampleText: "SELECT * FROM orders WHERE id = :1",
			}},
		},
		Storage: &oracle.Storage{
			Section:     engine.Section{Exactness: model.ExactnessScraped},
			Tablespaces: []oracle.Tablespace{{Name: "USERS", UsedBytes: 75, TotalBytes: 100, MaxBytes: 200, UsedRatio: 0.75}},
		},
		Memory: &oracle.Memory{
			Section: engine.Section{Exactness: model.ExactnessScraped}, SGABytes: 4 << 30, PGABytes: 1 << 30,
		},
		Resources: &oracle.Resources{
			Section: engine.Section{Exactness: model.ExactnessScraped},
			Limits:  []oracle.ResourceLimit{{Name: "sessions", CurrentUtilization: 12, Limit: 100}},
		},
		Tables: &oracle.Tables{Section: engine.Section{Exactness: model.ExactnessScraped}},
		Indexes: &oracle.Indexes{
			Section: engine.Section{Exactness: model.ExactnessScraped},
			Rows:    []oracle.Index{{Name: "ORDERS_I", Status: "UNUSABLE"}},
		},
		Configuration: &oracle.Configuration{
			Section:    engine.Section{Exactness: model.ExactnessScraped},
			Parameters: map[string]oracle.Parameter{"statistics_level": {Value: "TYPICAL", Default: true}},
		},
		Recovery: &oracle.Recovery{
			Section: engine.Section{Exactness: model.ExactnessScraped}, DatabaseRole: "PRIMARY", OpenMode: "READ WRITE",
			ProtectionMode: "MAXIMUM PERFORMANCE", ProtectionLevel: "MAXIMUM PERFORMANCE", SwitchoverStatus: "NOT ALLOWED",
			ArchiveDestinations: []oracle.ArchiveDestination{{Name: "LOG_ARCHIVE_DEST_1", Status: "VALID", Target: "PRIMARY"}},
			DataGuardStats:      map[string]string{},
		},
	}
	report := engine.NewReport(engine.Oracle, engine.Target{
		Identity: "dbid:42/con:3", Database: "ORCL", Container: "APP", Version: "19.0.0.0.0",
	}, time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC), time.Second, data)
	report.Findings = []model.Finding{{
		ID: "oracle_unusable_indexes", Severity: model.SeverityCritical, Title: "1 Oracle index unusable",
		Detail: "An unusable index cannot support normal access.", Evidence: []string{"APP.ORDERS_I"},
	}}
	return report
}

func TestTerminalRendersOracleSummaryAndFullDetails(t *testing.T) {
	for _, full := range []bool{false, true} {
		var output bytes.Buffer
		if err := Terminal(&output, sampleReport(), Options{Full: full}); err != nil {
			t.Fatal(err)
		}
		got := output.String()
		for _, want := range []string{"connected", "ORCL/APP", "oracle 19.0.0.0.0", "read-only", "CRITICAL", "HEALTH", "SESSIONS", "STORAGE", "RECOVERY"} {
			if !strings.Contains(got, want) {
				t.Errorf("full=%v output missing %q", full, want)
			}
		}
		if full {
			for _, want := range []string{"LOCKS", "TOP SQL", "abc123", "MEMORY", "TABLES", "INDEXES", "CONFIGURATION"} {
				if !strings.Contains(got, want) {
					t.Errorf("full output missing %q", want)
				}
			}
		} else if strings.Contains(got, "TOP SQL") {
			t.Error("summary output contains full SQL section")
		}
	}
}

func TestJSONWritesVersionedOracleEnvelope(t *testing.T) {
	var output bytes.Buffer
	if err := JSON(&output, sampleReport()); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["schema_version"] != engine.SchemaVersion || decoded["engine"] != "oracle" {
		t.Fatalf("unexpected envelope: %#v", decoded)
	}
	if decoded["engine_data"] == nil {
		t.Fatal("engine_data is missing")
	}
}

func TestTerminalRejectsWrongEngineData(t *testing.T) {
	report := engine.NewReport(engine.TimesTen, engine.Target{}, time.Now(), 0, struct{}{})
	if err := Terminal(&bytes.Buffer{}, report, Options{}); err == nil {
		t.Fatal("expected wrong-engine error")
	}
}
