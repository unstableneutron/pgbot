package timesten

import (
	"fmt"
	"math"
	"strings"

	"github.com/pgrundev/pgbot/internal/engine"
	"github.com/pgrundev/pgbot/internal/model"
)

func unavailable(err error) engine.Section {
	reason := "collector unavailable"
	if err != nil {
		reason = strings.TrimSpace(RedactDSN(err.Error()))
	}
	return engine.Section{Exactness: model.ExactnessUnavailable, Reason: reason}
}

func unsupported(reason string) engine.Section {
	return engine.Section{Exactness: model.ExactnessUnavailable, Reason: reason}
}

func scraped() engine.Section {
	return engine.Section{Exactness: model.ExactnessScraped}
}

func sampled(reason string) engine.Section {
	return engine.Section{Exactness: model.ExactnessSampled, Reason: reason}
}

func cumulative(reason string) engine.Section {
	return engine.Section{Exactness: model.ExactnessCumulative, Reason: reason}
}

func reset(reason string) engine.Section {
	return engine.Section{Exactness: model.ExactnessReset, Reason: reason}
}

func typeError(name string, value any) error {
	return fmt.Errorf("%s collector returned %T", name, value)
}

func round2(value float64) float64 { return math.Round(value*100) / 100 }

func ratio(used, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(used) / float64(total)
}
