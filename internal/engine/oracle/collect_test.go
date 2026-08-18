package oracle

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
	"github.com/pgrundev/pgbot/internal/model"
)

func TestHealthCollectorAssemblesRates(t *testing.T) {
	before := healthSample{
		"user commits":          100,
		"user rollbacks":        100,
		"execute count":         100,
		"parse count (total)":   100,
		"parse count (hard)":    100,
		"physical reads":        100,
		"session logical reads": 100,
		"redo size":             100,
	}
	after := healthSample{
		"user commits":          120,
		"user rollbacks":        104,
		"execute count":         140,
		"parse count (total)":   120,
		"parse count (hard)":    102,
		"physical reads":        106,
		"session logical reads": 160,
		"redo size":             300,
	}
	state := &runState{}
	new(healthCollector).Assemble(state, engine.SamplePair{A: before, B: after}, 2*time.Second)

	got := state.Data.Health
	if got == nil || got.Exactness != model.ExactnessSampled {
		t.Fatalf("health section = %#v", got)
	}
	if got.TransactionsPerSec != 12 || got.ExecutionsPerSec != 20 || got.HardParseRatio != 0.1 {
		t.Errorf("health rates = %#v", got)
	}
}

func TestHealthCollectorMarksCounterReset(t *testing.T) {
	before := healthSample{
		"user commits": 2, "user rollbacks": 2, "execute count": 2,
		"parse count (total)": 2, "parse count (hard)": 2, "physical reads": 2,
		"session logical reads": 2, "redo size": 2,
	}
	after := healthSample{
		"user commits": 1, "user rollbacks": 3, "execute count": 3,
		"parse count (total)": 3, "parse count (hard)": 3, "physical reads": 3,
		"session logical reads": 3, "redo size": 3,
	}
	state := &runState{}
	new(healthCollector).Assemble(state, engine.SamplePair{A: before, B: after}, time.Second)

	if got := state.Data.Health; got == nil || got.Exactness != model.ExactnessReset {
		t.Fatalf("health section = %#v", got)
	}
}

func TestGaugeCollectorsAssembleTypedSamples(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	state := &runState{}

	new(sessionsCollector).Assemble(state, engine.SamplePair{A: sessionsSample{Total: 8, Active: 2}}, 0)
	new(locksCollector).Assemble(state, engine.SamplePair{A: []BlockedSession{{SID: 10}}}, 0)
	new(topSQLCollector).Assemble(state, engine.SamplePair{A: []SQLStatement{{SQLID: "abc123"}}}, 0)
	new(storageCollector).Assemble(state, engine.SamplePair{A: []Tablespace{{Name: "USERS", UsedRatio: 0.75}}}, 0)
	new(memoryCollector).Assemble(state, engine.SamplePair{A: memorySample{SGABytes: 1024, PGABytes: 512}}, 0)
	new(resourcesCollector).Assemble(state, engine.SamplePair{A: []ResourceLimit{{Name: "sessions", Limit: 100}}}, 0)
	new(tablesCollector).Assemble(state, engine.SamplePair{A: tablesSample{Rows: []Table{{Owner: "APP", Name: "ORDERS", LastAnalyzed: &now}}}}, 0)
	new(indexesCollector).Assemble(state, engine.SamplePair{A: indexesSample{Rows: []Index{{Owner: "APP", Name: "ORDERS_PK"}}}}, 0)
	new(configurationCollector).Assemble(state, engine.SamplePair{A: map[string]Parameter{"processes": {Value: "300"}}}, 0)
	new(recoveryCollector).Assemble(state, engine.SamplePair{A: recoverySample{
		DatabaseRole: "PRIMARY",
		DataGuardStats: map[string]string{
			"apply lag": "+00 00:00:00 day(2) to second(0) interval",
		},
	}}, 0)

	if state.Data.Sessions == nil || state.Data.Sessions.Total != 8 {
		t.Errorf("sessions = %#v", state.Data.Sessions)
	}
	if state.Data.Locks == nil || len(state.Data.Locks.Blocked) != 1 {
		t.Errorf("locks = %#v", state.Data.Locks)
	}
	if state.Data.TopSQL == nil || state.Data.TopSQL.Exactness != model.ExactnessCumulative {
		t.Errorf("top SQL = %#v", state.Data.TopSQL)
	}
	if state.Data.Storage == nil || state.Data.Storage.Tablespaces[0].Name != "USERS" {
		t.Errorf("storage = %#v", state.Data.Storage)
	}
	if state.Data.Memory == nil || state.Data.Memory.SGABytes != 1024 {
		t.Errorf("memory = %#v", state.Data.Memory)
	}
	if state.Data.Resources == nil || state.Data.Resources.Limits[0].Limit != 100 {
		t.Errorf("resources = %#v", state.Data.Resources)
	}
	if state.Data.Tables == nil || !state.Data.Tables.Rows[0].LastAnalyzed.Equal(now) {
		t.Errorf("tables = %#v", state.Data.Tables)
	}
	if state.Data.Indexes == nil || state.Data.Indexes.Rows[0].Name != "ORDERS_PK" {
		t.Errorf("indexes = %#v", state.Data.Indexes)
	}
	if state.Data.Configuration == nil || state.Data.Configuration.Parameters["processes"].Value != "300" {
		t.Errorf("configuration = %#v", state.Data.Configuration)
	}
	if state.Data.Recovery == nil || state.Data.Recovery.DatabaseRole != "PRIMARY" {
		t.Errorf("recovery = %#v", state.Data.Recovery)
	}
}

func TestCollectorFailureIsLocalAndRedacted(t *testing.T) {
	state := &runState{}
	wantErr := errors.New("connect oracle://monitor:secret@db.example/ORCL failed")
	new(storageCollector).Assemble(state, engine.SamplePair{Err: wantErr}, 0)

	if state.Data.Storage == nil || state.Data.Storage.Exactness != model.ExactnessUnavailable {
		t.Fatalf("storage = %#v", state.Data.Storage)
	}
	if strings.Contains(state.Data.Storage.Reason, "secret") || !strings.Contains(state.Data.Storage.Reason, "REDACTED") {
		t.Errorf("unsafe failure reason %q", state.Data.Storage.Reason)
	}
}

func TestSchemaCollectorsPreserveTruncationSignal(t *testing.T) {
	state := &runState{}
	new(tablesCollector).Assemble(state, engine.SamplePair{A: tablesSample{Truncated: true}}, 0)
	new(indexesCollector).Assemble(state, engine.SamplePair{A: indexesSample{Truncated: true}}, 0)

	if state.Data.Tables == nil || !state.Data.Tables.Truncated || state.Data.Tables.Rows == nil {
		t.Errorf("tables = %#v", state.Data.Tables)
	}
	if state.Data.Indexes == nil || !state.Data.Indexes.Truncated || state.Data.Indexes.Rows == nil {
		t.Errorf("indexes = %#v", state.Data.Indexes)
	}
}

func TestInspectRejectsNilTarget(t *testing.T) {
	if _, err := Inspect(context.Background(), nil, InspectOptions{}); err == nil {
		t.Fatal("expected nil-target error")
	}
}
