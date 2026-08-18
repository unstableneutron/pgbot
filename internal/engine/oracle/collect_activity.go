package oracle

import (
	"context"
	"database/sql"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

type sessionsSample struct {
	Total       int64
	Active      int64
	Inactive    int64
	Blocked     int64
	LongRunning int64
}

type sessionsCollector struct{ target *Target }

func (*sessionsCollector) Name() string            { return "sessions" }
func (*sessionsCollector) Kind() engine.SampleKind { return engine.Gauge }
func (c *sessionsCollector) Sample(ctx context.Context) (any, error) {
	var out sessionsSample
	var active, inactive, blocked, longRunning sql.NullInt64
	err := c.target.queryRow(ctx, sessionsSQL, nil, func(row *sql.Row) error {
		return row.Scan(&out.Total, &active, &inactive, &blocked, &longRunning)
	})
	out.Active = active.Int64
	out.Inactive = inactive.Int64
	out.Blocked = blocked.Int64
	out.LongRunning = longRunning.Int64
	return out, err
}
func (*sessionsCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.Sessions = &Sessions{Section: unavailable(pair.Err)}
		return
	}
	sample, ok := pair.A.(sessionsSample)
	if !ok {
		state.Data.Sessions = &Sessions{Section: unavailable(typeError("sessions", pair.A))}
		return
	}
	state.Data.Sessions = &Sessions{
		Section: scraped(), Total: sample.Total, Active: sample.Active, Inactive: sample.Inactive,
		Blocked: sample.Blocked, LongRunning: sample.LongRunning,
	}
}

type locksCollector struct{ target *Target }

func (*locksCollector) Name() string            { return "locks" }
func (*locksCollector) Kind() engine.SampleKind { return engine.Gauge }
func (c *locksCollector) Sample(ctx context.Context) (any, error) {
	var out []BlockedSession
	err := c.target.query(ctx, locksSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var row BlockedSession
			var blockingInstance, blockingSession, finalInstance, finalSession sql.NullInt64
			if err := rows.Scan(
				&row.Instance, &row.SID, &row.Serial, &row.Username, &row.Status, &row.WaitClass,
				&row.Event, &row.SecondsInWait, &blockingInstance, &blockingSession,
				&finalInstance, &finalSession,
			); err != nil {
				return err
			}
			row.BlockingInstance = int(blockingInstance.Int64)
			row.BlockingSession = int(blockingSession.Int64)
			row.FinalBlockingInstance = int(finalInstance.Int64)
			row.FinalBlockingSession = int(finalSession.Int64)
			out = append(out, row)
		}
		return nil
	})
	return out, err
}
func (*locksCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.Locks = &Locks{Section: unavailable(pair.Err)}
		return
	}
	rows, ok := pair.A.([]BlockedSession)
	if !ok {
		state.Data.Locks = &Locks{Section: unavailable(typeError("locks", pair.A))}
		return
	}
	if rows == nil {
		rows = []BlockedSession{}
	}
	state.Data.Locks = &Locks{Section: scraped(), Blocked: rows}
}
