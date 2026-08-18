package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SampleKind controls whether a collector reads once or computes a rate from
// two samples.
type SampleKind uint8

const (
	Gauge SampleKind = iota
	Counter
)

// SamplePair contains the values and first error from one collector run.
type SamplePair struct {
	A   any
	B   any
	Err error
}

// Collector is the consumer-owned seam between the shared scheduler and one
// engine's diagnostic code. Connectors and query APIs stay engine-specific.
type Collector[R any] interface {
	Name() string
	Kind() SampleKind
	Sample(context.Context) (any, error)
	Assemble(*R, SamplePair, time.Duration)
}

// RunOptions controls only shared scheduling behavior. Query timeouts and pool
// limits belong to each engine connector.
type RunOptions struct {
	Interval    time.Duration
	Deadline    time.Duration
	Concurrency int
}

func (o RunOptions) normalized() RunOptions {
	if o.Interval <= 0 {
		o.Interval = time.Second
	}
	if o.Deadline <= 0 {
		o.Deadline = 20*time.Second + o.Interval
	}
	if o.Concurrency <= 0 {
		o.Concurrency = 4
	}
	return o
}

// Run samples collectors and assembles one engine-owned report. Collector
// failures stay local to their section; only invalid setup or the run context
// failing aborts the whole inspection.
func Run[R any](
	ctx context.Context,
	opts RunOptions,
	collectors []Collector[R],
	newReport func(collectedAt time.Time, sample time.Duration) *R,
) (*R, error) {
	opts = opts.normalized()
	if newReport == nil {
		return nil, fmt.Errorf("engine runner: nil report factory")
	}
	if err := validateCollectors(collectors); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Deadline)
	defer cancel()

	results := make(map[string]*SamplePair, len(collectors))
	var mu sync.Mutex
	runPhase(ctx, collectors, opts.Concurrency, func(c Collector[R]) bool { return true }, func(c Collector[R], value any, err error) {
		mu.Lock()
		defer mu.Unlock()
		results[c.Name()] = &SamplePair{A: value, Err: err}
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tA := time.Now().UTC()
	dt := time.Duration(0)
	if hasCounters(collectors) {
		timer := time.NewTimer(opts.Interval)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}

		tB := time.Now().UTC()
		dt = tB.Sub(tA)
		runPhase(ctx, collectors, opts.Concurrency, func(c Collector[R]) bool { return c.Kind() == Counter }, func(c Collector[R], value any, err error) {
			mu.Lock()
			defer mu.Unlock()
			pair := results[c.Name()]
			if pair == nil {
				pair = &SamplePair{}
				results[c.Name()] = pair
			}
			pair.B = value
			if pair.Err == nil {
				pair.Err = err
			}
		})
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	collectedAt := time.Now().UTC()
	report := newReport(collectedAt, dt)
	if report == nil {
		return nil, fmt.Errorf("engine runner: report factory returned nil")
	}
	for _, collector := range collectors {
		pair := results[collector.Name()]
		if pair == nil {
			pair = &SamplePair{}
		}
		collector.Assemble(report, *pair, dt)
	}
	return report, nil
}

func validateCollectors[R any](collectors []Collector[R]) error {
	seen := make(map[string]struct{}, len(collectors))
	for _, collector := range collectors {
		name := collector.Name()
		if name == "" {
			return fmt.Errorf("engine runner: collector name is empty")
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("engine runner: duplicate collector %q", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func hasCounters[R any](collectors []Collector[R]) bool {
	for _, collector := range collectors {
		if collector.Kind() == Counter {
			return true
		}
	}
	return false
}

func runPhase[R any](
	ctx context.Context,
	collectors []Collector[R],
	limit int,
	include func(Collector[R]) bool,
	record func(Collector[R], any, error),
) {
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, collector := range collectors {
		if !include(collector) {
			continue
		}
		collector := collector
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				record(collector, nil, ctx.Err())
				return
			}
			value, err := collector.Sample(ctx)
			record(collector, value, err)
		}()
	}
	wg.Wait()
}
