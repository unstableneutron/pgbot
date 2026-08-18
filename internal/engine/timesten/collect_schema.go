package timesten

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

type tablesSample struct {
	Rows      []Table
	Truncated bool
}

type tablesCollector struct{ target *Target }

func (*tablesCollector) Name() string            { return "tables" }
func (*tablesCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *tablesCollector) Sample(ctx context.Context) (any, error) {
	value := tablesSample{Rows: []Table{}}
	err := c.target.query(ctx, tablesSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var table Table
			var lastStats sql.NullString
			if err := rows.Scan(&table.Owner, &table.Name, &table.EstimatedRows, &table.ColumnCount, &table.MaximumRowBytes, &lastStats); err != nil {
				return err
			}
			table.LastStatsUpdate = strings.TrimSpace(lastStats.String)
			table.MissingStatistics = table.EstimatedRows > 0 && table.LastStatsUpdate == ""
			if len(value.Rows) < 500 {
				value.Rows = append(value.Rows, table)
			} else {
				value.Truncated = true
			}
		}
		return nil
	})
	return value, err
}

func (*tablesCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.Tables = &Tables{Section: unavailable(pair.Err), Rows: []Table{}}
		return
	}
	value, ok := pair.A.(tablesSample)
	if !ok {
		state.Data.Tables = &Tables{Section: unavailable(typeError("tables", pair.A)), Rows: []Table{}}
		return
	}
	state.Data.Tables = &Tables{Section: scraped(), Rows: value.Rows, Truncated: value.Truncated}
}

type indexesSample struct {
	Rows      []Index
	Truncated bool
}

type indexesCollector struct{ target *Target }

func (*indexesCollector) Name() string            { return "indexes" }
func (*indexesCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *indexesCollector) Sample(ctx context.Context) (any, error) {
	value := indexesSample{Rows: []Index{}}
	err := c.target.query(ctx, indexesSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var index Index
			var unique, primary []byte
			if err := rows.Scan(
				&index.Owner, &index.Name, &index.TableOwner, &index.TableName,
				&index.TypeCode, &unique, &primary, &index.ColumnCount, &index.HashPages,
			); err != nil {
				return err
			}
			index.Unique = binaryFlag(unique)
			index.Primary = binaryFlag(primary)
			if len(value.Rows) < 500 {
				value.Rows = append(value.Rows, index)
			} else {
				value.Truncated = true
			}
		}
		return nil
	})
	return value, err
}

func (*indexesCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.Indexes = &Indexes{Section: unavailable(pair.Err), Rows: []Index{}}
		return
	}
	value, ok := pair.A.(indexesSample)
	if !ok {
		state.Data.Indexes = &Indexes{Section: unavailable(typeError("indexes", pair.A)), Rows: []Index{}}
		return
	}
	state.Data.Indexes = &Indexes{Section: scraped(), Rows: value.Rows, Truncated: value.Truncated}
}

func binaryFlag(value []byte) bool {
	for _, item := range value {
		if item != 0 {
			return true
		}
	}
	return false
}
