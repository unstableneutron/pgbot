package engine

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type testReport struct {
	pairs map[string]SamplePair
	dt    time.Duration
}

type testCollector struct {
	name   string
	kind   SampleKind
	calls  atomic.Int32
	sample func(int32) (any, error)
}

func (c *testCollector) Name() string     { return c.name }
func (c *testCollector) Kind() SampleKind { return c.kind }
func (c *testCollector) Sample(context.Context) (any, error) {
	call := c.calls.Add(1)
	return c.sample(call)
}
func (c *testCollector) Assemble(report *testReport, pair SamplePair, dt time.Duration) {
	if report.pairs == nil {
		report.pairs = make(map[string]SamplePair)
	}
	report.pairs[c.name] = pair
	report.dt = dt
}

func TestRunSamplesGaugesOnceAndCountersTwice(t *testing.T) {
	gauge := &testCollector{name: "gauge", kind: Gauge, sample: func(call int32) (any, error) { return call, nil }}
	counter := &testCollector{name: "counter", kind: Counter, sample: func(call int32) (any, error) { return call * 10, nil }}

	report, err := Run(context.Background(), RunOptions{Interval: time.Millisecond}, []Collector[testReport]{gauge, counter}, func(time.Time, time.Duration) *testReport {
		return &testReport{}
	})
	if err != nil {
		t.Fatal(err)
	}
	if gauge.calls.Load() != 1 {
		t.Fatalf("gauge calls = %d, want 1", gauge.calls.Load())
	}
	if counter.calls.Load() != 2 {
		t.Fatalf("counter calls = %d, want 2", counter.calls.Load())
	}
	if got := report.pairs["counter"]; got.A != int32(10) || got.B != int32(20) || got.Err != nil {
		t.Fatalf("counter pair = %#v", got)
	}
	if report.dt <= 0 {
		t.Fatalf("sample duration = %v, want positive", report.dt)
	}
}

func TestRunKeepsCollectorErrorLocal(t *testing.T) {
	want := errors.New("view unavailable")
	collector := &testCollector{name: "sessions", kind: Gauge, sample: func(int32) (any, error) { return nil, want }}
	report, err := Run(context.Background(), RunOptions{}, []Collector[testReport]{collector}, func(time.Time, time.Duration) *testReport {
		return &testReport{}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(report.pairs["sessions"].Err, want) {
		t.Fatalf("collector error = %v, want %v", report.pairs["sessions"].Err, want)
	}
}

func TestRunRejectsDuplicateCollectorNames(t *testing.T) {
	first := &testCollector{name: "health", sample: func(int32) (any, error) { return nil, nil }}
	second := &testCollector{name: "health", sample: func(int32) (any, error) { return nil, nil }}
	_, err := Run(context.Background(), RunOptions{}, []Collector[testReport]{first, second}, func(time.Time, time.Duration) *testReport {
		return &testReport{}
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate collector") {
		t.Fatalf("error = %v, want duplicate collector error", err)
	}
}

func TestRunStopsDuringCounterInterval(t *testing.T) {
	counter := &testCollector{name: "health", kind: Counter, sample: func(int32) (any, error) { return nil, nil }}
	_, err := Run(context.Background(), RunOptions{Interval: time.Second, Deadline: 10 * time.Millisecond}, []Collector[testReport]{counter}, func(time.Time, time.Duration) *testReport {
		return &testReport{}
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}
