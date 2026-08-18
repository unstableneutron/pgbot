SELECT owner,
       index_name,
       table_owner,
       table_name,
       status,
       visibility,
       NVL(distinct_keys, 0),
       NVL(clustering_factor, 0),
       last_analyzed
FROM dba_indexes
WHERE owner NOT IN ('SYS', 'SYSTEM', 'XDB', 'MDSYS', 'CTXSYS')
ORDER BY owner, index_name
FETCH FIRST 501 ROWS ONLY
