package timesten

import (
	"context"
	"database/sql"
	"encoding/hex"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

type topSQLCollector struct{ target *Target }

func (*topSQLCollector) Name() string            { return "top_sql" }
func (*topSQLCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *topSQLCollector) Sample(ctx context.Context) (any, error) {
	statements := []SQLStatement{}
	err := c.target.query(ctx, topSQLSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var statement SQLStatement
			var hash []byte
			if err := rows.Scan(
				&statement.CommandID, &hash, &statement.Owner, &statement.Executions,
				&statement.MinSeconds, &statement.MaxSeconds, &statement.LastSeconds,
				&statement.SampleText, &statement.LastCollectedAt,
			); err != nil {
				return err
			}
			statement.SQLHash = hex.EncodeToString(hash)
			statement.SampleText = ScrubQueryText(statement.SampleText)
			statement.LastCollectedAt = statement.LastCollectedAt.UTC()
			statements = append(statements, statement)
		}
		return nil
	})
	return statements, err
}

func (*topSQLCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.TopSQL = &TopSQL{Section: unavailable(pair.Err), Statements: []SQLStatement{}}
		return
	}
	statements, ok := pair.A.([]SQLStatement)
	if !ok {
		state.Data.TopSQL = &TopSQL{Section: unavailable(typeError("top SQL", pair.A)), Statements: []SQLStatement{}}
		return
	}
	if len(statements) == 0 {
		state.Data.TopSQL = &TopSQL{
			Section:    unsupported("TTStats returned no top SQL samples; enable TTStats collection to populate this section"),
			Statements: []SQLStatement{},
		}
		return
	}
	state.Data.TopSQL = &TopSQL{Section: cumulative("TTStats history is cumulative between TTStats snapshots"), Statements: statements}
}
