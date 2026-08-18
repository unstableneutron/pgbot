package oracle

import (
	"time"

	"github.com/pgrundev/pgbot/internal/engine"
)

// Data is the versioned Oracle payload stored under engine_data.
type Data struct {
	Health        *Health        `json:"health"`
	Sessions      *Sessions      `json:"sessions"`
	Locks         *Locks         `json:"locks"`
	TopSQL        *TopSQL        `json:"top_sql"`
	Storage       *Storage       `json:"storage"`
	Memory        *Memory        `json:"memory"`
	Resources     *Resources     `json:"resources"`
	Tables        *Tables        `json:"tables"`
	Indexes       *Indexes       `json:"indexes"`
	Configuration *Configuration `json:"configuration"`
	Recovery      *Recovery      `json:"recovery"`
}

type Health struct {
	engine.Section
	TransactionsPerSec float64 `json:"transactions_per_sec"`
	CommitsPerSec      float64 `json:"commits_per_sec"`
	RollbacksPerSec    float64 `json:"rollbacks_per_sec"`
	ExecutionsPerSec   float64 `json:"executions_per_sec"`
	ParsesPerSec       float64 `json:"parses_per_sec"`
	HardParsesPerSec   float64 `json:"hard_parses_per_sec"`
	HardParseRatio     float64 `json:"hard_parse_ratio"`
	PhysicalReadsPerS  float64 `json:"physical_reads_per_sec"`
	LogicalReadsPerS   float64 `json:"logical_reads_per_sec"`
	RedoBytesPerSec    float64 `json:"redo_bytes_per_sec"`
}

type Sessions struct {
	engine.Section
	Total       int64 `json:"total"`
	Active      int64 `json:"active"`
	Inactive    int64 `json:"inactive"`
	Blocked     int64 `json:"blocked"`
	LongRunning int64 `json:"long_running"`
}

type Locks struct {
	engine.Section
	Blocked []BlockedSession `json:"blocked"`
}

type BlockedSession struct {
	Instance              int     `json:"instance"`
	SID                   int     `json:"sid"`
	Serial                int64   `json:"serial"`
	Username              string  `json:"username"`
	Status                string  `json:"status"`
	WaitClass             string  `json:"wait_class"`
	Event                 string  `json:"event"`
	SecondsInWait         float64 `json:"seconds_in_wait"`
	BlockingInstance      int     `json:"blocking_instance"`
	BlockingSession       int     `json:"blocking_session"`
	FinalBlockingInstance int     `json:"final_blocking_instance"`
	FinalBlockingSession  int     `json:"final_blocking_session"`
}

type TopSQL struct {
	engine.Section
	Statements []SQLStatement `json:"statements"`
}

type SQLStatement struct {
	Instance       int     `json:"instance"`
	SQLID          string  `json:"sql_id"`
	PlanHash       uint64  `json:"plan_hash"`
	Executions     int64   `json:"executions"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	CPUSeconds     float64 `json:"cpu_seconds"`
	BufferGets     int64   `json:"buffer_gets"`
	DiskReads      int64   `json:"disk_reads"`
	RowsProcessed  int64   `json:"rows_processed"`
	SampleText     string  `json:"sample_text"`
}

type Storage struct {
	engine.Section
	Tablespaces []Tablespace `json:"tablespaces"`
}

type Tablespace struct {
	Name       string  `json:"name"`
	TotalBytes int64   `json:"total_bytes"`
	MaxBytes   int64   `json:"max_bytes"`
	UsedBytes  int64   `json:"used_bytes"`
	UsedRatio  float64 `json:"used_ratio"`
}

type Memory struct {
	engine.Section
	SGABytes int64 `json:"sga_bytes"`
	PGABytes int64 `json:"pga_bytes"`
}

type Resources struct {
	engine.Section
	Limits []ResourceLimit `json:"limits"`
}

type ResourceLimit struct {
	Name               string `json:"name"`
	CurrentUtilization int64  `json:"current_utilization"`
	MaxUtilization     int64  `json:"max_utilization"`
	Limit              int64  `json:"limit"`
	Unlimited          bool   `json:"unlimited"`
}

type Tables struct {
	engine.Section
	Rows      []Table `json:"rows"`
	Truncated bool    `json:"truncated"`
}

type Table struct {
	Owner        string     `json:"owner"`
	Name         string     `json:"name"`
	Rows         int64      `json:"rows"`
	LastAnalyzed *time.Time `json:"last_analyzed,omitempty"`
	StaleStats   bool       `json:"stale_stats"`
}

type Indexes struct {
	engine.Section
	Rows      []Index `json:"rows"`
	Truncated bool    `json:"truncated"`
}

type Index struct {
	Owner            string     `json:"owner"`
	Name             string     `json:"name"`
	TableOwner       string     `json:"table_owner"`
	TableName        string     `json:"table_name"`
	Status           string     `json:"status"`
	Visibility       string     `json:"visibility"`
	DistinctKeys     int64      `json:"distinct_keys"`
	ClusteringFactor int64      `json:"clustering_factor"`
	LastAnalyzed     *time.Time `json:"last_analyzed,omitempty"`
}

type Configuration struct {
	engine.Section
	Parameters map[string]Parameter `json:"parameters"`
}

type Parameter struct {
	Value      string `json:"value"`
	Default    bool   `json:"default"`
	ModifiedBy string `json:"modified_by"`
}

type Recovery struct {
	engine.Section
	DatabaseRole        string               `json:"database_role"`
	OpenMode            string               `json:"open_mode"`
	ProtectionMode      string               `json:"protection_mode"`
	ProtectionLevel     string               `json:"protection_level"`
	SwitchoverStatus    string               `json:"switchover_status"`
	ArchiveDestinations []ArchiveDestination `json:"archive_destinations"`
	DataGuardStats      map[string]string    `json:"data_guard_stats"`
}

type ArchiveDestination struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Target string `json:"target"`
	Error  string `json:"error,omitempty"`
}
