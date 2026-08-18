SELECT owner,
       index_name,
       table_owner,
       table_name,
       status,
       visibility,
       NVL(distinct_keys, 0),
       NVL(clustering_factor, 0),
       last_analyzed
FROM dba_indexes i
JOIN dba_users u ON u.username = i.owner
WHERE u.oracle_maintained = 'N'
ORDER BY i.owner, i.index_name
FETCH FIRST 501 ROWS ONLY
