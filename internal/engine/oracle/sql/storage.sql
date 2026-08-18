SELECT df.tablespace_name,
       df.total_bytes,
       df.max_bytes,
       df.total_bytes - NVL(fs.free_bytes, 0) AS used_bytes
FROM (
  SELECT tablespace_name,
         SUM(bytes) AS total_bytes,
         SUM(CASE WHEN maxbytes > bytes THEN maxbytes ELSE bytes END) AS max_bytes
  FROM dba_data_files
  GROUP BY tablespace_name
) df
LEFT JOIN (
  SELECT tablespace_name, SUM(bytes) AS free_bytes
  FROM dba_free_space
  GROUP BY tablespace_name
) fs ON fs.tablespace_name = df.tablespace_name
ORDER BY df.tablespace_name
