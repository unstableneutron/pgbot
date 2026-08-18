package oracle

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

type tablesCollector struct{ target *Target }

func (*tablesCollector) Name() string            { return "tables" }
func (*tablesCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *tablesCollector) Sample(ctx context.Context) (any, error) {
	var out []Table
	err := c.target.query(ctx, tablesSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var row Table
			var lastAnalyzed sql.NullTime
			var staleStats string
			if err := rows.Scan(&row.Owner, &row.Name, &row.Rows, &lastAnalyzed, &staleStats); err != nil {
				return err
			}
			row.LastAnalyzed = timePointer(lastAnalyzed)
			row.StaleStats = strings.EqualFold(staleStats, "YES")
			out = append(out, row)
		}
		return nil
	})
	return out, err
}

func (*tablesCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.Tables = &Tables{Section: unavailable(pair.Err)}
		return
	}
	rows, ok := pair.A.([]Table)
	if !ok {
		state.Data.Tables = &Tables{Section: unavailable(typeError("tables", pair.A))}
		return
	}
	if rows == nil {
		rows = []Table{}
	}
	state.Data.Tables = &Tables{Section: scraped(), Rows: rows}
}

type indexesCollector struct{ target *Target }

func (*indexesCollector) Name() string            { return "indexes" }
func (*indexesCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *indexesCollector) Sample(ctx context.Context) (any, error) {
	var out []Index
	err := c.target.query(ctx, indexesSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var row Index
			var lastAnalyzed sql.NullTime
			if err := rows.Scan(
				&row.Owner,
				&row.Name,
				&row.TableOwner,
				&row.TableName,
				&row.Status,
				&row.Visibility,
				&row.DistinctKeys,
				&row.ClusteringFactor,
				&lastAnalyzed,
			); err != nil {
				return err
			}
			row.LastAnalyzed = timePointer(lastAnalyzed)
			out = append(out, row)
		}
		return nil
	})
	return out, err
}

func (*indexesCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.Indexes = &Indexes{Section: unavailable(pair.Err)}
		return
	}
	rows, ok := pair.A.([]Index)
	if !ok {
		state.Data.Indexes = &Indexes{Section: unavailable(typeError("indexes", pair.A))}
		return
	}
	if rows == nil {
		rows = []Index{}
	}
	state.Data.Indexes = &Indexes{Section: scraped(), Rows: rows}
}

type configurationCollector struct{ target *Target }

func (*configurationCollector) Name() string            { return "configuration" }
func (*configurationCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *configurationCollector) Sample(ctx context.Context) (any, error) {
	out := make(map[string]Parameter)
	err := c.target.query(ctx, configurationSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var name, isDefault, modifiedBy string
			var value sql.NullString
			if err := rows.Scan(&name, &value, &isDefault, &modifiedBy); err != nil {
				return err
			}
			out[name] = Parameter{
				Value:      value.String,
				Default:    strings.EqualFold(isDefault, "TRUE"),
				ModifiedBy: modifiedBy,
			}
		}
		return nil
	})
	return out, err
}

func (*configurationCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.Configuration = &Configuration{Section: unavailable(pair.Err)}
		return
	}
	parameters, ok := pair.A.(map[string]Parameter)
	if !ok {
		state.Data.Configuration = &Configuration{Section: unavailable(typeError("configuration", pair.A))}
		return
	}
	if parameters == nil {
		parameters = map[string]Parameter{}
	}
	state.Data.Configuration = &Configuration{Section: scraped(), Parameters: parameters}
}
