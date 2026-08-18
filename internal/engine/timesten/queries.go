package timesten

import "embed"

//go:embed sql/*.sql
var queryFiles embed.FS

//go:embed sql/health.sql
var healthSQL string

//go:embed sql/space.sql
var spaceSQL string

//go:embed sql/tables.sql
var tablesSQL string

//go:embed sql/indexes.sql
var indexesSQL string

//go:embed sql/top_sql.sql
var topSQLSQL string

//go:embed sql/replication_definitions.sql
var replicationDefinitionsSQL string

//go:embed sql/replication_peers.sql
var replicationPeersSQL string
