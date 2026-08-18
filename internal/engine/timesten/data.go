package timesten

import (
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

// Data is the versioned TimesTen payload stored under engine_data.
type Data struct {
	Health        *Health        `json:"health"`
	Connections   *Connections   `json:"connections"`
	Locks         *Locks         `json:"locks"`
	TopSQL        *TopSQL        `json:"top_sql"`
	Space         *Space         `json:"space"`
	Persistence   *Persistence   `json:"persistence"`
	Tables        *Tables        `json:"tables"`
	Indexes       *Indexes       `json:"indexes"`
	Configuration *Configuration `json:"configuration"`
	Replication   *Replication   `json:"replication"`
}

type Health struct {
	engine.Section
	TransactionsPerSec float64 `json:"transactions_per_sec"`
	CommitsPerSec      float64 `json:"commits_per_sec"`
	RollbacksPerSec    float64 `json:"rollbacks_per_sec"`
	RollbackRatio      float64 `json:"rollback_ratio"`
	SelectsPerSec      float64 `json:"selects_per_sec"`
	WritesPerSec       float64 `json:"writes_per_sec"`
	LogBytesPerSec     float64 `json:"log_bytes_per_sec"`
	LogBufferWaitsPerS float64 `json:"log_buffer_waits_per_sec"`
	CheckpointsPerSec  float64 `json:"checkpoints_per_sec"`
}

type Connections struct {
	engine.Section
	Current                 int64   `json:"current"`
	Established             int64   `json:"established"`
	Disconnected            int64   `json:"disconnected"`
	ClientServerEstablished int64   `json:"client_server_established"`
	EstablishedPerSec       float64 `json:"established_per_sec"`
}

type Locks struct {
	engine.Section
	Deadlocks          int64   `json:"deadlocks"`
	Timeouts           int64   `json:"timeouts"`
	WaitGrants         int64   `json:"wait_grants"`
	DeadlocksPerSec    float64 `json:"deadlocks_per_sec"`
	TimeoutsPerSec     float64 `json:"timeouts_per_sec"`
	WaitGrantsPerSec   float64 `json:"wait_grants_per_sec"`
	ActiveRowsExcluded bool    `json:"active_rows_excluded"`
}

type TopSQL struct {
	engine.Section
	Statements []SQLStatement `json:"statements"`
}

type SQLStatement struct {
	CommandID       int64     `json:"command_id"`
	SQLHash         string    `json:"sql_hash"`
	Owner           string    `json:"owner"`
	Executions      int64     `json:"executions"`
	MinSeconds      float64   `json:"min_seconds"`
	MaxSeconds      float64   `json:"max_seconds"`
	LastSeconds     float64   `json:"last_seconds"`
	SampleText      string    `json:"sample_text"`
	LastCollectedAt time.Time `json:"last_collected_at"`
}

type Space struct {
	engine.Section
	Permanent SpacePool `json:"permanent"`
	Temporary SpacePool `json:"temporary"`
}

type SpacePool struct {
	AllocatedBytes int64   `json:"allocated_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	HighWaterBytes int64   `json:"high_water_bytes"`
	UsedRatio      float64 `json:"used_ratio"`
	HighWaterRatio float64 `json:"high_water_ratio"`
}

type Persistence struct {
	engine.Section
	RequiredRecovery bool  `json:"required_recovery"`
	FirstLogFile     int64 `json:"first_log_file"`
	LastLogFile      int64 `json:"last_log_file"`
	ReplicationHold  int64 `json:"replication_hold_log_file"`
}

type Tables struct {
	engine.Section
	Rows      []Table `json:"rows"`
	Truncated bool    `json:"truncated"`
}

type Table struct {
	Owner             string `json:"owner"`
	Name              string `json:"name"`
	EstimatedRows     int64  `json:"estimated_rows"`
	ColumnCount       int    `json:"column_count"`
	MaximumRowBytes   int64  `json:"maximum_row_bytes"`
	LastStatsUpdate   string `json:"last_stats_update,omitempty"`
	MissingStatistics bool   `json:"missing_statistics"`
}

type Indexes struct {
	engine.Section
	Rows      []Index `json:"rows"`
	Truncated bool    `json:"truncated"`
}

type Index struct {
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	TableOwner  string `json:"table_owner"`
	TableName   string `json:"table_name"`
	TypeCode    int    `json:"type_code"`
	Unique      bool   `json:"unique"`
	Primary     bool   `json:"primary"`
	ColumnCount int    `json:"column_count"`
	HashPages   int64  `json:"hash_pages"`
}

type Configuration struct {
	engine.Section
}

type Replication struct {
	engine.Section
	Definitions []ReplicationDefinition `json:"definitions"`
	Peers       []ReplicationPeer       `json:"peers"`
}

type ReplicationDefinition struct {
	Owner   string `json:"owner"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

type ReplicationPeer struct {
	Owner        string   `json:"owner"`
	Name         string   `json:"name"`
	SubscriberID int64    `json:"subscriber_id"`
	TrackID      int      `json:"track_id"`
	State        *int64   `json:"state,omitempty"`
	Latency      *float64 `json:"latency_seconds,omitempty"`
	LastSent     *int64   `json:"last_sent_epoch,omitempty"`
	LastReceived *int64   `json:"last_received_epoch,omitempty"`
}
