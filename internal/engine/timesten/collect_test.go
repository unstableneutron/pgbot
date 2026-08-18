package timesten

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
	"github.com/pgrundev/pgbot/internal/model"
)

func TestHealthCollectorAssemblesTimesTenSections(t *testing.T) {
	before := healthSample{
		"txn.commits.count": 100, "txn.rollbacks": 10,
		"stmt.executes.selects": 200, "stmt.executes.inserts": 4,
		"log.buffer.bytes_inserted": 1000, "log.buffer.waits": 1,
		"ckpt.completed": 2, "connections.established.count": 20,
		"connections.disconnected": 15, "connections.established.client_server": 18,
		"lock.deadlocks": 0, "lock.timeouts": 1, "lock.locks_granted.wait": 2,
	}
	after := healthSample{
		"txn.commits.count": 120, "txn.rollbacks": 14,
		"stmt.executes.selects": 240, "stmt.executes.inserts": 8,
		"log.buffer.bytes_inserted": 1400, "log.buffer.waits": 3,
		"ckpt.completed": 4, "connections.established.count": 24,
		"connections.disconnected": 17, "connections.established.client_server": 22,
		"lock.deadlocks": 1, "lock.timeouts": 2, "lock.locks_granted.wait": 6,
	}
	state := &runState{}
	new(healthCollector).Assemble(state, engine.SamplePair{A: before, B: after}, 2*time.Second)

	if state.Data.Health == nil || state.Data.Health.TransactionsPerSec != 12 || state.Data.Health.SelectsPerSec != 20 {
		t.Errorf("health = %#v", state.Data.Health)
	}
	if state.Data.Connections == nil || state.Data.Connections.Current != 7 || state.Data.Connections.EstablishedPerSec != 2 {
		t.Errorf("connections = %#v", state.Data.Connections)
	}
	if state.Data.Locks == nil || state.Data.Locks.DeadlocksPerSec != 0.5 || !state.Data.Locks.ActiveRowsExcluded {
		t.Errorf("locks = %#v", state.Data.Locks)
	}
}

func TestHealthCollectorMarksCounterReset(t *testing.T) {
	state := &runState{}
	new(healthCollector).Assemble(state, engine.SamplePair{
		A: healthSample{"txn.commits.count": 2},
		B: healthSample{"txn.commits.count": 1},
	}, time.Second)
	if state.Data.Health == nil || state.Data.Health.Exactness != model.ExactnessReset {
		t.Fatalf("health = %#v", state.Data.Health)
	}
}

func TestGaugeCollectorsAssembleTypedSamples(t *testing.T) {
	state := &runState{}
	new(spaceCollector).Assemble(state, engine.SamplePair{A: spaceSample{
		PermAllocated: 100, PermUsed: 80, PermHighWater: 90,
		TempAllocated: 50, TempUsed: 10, TempHighWater: 20,
		Recovery: 1, FirstLogFile: 2, LastLogFile: 3, ReplicationLog: 1,
	}}, 0)
	new(tablesCollector).Assemble(state, engine.SamplePair{A: tablesSample{Rows: []Table{{Owner: "APP", Name: "T"}}}}, 0)
	new(indexesCollector).Assemble(state, engine.SamplePair{A: indexesSample{Rows: []Index{{Owner: "APP", Name: "T_PK"}}}}, 0)
	new(topSQLCollector).Assemble(state, engine.SamplePair{A: []SQLStatement{{CommandID: 7}}}, 0)
	new(replicationCollector).Assemble(state, engine.SamplePair{A: replicationSample{
		Definitions: []ReplicationDefinition{{Owner: "APP", Name: "REP"}},
		Peers:       []ReplicationPeer{{Owner: "APP", Name: "REP", SubscriberID: 2}},
	}}, 0)

	if state.Data.Space == nil || state.Data.Space.Permanent.UsedRatio != 0.8 || state.Data.Space.Permanent.AllocatedBytes != 100*1024 {
		t.Errorf("space = %#v", state.Data.Space)
	}
	if state.Data.Persistence == nil || !state.Data.Persistence.RequiredRecovery {
		t.Errorf("persistence = %#v", state.Data.Persistence)
	}
	if state.Data.Tables == nil || len(state.Data.Tables.Rows) != 1 || state.Data.Indexes == nil || len(state.Data.Indexes.Rows) != 1 {
		t.Errorf("schema = %#v, %#v", state.Data.Tables, state.Data.Indexes)
	}
	if state.Data.TopSQL == nil || state.Data.TopSQL.Exactness != model.ExactnessCumulative {
		t.Errorf("top SQL = %#v", state.Data.TopSQL)
	}
	if state.Data.Replication == nil || len(state.Data.Replication.Definitions) != 1 {
		t.Errorf("replication = %#v", state.Data.Replication)
	}
}

func TestTopSQLWithoutTTStatsIsUnavailable(t *testing.T) {
	state := &runState{}
	new(topSQLCollector).Assemble(state, engine.SamplePair{A: []SQLStatement{}}, 0)
	if state.Data.TopSQL == nil || state.Data.TopSQL.Exactness != model.ExactnessUnavailable || !strings.Contains(state.Data.TopSQL.Reason, "TTStats") {
		t.Fatalf("top SQL = %#v", state.Data.TopSQL)
	}
}

func TestCollectorFailureIsLocalAndRedacted(t *testing.T) {
	state := &runState{}
	wantErr := errors.New("connect DRIVER={TimesTen};PWD=secret failed")
	new(spaceCollector).Assemble(state, engine.SamplePair{Err: wantErr}, 0)
	if state.Data.Space == nil || state.Data.Space.Exactness != model.ExactnessUnavailable {
		t.Fatalf("space = %#v", state.Data.Space)
	}
	if strings.Contains(state.Data.Space.Reason, "secret") || !strings.Contains(state.Data.Space.Reason, "REDACTED") {
		t.Errorf("unsafe reason %q", state.Data.Space.Reason)
	}
}

func TestSchemaCollectorsPreserveTruncation(t *testing.T) {
	state := &runState{}
	new(tablesCollector).Assemble(state, engine.SamplePair{A: tablesSample{Rows: []Table{}, Truncated: true}}, 0)
	new(indexesCollector).Assemble(state, engine.SamplePair{A: indexesSample{Rows: []Index{}, Truncated: true}}, 0)
	if state.Data.Tables == nil || !state.Data.Tables.Truncated || state.Data.Tables.Rows == nil {
		t.Errorf("tables = %#v", state.Data.Tables)
	}
	if state.Data.Indexes == nil || !state.Data.Indexes.Truncated || state.Data.Indexes.Rows == nil {
		t.Errorf("indexes = %#v", state.Data.Indexes)
	}
}

func TestBinaryFlag(t *testing.T) {
	if binaryFlag([]byte{0}) || !binaryFlag([]byte{1}) {
		t.Fatal("binary flag conversion failed")
	}
}
