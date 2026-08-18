package timesten

import (
	"context"
	"database/sql"
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

type replicationSample struct {
	Definitions []ReplicationDefinition
	Peers       []ReplicationPeer
}

type replicationCollector struct{ target *Target }

func (*replicationCollector) Name() string            { return "replication" }
func (*replicationCollector) Kind() engine.SampleKind { return engine.Gauge }

func (c *replicationCollector) Sample(ctx context.Context) (any, error) {
	value := replicationSample{Definitions: []ReplicationDefinition{}, Peers: []ReplicationPeer{}}
	if err := c.target.query(ctx, replicationDefinitionsSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var definition ReplicationDefinition
			if err := rows.Scan(&definition.Owner, &definition.Name, &definition.Version); err != nil {
				return err
			}
			value.Definitions = append(value.Definitions, definition)
		}
		return nil
	}); err != nil {
		return value, err
	}
	err := c.target.query(ctx, replicationPeersSQL, func(rows *sql.Rows) error {
		for rows.Next() {
			var peer ReplicationPeer
			var state, sent, received sql.NullInt64
			var latency sql.NullFloat64
			if err := rows.Scan(
				&peer.Owner, &peer.Name, &peer.SubscriberID, &peer.TrackID,
				&state, &latency, &sent, &received,
			); err != nil {
				return err
			}
			if state.Valid {
				peer.State = &state.Int64
			}
			if latency.Valid {
				peer.Latency = &latency.Float64
			}
			if sent.Valid {
				peer.LastSent = &sent.Int64
			}
			if received.Valid {
				peer.LastReceived = &received.Int64
			}
			value.Peers = append(value.Peers, peer)
		}
		return nil
	})
	return value, err
}

func (*replicationCollector) Assemble(state *runState, pair engine.SamplePair, _ time.Duration) {
	if pair.Err != nil {
		state.Data.Replication = &Replication{
			Section: unavailable(pair.Err), Definitions: []ReplicationDefinition{}, Peers: []ReplicationPeer{},
		}
		return
	}
	value, ok := pair.A.(replicationSample)
	if !ok {
		state.Data.Replication = &Replication{
			Section: unavailable(typeError("replication", pair.A)), Definitions: []ReplicationDefinition{}, Peers: []ReplicationPeer{},
		}
		return
	}
	state.Data.Replication = &Replication{Section: scraped(), Definitions: value.Definitions, Peers: value.Peers}
}
