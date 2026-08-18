package oracle

import (
	"context"
	"database/sql"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

type topSQLCollector struct{ target *Target }

func (*topSQLCollector) Name() string            { return "top_sql" }
func (*topSQLCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *topSQLCollector) Sample(ctx context.Context) (any, error) {
	var out []SQLStatement
	err := c.target.query(ctx, topSQLSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var row SQLStatement
			var planHash int64
			var sampleText sql.NullString
			if err := rows.Scan(
				&row.Instance,
				&row.SQLID,
				&planHash,
				&row.Executions,
				&row.ElapsedSeconds,
				&row.CPUSeconds,
				&row.BufferGets,
				&row.DiskReads,
				&row.RowsProcessed,
				&sampleText,
			); err != nil {
				return err
			}
			row.PlanHash = uint64(planHash)
			row.ElapsedSeconds = round2(row.ElapsedSeconds)
			row.CPUSeconds = round2(row.CPUSeconds)
			row.SampleText = ScrubQueryText(sampleText.String)
			out = append(out, row)
		}
		return nil
	})
	return out, err
}

func (*topSQLCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.TopSQL = &TopSQL{Section: unavailable(pair.Err)}
		return
	}
	rows, ok := pair.A.([]SQLStatement)
	if !ok {
		state.Data.TopSQL = &TopSQL{Section: unavailable(typeError("top SQL", pair.A))}
		return
	}
	if rows == nil {
		rows = []SQLStatement{}
	}
	state.Data.TopSQL = &TopSQL{Section: cumulative(), Statements: rows}
}
