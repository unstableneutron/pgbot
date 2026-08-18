package oracle

import "embed"

//go:embed sql/*.sql
var queryFiles embed.FS

//go:embed sql/health.sql
var healthSQL string

//go:embed sql/sessions.sql
var sessionsSQL string

//go:embed sql/locks.sql
var locksSQL string

//go:embed sql/top_sql.sql
var topSQLSQL string

//go:embed sql/storage.sql
var storageSQL string

//go:embed sql/memory.sql
var memorySQL string

//go:embed sql/resources.sql
var resourcesSQL string

//go:embed sql/tables.sql
var tablesSQL string

//go:embed sql/indexes.sql
var indexesSQL string

//go:embed sql/configuration.sql
var configurationSQL string

//go:embed sql/recovery.sql
var recoverySQL string

//go:embed sql/archive_destinations.sql
var archiveDestinationsSQL string

//go:embed sql/dataguard_stats.sql
var dataGuardStatsSQL string
