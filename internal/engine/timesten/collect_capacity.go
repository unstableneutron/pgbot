package timesten

import (
	"context"
	"database/sql"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

type spaceSample struct {
	PermAllocated  int64
	PermUsed       int64
	PermHighWater  int64
	TempAllocated  int64
	TempUsed       int64
	TempHighWater  int64
	Recovery       int
	FirstLogFile   int64
	LastLogFile    int64
	ReplicationLog int64
}

type spaceCollector struct{ target *Target }

func (*spaceCollector) Name() string            { return "space" }
func (*spaceCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *spaceCollector) Sample(ctx context.Context) (any, error) {
	var value spaceSample
	err := c.target.queryRow(ctx, spaceSQL, func(row *sql.Row) error {
		return row.Scan(
			&value.PermAllocated, &value.PermUsed, &value.PermHighWater,
			&value.TempAllocated, &value.TempUsed, &value.TempHighWater,
			&value.Recovery, &value.FirstLogFile, &value.LastLogFile, &value.ReplicationLog,
		)
	})
	return value, err
}

func (*spaceCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		section := unavailable(pair.Err)
		state.Data.Space = &Space{Section: section}
		state.Data.Persistence = &Persistence{Section: section}
		return
	}
	value, ok := pair.A.(spaceSample)
	if !ok {
		section := unavailable(typeError("space", pair.A))
		state.Data.Space = &Space{Section: section}
		state.Data.Persistence = &Persistence{Section: section}
		return
	}
	const kibibyte = int64(1024)
	state.Data.Space = &Space{
		Section: scraped(),
		Permanent: SpacePool{
			AllocatedBytes: value.PermAllocated * kibibyte,
			UsedBytes:      value.PermUsed * kibibyte,
			HighWaterBytes: value.PermHighWater * kibibyte,
			UsedRatio:      round2(ratio(value.PermUsed, value.PermAllocated)),
			HighWaterRatio: round2(ratio(value.PermHighWater, value.PermAllocated)),
		},
		Temporary: SpacePool{
			AllocatedBytes: value.TempAllocated * kibibyte,
			UsedBytes:      value.TempUsed * kibibyte,
			HighWaterBytes: value.TempHighWater * kibibyte,
			UsedRatio:      round2(ratio(value.TempUsed, value.TempAllocated)),
			HighWaterRatio: round2(ratio(value.TempHighWater, value.TempAllocated)),
		},
	}
	state.Data.Persistence = &Persistence{
		Section:          scraped(),
		RequiredRecovery: value.Recovery != 0,
		FirstLogFile:     value.FirstLogFile,
		LastLogFile:      value.LastLogFile,
		ReplicationHold:  value.ReplicationLog,
	}
}
