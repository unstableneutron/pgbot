package timesten

import (
	"context"
	"fmt"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

// InspectOptions controls one TimesTen inspection run.
type InspectOptions struct {
	Interval    time.Duration
	Deadline    time.Duration
	Concurrency int
}

type runState struct {
	Data        Data
	CollectedAt time.Time
	Sample      time.Duration
}

// Inspect collects one SELECT-only TimesTen report.
func Inspect(ctx context.Context, target *Target, opts InspectOptions) (*engine.Report, error) {
	if target == nil || target.db == nil {
		return nil, fmt.Errorf("inspect TimesTen database: target is not open")
	}
	collectors := []engine.Collector[runState]{
		&healthCollector{target: target},
		&spaceCollector{target: target},
		&topSQLCollector{target: target},
		&tablesCollector{target: target},
		&indexesCollector{target: target},
		&replicationCollector{target: target},
	}
	state, err := engine.Run(ctx, engine.RunOptions{
		Interval: opts.Interval, Deadline: opts.Deadline, Concurrency: opts.Concurrency,
	}, collectors, func(collectedAt time.Time, sample time.Duration) *runState {
		return &runState{CollectedAt: collectedAt, Sample: sample}
	})
	if err != nil {
		return nil, err
	}
	state.Data.Configuration = &Configuration{Section: unsupported(
		"configuration requires the ttConfiguration built-in procedure, which is excluded from the SELECT-only boundary",
	)}

	caps := target.Capabilities()
	return engine.NewReport(engine.TimesTen, engine.Target{
		Identity: caps.Identity(),
		Database: caps.Database,
		Instance: caps.Server,
		Version:  caps.Version,
		Topology: "classic-client-server",
	}, state.CollectedAt, state.Sample, &state.Data), nil
}
