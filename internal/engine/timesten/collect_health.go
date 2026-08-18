package timesten

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

type healthSample map[string]int64

type healthCollector struct{ target *Target }

func (*healthCollector) Name() string            { return "health" }
func (*healthCollector) Kind() engine.SampleKind { return engine.Counter }

func (c *healthCollector) Sample(ctx context.Context) (any, error) {
	values := make(healthSample)
	err := c.target.query(ctx, healthSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var name string
			var value int64
			if err := rows.Scan(&name, &value); err != nil {
				return err
			}
			values[name] = value
		}
		return nil
	})
	return values, err
}

func (*healthCollector) Assemble(state *runState, pair engine.SamplePair, interval time.Duration) {
	if pair.Err != nil {
		section := unavailable(pair.Err)
		state.Data.Health = &Health{Section: section}
		state.Data.Connections = &Connections{Section: section}
		state.Data.Locks = &Locks{Section: section, ActiveRowsExcluded: true}
		return
	}
	before, ok := pair.A.(healthSample)
	if !ok {
		err := typeError("health", pair.A)
		section := unavailable(err)
		state.Data.Health = &Health{Section: section}
		state.Data.Connections = &Connections{Section: section}
		state.Data.Locks = &Locks{Section: section, ActiveRowsExcluded: true}
		return
	}
	after, ok := pair.B.(healthSample)
	if !ok || interval <= 0 {
		err := typeError("health second sample", pair.B)
		section := unavailable(err)
		state.Data.Health = &Health{Section: section}
		state.Data.Connections = &Connections{Section: section}
		state.Data.Locks = &Locks{Section: section, ActiveRowsExcluded: true}
		return
	}

	for name, first := range before {
		if second, found := after[name]; found && second < first {
			section := reset(fmt.Sprintf("TimesTen counter %s decreased during the sample", name))
			state.Data.Health = &Health{Section: section}
			state.Data.Connections = &Connections{Section: section}
			state.Data.Locks = &Locks{Section: section, ActiveRowsExcluded: true}
			return
		}
	}
	seconds := interval.Seconds()
	delta := func(name string) int64 { return after[name] - before[name] }
	rate := func(name string) float64 { return round2(float64(delta(name)) / seconds) }
	writes := delta("stmt.executes.alters") + delta("stmt.executes.creates") +
		delta("stmt.executes.deletes") + delta("stmt.executes.drops") +
		delta("stmt.executes.inserts") + delta("stmt.executes.merges") + delta("stmt.executes.updates")
	commits := delta("txn.commits.count")
	rollbacks := delta("txn.rollbacks")
	transactions := commits + rollbacks
	rollbackRatio := float64(0)
	if transactions > 0 {
		rollbackRatio = float64(rollbacks) / float64(transactions)
	}
	state.Data.Health = &Health{
		Section:            sampled("rates use two cumulative SYS.SYSTEMSTATS samples"),
		TransactionsPerSec: round2(float64(transactions) / seconds),
		CommitsPerSec:      rate("txn.commits.count"),
		RollbacksPerSec:    rate("txn.rollbacks"),
		RollbackRatio:      round2(rollbackRatio),
		SelectsPerSec:      rate("stmt.executes.selects"),
		WritesPerSec:       round2(float64(writes) / seconds),
		LogBytesPerSec:     rate("log.buffer.bytes_inserted"),
		LogBufferWaitsPerS: rate("log.buffer.waits"),
		CheckpointsPerSec:  rate("ckpt.completed"),
	}
	established := after["connections.established.count"]
	disconnected := after["connections.disconnected"]
	current := established - disconnected
	if current < 0 {
		current = 0
	}
	state.Data.Connections = &Connections{
		Section:                 cumulative("TimesTen exposes connection totals but not session rows through SELECT-only tables"),
		Current:                 current,
		Established:             established,
		Disconnected:            disconnected,
		ClientServerEstablished: after["connections.established.client_server"],
		EstablishedPerSec:       rate("connections.established.count"),
	}
	state.Data.Locks = &Locks{
		Section:            sampled("active lock rows require ttXactAdmin and are excluded from the SELECT-only boundary"),
		Deadlocks:          after["lock.deadlocks"],
		Timeouts:           after["lock.timeouts"],
		WaitGrants:         after["lock.locks_granted.wait"],
		DeadlocksPerSec:    rate("lock.deadlocks"),
		TimeoutsPerSec:     rate("lock.timeouts"),
		WaitGrantsPerSec:   rate("lock.locks_granted.wait"),
		ActiveRowsExcluded: true,
	}
}
