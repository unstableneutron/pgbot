SELECT name, value
FROM v$sysstat
WHERE name IN (
  'user commits',
  'user rollbacks',
  'execute count',
  'parse count (total)',
  'parse count (hard)',
  'physical reads',
  'session logical reads',
  'redo size'
)
