SELECT RTRIM(NAME), VALUE
FROM SYS.SYSTEMSTATS
WHERE NAME IN (
  'connections.disconnected',
  'connections.established.client_server',
  'connections.established.count',
  'lock.deadlocks',
  'lock.locks_granted.wait',
  'lock.timeouts',
  'log.buffer.bytes_inserted',
  'log.buffer.waits',
  'stmt.executes.alters',
  'stmt.executes.creates',
  'stmt.executes.deletes',
  'stmt.executes.drops',
  'stmt.executes.inserts',
  'stmt.executes.merges',
  'stmt.executes.selects',
  'stmt.executes.updates',
  'txn.commits.count',
  'txn.rollbacks',
  'ckpt.completed'
)

