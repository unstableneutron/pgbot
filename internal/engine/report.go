// Package engine defines the small contracts shared by non-PostgreSQL
// inspection engines. PostgreSQL keeps its existing model and collector path.
package engine

import (
	"time"

	"github.com/pgrundev/pgbot/internal/model"
)

// SchemaVersion is the machine-output contract used by engine-based reports.
// It is separate from model.SchemaVersion so pgbot's PostgreSQL JSON contract
// can remain unchanged.
const SchemaVersion = "2.0.0"

// Name identifies the database engine that produced a report.
type Name string

const (
	Oracle   Name = "oracle"
	TimesTen Name = "timesten"
)

// Target identifies one inspected database without imposing one engine's
// topology on another. Engine-specific identifiers remain in Data.
type Target struct {
	Identity  string `json:"identity"`
	Database  string `json:"database"`
	Instance  string `json:"instance,omitempty"`
	Container string `json:"container,omitempty"`
	Version   string `json:"version"`
	Topology  string `json:"topology,omitempty"`
}

// Window describes the wall-clock sample used for rate calculations.
type Window struct {
	SampleSeconds float64 `json:"sample_seconds"`
}

// Section records how an engine obtained one diagnostic section.
type Section struct {
	Exactness string `json:"exactness"`
	Reason    string `json:"reason,omitempty"`
}

// Report is the stable envelope shared by orabot and ttbot. Data must be an
// engine-owned, versioned struct. The common envelope deliberately contains no
// PostgreSQL, Oracle, or TimesTen metric fields.
type Report struct {
	SchemaVersion string          `json:"schema_version"`
	Engine        Name            `json:"engine"`
	CollectedAt   time.Time       `json:"collected_at"`
	Target        Target          `json:"target"`
	Window        Window          `json:"window"`
	Findings      []model.Finding `json:"findings"`
	Data          any             `json:"engine_data"`
}

// NewReport builds a report with deterministic non-nil finding output.
func NewReport(name Name, target Target, collectedAt time.Time, sample time.Duration, data any) *Report {
	return &Report{
		SchemaVersion: SchemaVersion,
		Engine:        name,
		CollectedAt:   collectedAt.UTC(),
		Target:        target,
		Window:        Window{SampleSeconds: sample.Seconds()},
		Findings:      []model.Finding{},
		Data:          data,
	}
}
