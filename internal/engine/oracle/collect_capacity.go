package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

type storageCollector struct{ target *Target }

func (*storageCollector) Name() string            { return "storage" }
func (*storageCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *storageCollector) Sample(ctx context.Context) (any, error) {
	var out []Tablespace
	err := c.target.query(ctx, storageSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var row Tablespace
			if err := rows.Scan(&row.Name, &row.TotalBytes, &row.MaxBytes, &row.UsedBytes); err != nil {
				return err
			}
			if row.TotalBytes > 0 {
				row.UsedRatio = round2(float64(row.UsedBytes) / float64(row.TotalBytes))
			}
			out = append(out, row)
		}
		return nil
	})
	return out, err
}

func (*storageCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.Storage = &Storage{Section: unavailable(pair.Err)}
		return
	}
	rows, ok := pair.A.([]Tablespace)
	if !ok {
		state.Data.Storage = &Storage{Section: unavailable(typeError("storage", pair.A))}
		return
	}
	if rows == nil {
		rows = []Tablespace{}
	}
	state.Data.Storage = &Storage{Section: scraped(), Tablespaces: rows}
}

type memorySample struct {
	SGABytes int64
	PGABytes int64
}

type memoryCollector struct{ target *Target }

func (*memoryCollector) Name() string            { return "memory" }
func (*memoryCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *memoryCollector) Sample(ctx context.Context) (any, error) {
	var out memorySample
	err := c.target.query(ctx, memorySQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var area string
			var bytes int64
			if err := rows.Scan(&area, &bytes); err != nil {
				return err
			}
			switch strings.ToUpper(area) {
			case "SGA":
				out.SGABytes = bytes
			case "PGA":
				out.PGABytes = bytes
			}
		}
		return nil
	})
	return out, err
}

func (*memoryCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.Memory = &Memory{Section: unavailable(pair.Err)}
		return
	}
	sample, ok := pair.A.(memorySample)
	if !ok {
		state.Data.Memory = &Memory{Section: unavailable(typeError("memory", pair.A))}
		return
	}
	state.Data.Memory = &Memory{Section: scraped(), SGABytes: sample.SGABytes, PGABytes: sample.PGABytes}
}

type resourcesCollector struct{ target *Target }

func (*resourcesCollector) Name() string            { return "resources" }
func (*resourcesCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *resourcesCollector) Sample(ctx context.Context) (any, error) {
	var out []ResourceLimit
	err := c.target.query(ctx, resourcesSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var row ResourceLimit
			var limit string
			if err := rows.Scan(&row.Name, &row.CurrentUtilization, &row.MaxUtilization, &limit); err != nil {
				return err
			}
			limit = strings.TrimSpace(limit)
			if strings.EqualFold(limit, "UNLIMITED") {
				row.Unlimited = true
			} else {
				value, err := strconv.ParseInt(limit, 10, 64)
				if err != nil {
					return fmt.Errorf("parse Oracle %s resource limit %q: %w", row.Name, limit, err)
				}
				row.Limit = value
			}
			out = append(out, row)
		}
		return nil
	})
	return out, err
}

func (*resourcesCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.Resources = &Resources{Section: unavailable(pair.Err)}
		return
	}
	rows, ok := pair.A.([]ResourceLimit)
	if !ok {
		state.Data.Resources = &Resources{Section: unavailable(typeError("resources", pair.A))}
		return
	}
	if rows == nil {
		rows = []ResourceLimit{}
	}
	state.Data.Resources = &Resources{Section: scraped(), Limits: rows}
}
