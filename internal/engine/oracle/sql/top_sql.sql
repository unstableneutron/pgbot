SELECT inst_id,
       sql_id,
       plan_hash_value,
       executions,
       elapsed_time / 1000000,
       cpu_time / 1000000,
       buffer_gets,
       disk_reads,
       rows_processed,
       SUBSTR(sql_text, 1, 1000)
FROM gv$sqlstats
WHERE sql_id IS NOT NULL
ORDER BY elapsed_time DESC
FETCH FIRST 20 ROWS ONLY
