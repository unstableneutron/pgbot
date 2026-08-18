SELECT inst_id,
       sid,
       serial#,
       NVL(username, '(unknown)'),
       status,
       NVL(wait_class, 'Unknown'),
       NVL(event, 'Unknown'),
       seconds_in_wait,
       blocking_instance,
       blocking_session,
       final_blocking_instance,
       final_blocking_session
FROM gv$session
WHERE blocking_session IS NOT NULL
ORDER BY seconds_in_wait DESC
FETCH FIRST 20 ROWS ONLY
