package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
	"github.com/pgrundev/pgbot/internal/model"
)

func (t *Target) query(ctx context.Context, query string, scan func(*sql.Rows) error) error {
	if err := ValidateDefaultQuery(query); err != nil {
		return err
	}
	return t.readOnly(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		if err := scan(rows); err != nil {
			return err
		}
		return rows.Err()
	})
}

func unavailable(err error) engine.Section {
	reason := "collector unavailable"
	if err != nil {
		reason = strings.TrimSpace(RedactDSN(err.Error()))
	}
	return engine.Section{Exactness: model.ExactnessUnavailable, Reason: reason}
}

func scraped() engine.Section {
	return engine.Section{Exactness: model.ExactnessScraped}
}

func cumulative() engine.Section {
	return engine.Section{Exactness: model.ExactnessCumulative}
}

func sampled() engine.Section {
	return engine.Section{Exactness: model.ExactnessSampled}
}

func reset(reason string) engine.Section {
	return engine.Section{Exactness: model.ExactnessReset, Reason: reason}
}

func round2(value float64) float64 { return math.Round(value*100) / 100 }

func timePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

func typeError(name string, value any) error {
	return fmt.Errorf("%s collector returned %T", name, value)
}
