SELECT owner,
       table_name,
       NVL(num_rows, 0),
       last_analyzed,
       NVL(stale_stats, 'NO')
FROM dba_tab_statistics
WHERE object_type = 'TABLE'
  AND owner NOT IN ('SYS', 'SYSTEM', 'XDB', 'MDSYS', 'CTXSYS')
ORDER BY owner, table_name
FETCH FIRST 501 ROWS ONLY
