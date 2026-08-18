# Oracle Database support

`orabot` is a separate read-only command. It does not change `pgbot` or its
PostgreSQL output.

The first supported target is Oracle Database 19c or later with one database
instance. Connect directly to one CDB root or PDB service. Oracle RAC is
detected and rejected because the first collector does not yet make every
gauge and counter RAC-safe.

## Build and run

```sh
go build -o orabot ./cmd/orabot
export ORACLE_DATABASE_URL='oracle://orabot_monitor:password@db.example:1521/APP'
./orabot inspect --full
./orabot inspect --format=json
```

The URL uses the `go-ora` URL format. Use the environment variable so that a
password does not occur in shell history or the process argument list.

`orabot` starts each collector transaction with `SET TRANSACTION READ ONLY`.
It also checks every built-in statement before execution. The check rejects
writes, multiple statements, comments, and Oracle Diagnostics Pack or Tuning
Pack objects. SQL text from the shared cursor is scrubbed before it enters a
report.

## Monitoring user

Create a dedicated local user in each target PDB. Run these grants as a user
that can grant access to the `SYS` views. Adapt password and profile rules to
the site's policy.

```sql
CREATE USER orabot_monitor IDENTIFIED BY "replace-this-password";
GRANT CREATE SESSION TO orabot_monitor;

GRANT SELECT ON SYS.V_$DATABASE TO orabot_monitor;
GRANT SELECT ON SYS.V_$INSTANCE TO orabot_monitor;
GRANT SELECT ON SYS.GV_$INSTANCE TO orabot_monitor;
GRANT SELECT ON SYS.V_$SYSSTAT TO orabot_monitor;
GRANT SELECT ON SYS.GV_$SESSION TO orabot_monitor;
GRANT SELECT ON SYS.GV_$SQLSTATS TO orabot_monitor;
GRANT SELECT ON SYS.V_$SGA TO orabot_monitor;
GRANT SELECT ON SYS.V_$PGASTAT TO orabot_monitor;
GRANT SELECT ON SYS.V_$RESOURCE_LIMIT TO orabot_monitor;
GRANT SELECT ON SYS.V_$PARAMETER TO orabot_monitor;
GRANT SELECT ON SYS.V_$ARCHIVE_DEST TO orabot_monitor;
GRANT SELECT ON SYS.V_$DATAGUARD_STATS TO orabot_monitor;

GRANT SELECT ON SYS.DBA_DATA_FILES TO orabot_monitor;
GRANT SELECT ON SYS.DBA_FREE_SPACE TO orabot_monitor;
GRANT SELECT ON SYS.DBA_TAB_STATISTICS TO orabot_monitor;
GRANT SELECT ON SYS.DBA_INDEXES TO orabot_monitor;
```

Do not grant `DBA`, `SELECT ANY TABLE`, `INSERT`, `UPDATE`, `DELETE`, or DDL
privileges. The four `DBA_*` grants expose metadata that the current storage and
schema collectors need. They do not grant access to application table rows.

If an optional view is not available, its collector section is marked
`unavailable` with a redacted reason. Other sections continue.

## Current scope

The inspect workflow includes:

- sampled health counters;
- sessions and blocking evidence;
- current top SQL with scrubbed literals;
- permanent tablespaces, SGA, PGA, and process or session limits;
- tables, optimizer statistics, and indexes;
- selected configuration parameters;
- archive destinations and Data Guard lag on a standby.

The storage check does not yet include temporary tablespaces, ASM or filesystem
free space, or recovery-area use. Schema and index output is limited to the
first 500 rows and records when the result is truncated. Live Oracle integration
tests remain required before this support is released as stable.
