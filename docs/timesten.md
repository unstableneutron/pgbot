# Oracle TimesTen support

`ttbot` inspects Oracle TimesTen 22.1 Classic through a client/server ODBC
connection. It is separate from `pgbot` and `orabot`. The default build does
not compile or link the TimesTen driver.

## Scope

The first release supports TimesTen 22.1 Classic client/server mode. It rejects
connection strings that do not include both `TTC_SERVER` and
`TTC_SERVER_DSN`. Direct mode and TimesTen Scaleout are outside this scope.

`ttbot` sends one statically checked `SELECT` statement at a time. It rejects
statement separators, SQL comments, DML, DDL, PL/SQL, and procedure calls
before they reach ODBC.

| Section | SELECT source | Notes |
| --- | --- | --- |
| Health, connections, lock counters | `SYS.SYSTEMSTATS` | Rates include `ttbot`'s own SELECT activity. Active session and lock rows require `ttXactAdmin` and are excluded. |
| Permanent and temporary memory | `SYS.MONITOR` | TimesTen reports these sizes in KiB. |
| Persistence | `SYS.MONITOR` | Includes the recovery-required flag and log file range. |
| Tables and indexes | `SYS.TABLES`, `SYS.TBL_STATS`, `SYS.INDEXES` | Limited to 500 application objects per section. |
| Top SQL | `SYS.TTSTATS_SQL_COMMAND_HIST`, `SYS.TTSTATS_TOP_SQL_CMD_TEXT` | Optional. TTStats must collect snapshots. SQL literals are scrubbed before output. |
| Classic replication | `TTREP.REPLICATIONS`, `TTREP.REPPEERS` | Reports definitions and peer state values without changing replication. |
| Configuration | None | `ttConfiguration` is a built-in procedure. `ttbot` reports this section as unsupported instead of allowing procedure calls. |

## Build

Install a TimesTen 22.1 client, a C compiler, and unixODBC development headers.
Register the client driver with unixODBC. For example:

```ini
[TimesTen 22.1 Client Driver]
Description=Oracle TimesTen 22.1 client driver
Driver=/path/to/timesten/install/lib/libttclient.so
Threading=0
```

Load the TimesTen client environment so `TIMESTEN_HOME` and the runtime library
path point to the same installation. Then build the tagged command:

```sh
go build -tags timesten -o ttbot ./cmd/ttbot
```

The build uses cgo and
`github.com/alexbrainman/odbc` at
`v0.0.0-20250601004241-49e6b2bc0cf0`. The untagged `pgbot` and `orabot` builds
do not import or link this driver.

## Monitor account

A stock TimesTen 22.1 Classic database grants `PUBLIC` read access to the core
system tables used by `ttbot`. The monitor account needs only a session:

```sql
CREATE USER PGBOT_MONITOR IDENTIFIED BY "replace-with-a-secret";
GRANT CREATE SESSION TO PGBOT_MONITOR;
```

If local hardening removed the stock `PUBLIC` grants, grant only these tables:

```sql
GRANT SELECT ON SYS.MONITOR TO PGBOT_MONITOR;
GRANT SELECT ON SYS.SYSTEMSTATS TO PGBOT_MONITOR;
GRANT SELECT ON SYS.TABLES TO PGBOT_MONITOR;
GRANT SELECT ON SYS.TBL_STATS TO PGBOT_MONITOR;
GRANT SELECT ON SYS.INDEXES TO PGBOT_MONITOR;
GRANT SELECT ON TTREP.REPLICATIONS TO PGBOT_MONITOR;
GRANT SELECT ON TTREP.REPPEERS TO PGBOT_MONITOR;
```

For top SQL, enable TTStats collection through the approved database
administration process and add these two grants:

```sql
GRANT SELECT ON SYS.TTSTATS_SQL_COMMAND_HIST TO PGBOT_MONITOR;
GRANT SELECT ON SYS.TTSTATS_TOP_SQL_CMD_TEXT TO PGBOT_MONITOR;
```

Do not grant `ADMIN`, `SELECT ANY TABLE`, DML, or DDL privileges. The live
integration test verifies that the account cannot run `INSERT`, `UPDATE`, or
`CREATE TABLE` statements.

## Run

Use an environment variable so the password does not enter shell history:

```sh
export TIMESTEN_CONNECTION_STRING='DRIVER={TimesTen 22.1 Client Driver};TTC_SERVER=db.example/6625;TTC_SERVER_DSN=sampledb;UID=PGBOT_MONITOR;PWD=replace-with-a-secret'

./ttbot inspect --full
./ttbot inspect --json --fail-on=critical
```

When they are absent, `ttbot` adds `SQLQueryTimeout=5` and `TTC_TIMEOUT=10`.
User-supplied values must be from 1 to 30 seconds. This hard server timeout is
important because ODBC cancellation is not instant. In the TimesTen XE
22.1.1.11.0 amd64 test container, a 300 ms Go context timeout returned after
approximately three to four seconds. The connection pool remained usable after
each cancellation.

Passwords in `PWD` and `PASSWORD` attributes are removed from command errors.
Avoid passing the connection string as a command argument because other local
users can sometimes read process arguments.

## Verification

Normal builds and tests stay native-free:

```sh
go test ./...
```

Run tagged tests on a host with the TimesTen client and unixODBC:

```sh
go test -tags timesten ./cmd/ttbot ./internal/engine/timesten/...
```

Live tests use `TIMESTEN_TEST_DSN`. The optional
`TIMESTEN_TEST_WRITE_TABLE` and `TIMESTEN_TEST_CANCEL_QUERY` values enable the
write-denial and cancellation checks used by the TimesTen XE test environment.

