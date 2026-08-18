package oracle

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
	out := make(healthSample)
	err := c.target.query(ctx, healthSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var name string
			var value int64
			if err := rows.Scan(&name, &value); err != nil {
				return err
			}
			out[name] = value
		}
		return nil
	})
	return out, err
}

func (*healthCollector) Assemble(state *runState, pair engine.SamplePair, dt time.Duration) {
	if pair.Err != nil {
		state.Data.Health = &Health{Section: unavailable(pair.Err)}
		return
	}
	a, ok := pair.A.(healthSample)
	if !ok {
		state.Data.Health = &Health{Section: unavailable(typeError("health", pair.A))}
		return
	}
	b, ok := pair.B.(healthSample)
	if !ok {
		state.Data.Health = &Health{Section: unavailable(typeError("health", pair.B))}
		return
	}
	if dt <= 0 {
		state.Data.Health = &Health{Section: unavailable(fmt.Errorf("health collector sample interval is not positive"))}
		return
	}
	rate := func(name string) (float64, error) {
		before, beforeOK := a[name]
		after, afterOK := b[name]
		if !beforeOK || !afterOK {
			return 0, fmt.Errorf("Oracle counter %q is missing", name)
		}
		delta := after - before
		if delta < 0 {
			return 0, fmt.Errorf("Oracle counter %q reset during sample", name)
		}
		return float64(delta) / dt.Seconds(), nil
	}
	names := []string{
		"user commits",
		"user rollbacks",
		"execute count",
		"parse count (total)",
		"parse count (hard)",
		"physical reads",
		"session logical reads",
		"redo size",
	}
	rates := make(map[string]float64, len(names))
	for _, name := range names {
		value, err := rate(name)
		if err != nil {
			state.Data.Health = &Health{Section: reset(err.Error())}
			return
		}
		rates[name] = value
	}
	commits := rates["user commits"]
	rollbacks := rates["user rollbacks"]
	executions := rates["execute count"]
	parses := rates["parse count (total)"]
	hardParses := rates["parse count (hard)"]
	physicalReads := rates["physical reads"]
	logicalReads := rates["session logical reads"]
	redoBytes := rates["redo size"]
	hardParseRatio := float64(0)
	if parses > 0 {
		hardParseRatio = hardParses / parses
	}
	state.Data.Health = &Health{
		Section:            sampled(),
		TransactionsPerSec: round2(commits + rollbacks),
		CommitsPerSec:      round2(commits),
		RollbacksPerSec:    round2(rollbacks),
		ExecutionsPerSec:   round2(executions),
		ParsesPerSec:       round2(parses),
		HardParsesPerSec:   round2(hardParses),
		HardParseRatio:     round2(hardParseRatio),
		PhysicalReadsPerS:  round2(physicalReads),
		LogicalReadsPerS:   round2(logicalReads),
		RedoBytesPerSec:    round2(redoBytes),
	}
}
