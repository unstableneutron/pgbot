package engine

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewReportKeepsEngineDataSeparate(t *testing.T) {
	collectedAt := time.Date(2026, 8, 17, 12, 0, 0, 0, time.FixedZone("test", -7*60*60))
	data := struct {
		Sessions int `json:"sessions"`
	}{Sessions: 7}
	report := NewReport(Oracle, Target{
		Identity:  "dbid:42/con:3",
		Database:  "ORCL",
		Container: "APPDB",
		Version:   "19.0.0.0.0",
	}, collectedAt, 2*time.Second, data)

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["schema_version"] != SchemaVersion {
		t.Fatalf("schema_version = %v, want %s", got["schema_version"], SchemaVersion)
	}
	if got["engine"] != string(Oracle) {
		t.Fatalf("engine = %v, want %s", got["engine"], Oracle)
	}
	if got["collected_at"] != "2026-08-17T19:00:00Z" {
		t.Fatalf("collected_at = %v, want UTC timestamp", got["collected_at"])
	}
	if findings, ok := got["findings"].([]any); !ok || len(findings) != 0 {
		t.Fatalf("findings = %#v, want empty array", got["findings"])
	}
	engineData, ok := got["engine_data"].(map[string]any)
	if !ok || engineData["sessions"] != float64(7) {
		t.Fatalf("engine_data = %#v", got["engine_data"])
	}
}
