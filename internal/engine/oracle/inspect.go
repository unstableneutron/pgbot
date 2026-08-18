package oracle

import (
	"context"
	"fmt"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

// InspectOptions controls one Oracle inspection run.
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

// Inspect collects one read-only Oracle report.
func Inspect(ctx context.Context, target *Target, opts InspectOptions) (*engine.Report, error) {
	if target == nil || target.db == nil {
		return nil, fmt.Errorf("inspect Oracle database: target is not open")
	}
	collectors := []engine.Collector[runState]{
		&healthCollector{target: target},
		&sessionsCollector{target: target},
		&locksCollector{target: target},
		&topSQLCollector{target: target},
		&storageCollector{target: target},
		&memoryCollector{target: target},
		&resourcesCollector{target: target},
		&tablesCollector{target: target},
		&indexesCollector{target: target},
		&configurationCollector{target: target},
		&recoveryCollector{target: target},
	}
	state, err := engine.Run(ctx, engine.RunOptions{
		Interval:    opts.Interval,
		Deadline:    opts.Deadline,
		Concurrency: opts.Concurrency,
	}, collectors, func(collectedAt time.Time, sample time.Duration) *runState {
		return &runState{CollectedAt: collectedAt, Sample: sample}
	})
	if err != nil {
		return nil, err
	}

	caps := target.Capabilities()
	topology := "single-instance"
	if caps.InstanceCount > 1 {
		topology = "rac"
	} else if caps.CDB {
		topology = "cdb"
	}
	report := engine.NewReport(engine.Oracle, engine.Target{
		Identity:  caps.Identity(),
		Database:  caps.Database,
		Instance:  caps.Instance,
		Container: caps.Container,
		Version:   caps.Version,
		Topology:  topology,
	}, state.CollectedAt, state.Sample, &state.Data)
	return report, nil
}
