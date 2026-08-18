package oracle

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

type recoverySample struct {
	DatabaseRole        string
	OpenMode            string
	ProtectionMode      string
	ProtectionLevel     string
	SwitchoverStatus    string
	ArchiveDestinations []ArchiveDestination
	DataGuardStats      map[string]string
}

type recoveryCollector struct{ target *Target }

func (*recoveryCollector) Name() string            { return "recovery" }
func (*recoveryCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *recoveryCollector) Sample(ctx context.Context) (any, error) {
	out := recoverySample{DataGuardStats: make(map[string]string)}
	if err := c.target.queryRow(ctx, recoverySQL, nil, func(row *sql.Row) error {
		return row.Scan(
			&out.DatabaseRole,
			&out.OpenMode,
			&out.ProtectionMode,
			&out.ProtectionLevel,
			&out.SwitchoverStatus,
		)
	}); err != nil {
		return nil, err
	}
	if err := c.target.query(ctx, archiveDestinationsSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var destination ArchiveDestination
			var destinationError sql.NullString
			if err := rows.Scan(
				&destination.ID,
				&destination.Name,
				&destination.Status,
				&destination.Target,
				&destinationError,
			); err != nil {
				return err
			}
			destination.Error = destinationError.String
			out.ArchiveDestinations = append(out.ArchiveDestinations, destination)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := c.target.query(ctx, dataGuardStatsSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var name string
			var value, unit sql.NullString
			if err := rows.Scan(&name, &value, &unit); err != nil {
				return err
			}
			out.DataGuardStats[name] = strings.TrimSpace(strings.TrimSpace(value.String) + " " + strings.TrimSpace(unit.String))
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (*recoveryCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.Recovery = &Recovery{Section: unavailable(pair.Err)}
		return
	}
	sample, ok := pair.A.(recoverySample)
	if !ok {
		state.Data.Recovery = &Recovery{Section: unavailable(typeError("recovery", pair.A))}
		return
	}
	if sample.ArchiveDestinations == nil {
		sample.ArchiveDestinations = []ArchiveDestination{}
	}
	if sample.DataGuardStats == nil {
		sample.DataGuardStats = map[string]string{}
	}
	state.Data.Recovery = &Recovery{
		Section:             scraped(),
		DatabaseRole:        sample.DatabaseRole,
		OpenMode:            sample.OpenMode,
		ProtectionMode:      sample.ProtectionMode,
		ProtectionLevel:     sample.ProtectionLevel,
		SwitchoverStatus:    sample.SwitchoverStatus,
		ArchiveDestinations: sample.ArchiveDestinations,
		DataGuardStats:      sample.DataGuardStats,
	}
}
