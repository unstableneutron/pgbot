SELECT owner,
       table_name,
       NVL(num_rows, 0),
       last_analyzed,
       NVL(stale_stats, 'NO')
FROM dba_tab_statistics s
JOIN dba_users u ON u.username = s.owner
WHERE s.object_type = 'TABLE'
  AND u.oracle_maintained = 'N'
ORDER BY s.owner, s.table_name
FETCH FIRST 501 ROWS ONLY
