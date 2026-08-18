package findings

import (
	"testing"

	"github.com/pgrundev/pgbot/internal/engine"
	"github.com/pgrundev/pgbot/internal/engine/timesten"
	"github.com/pgrundev/pgbot/internal/model"
)

func TestComputeFindsTimesTenRisksAndSorts(t *testing.T) {
	data := &timesten.Data{
		Health: &timesten.Health{
			Section:            engine.Section{Exactness: model.ExactnessSampled},
			TransactionsPerSec: 10, RollbackRatio: 0.4, LogBufferWaitsPerS: 2,
		},
		Locks: &timesten.Locks{
			Section:         engine.Section{Exactness: model.ExactnessSampled},
			DeadlocksPerSec: 1, TimeoutsPerSec: 1, WaitGrantsPerSec: 2,
		},
		Space: &timesten.Space{
			Section:   engine.Section{Exactness: model.ExactnessScraped},
			Permanent: timesten.SpacePool{AllocatedBytes: 1000, UsedBytes: 960, UsedRatio: 0.96},
		},
		Persistence: &timesten.Persistence{
			Section: engine.Section{Exactness: model.ExactnessScraped}, RequiredRecovery: true,
		},
		Tables: &timesten.Tables{
			Section: engine.Section{Exactness: model.ExactnessScraped},
			Rows:    []timesten.Table{{Owner: "APP", Name: "ORDERS", EstimatedRows: 100, MissingStatistics: true}},
		},
	}
	got := Compute(data)
	if len(got) != 6 {
		t.Fatalf("finding count = %d, want 6: %#v", len(got), got)
	}
	for index, finding := range got {
		if index > 0 && severityRank(got[index-1].Severity) < severityRank(finding.Severity) {
			t.Fatalf("findings are not severity sorted: %#v", got)
		}
		if finding.ID == "" || finding.Title == "" || finding.Detail == "" || finding.Confidence == 0 {
			t.Errorf("incomplete finding: %#v", finding)
		}
	}
}

func TestUnavailableSectionsDoNotProduceFindings(t *testing.T) {
	data := &timesten.Data{
		Space: &timesten.Space{
			Section:   engine.Section{Exactness: model.ExactnessUnavailable},
			Permanent: timesten.SpacePool{AllocatedBytes: 100, UsedBytes: 99, UsedRatio: 0.99},
		},
		Locks: &timesten.Locks{
			Section: engine.Section{Exactness: model.ExactnessUnavailable}, DeadlocksPerSec: 2,
		},
	}
	if got := Compute(data); len(got) != 0 {
		t.Fatalf("unavailable data produced findings: %#v", got)
	}
}

func TestMissingStatisticsUsesStableObjects(t *testing.T) {
	data := &timesten.Data{Tables: &timesten.Tables{
		Section: engine.Section{Exactness: model.ExactnessScraped},
		Rows: []timesten.Table{
			{Owner: "APP", Name: "A", EstimatedRows: 1, MissingStatistics: true},
			{Owner: "APP", Name: "B", EstimatedRows: 2, MissingStatistics: true},
		},
	}}
	got := Compute(data)
	if len(got) != 1 || len(got[0].Objects) != 2 || got[0].Objects[0] != "APP.A" {
		t.Fatalf("findings = %#v", got)
	}
}

func TestComputeNilReturnsNonNilSlice(t *testing.T) {
	if got := Compute(nil); got == nil || len(got) != 0 {
		t.Fatalf("Compute(nil) = %#v", got)
	}
}
