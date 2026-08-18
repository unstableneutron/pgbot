package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
	"github.com/pgrundev/pgbot/internal/engine/timesten"
	"github.com/pgrundev/pgbot/internal/model"
)

func testReport() *engine.Report {
	data := &timesten.Data{
		Health: &timesten.Health{Section: engine.Section{Exactness: model.ExactnessSampled}},
		Connections: &timesten.Connections{
			Section: engine.Section{Exactness: model.ExactnessCumulative, Reason: "session rows excluded"}, Current: 2,
		},
		Locks: &timesten.Locks{Section: engine.Section{Exactness: model.ExactnessSampled, Reason: "active rows excluded"}},
		TopSQL: &timesten.TopSQL{Section: engine.Section{Exactness: model.ExactnessCumulative}, Statements: []timesten.SQLStatement{{
			SQLHash: "abc", SampleText: "SELECT * FROM APP.T WHERE TOKEN='secret'", Executions: 4,
		}}},
		Space:       &timesten.Space{Section: engine.Section{Exactness: model.ExactnessScraped}},
		Persistence: &timesten.Persistence{Section: engine.Section{Exactness: model.ExactnessScraped}},
		Tables:      &timesten.Tables{Section: engine.Section{Exactness: model.ExactnessScraped}, Rows: []timesten.Table{}},
		Indexes:     &timesten.Indexes{Section: engine.Section{Exactness: model.ExactnessScraped}, Rows: []timesten.Index{}},
		Configuration: &timesten.Configuration{Section: engine.Section{
			Exactness: model.ExactnessUnavailable, Reason: "configuration needs a built-in procedure",
		}},
		Replication: &timesten.Replication{Section: engine.Section{Exactness: model.ExactnessScraped}},
	}
	return engine.NewReport(engine.TimesTen, engine.Target{
		Identity: "timesten:sampledb", Database: "sampledb", Version: "22.1-compatible", Topology: "classic-client-server",
	}, time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC), time.Second, data)
}

func TestTerminalRendersTimesTenSectionsAndRedactsSQL(t *testing.T) {
	var output bytes.Buffer
	if err := Terminal(&output, testReport(), Options{Full: true}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{"TimesTen 22.1-compatible", "Classic client/server", "CONNECTIONS", "configuration needs a built-in procedure"} {
		if !strings.Contains(text, want) {
			t.Errorf("terminal output lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "secret") {
		t.Errorf("terminal output leaked SQL literal:\n%s", text)
	}
}

func TestJSONWritesVersionedEnvelope(t *testing.T) {
	var output bytes.Buffer
	if err := JSON(&output, testReport()); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{`"schema_version": "2.0.0"`, `"engine": "timesten"`, `"engine_data"`} {
		if !strings.Contains(text, want) {
			t.Errorf("JSON lacks %q:\n%s", want, text)
		}
	}
}

func TestTerminalRejectsWrongEngine(t *testing.T) {
	report := testReport()
	report.Engine = engine.Oracle
	if err := Terminal(&bytes.Buffer{}, report, Options{}); err == nil {
		t.Fatal("expected wrong-engine error")
	}
}
