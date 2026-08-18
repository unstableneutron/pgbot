SELECT name, display_value, isdefault, ismodified
FROM v$parameter
WHERE name IN (
  'processes',
  'sessions',
  'open_cursors',
  'pga_aggregate_target',
  'sga_target',
  'sga_max_size',
  'db_cache_size',
  'shared_pool_size',
  'undo_retention',
  'fast_start_mttr_target',
  'archive_lag_target',
  'statistics_level',
  'cursor_sharing',
  'optimizer_mode',
  'parallel_degree_policy'
)
ORDER BY name
