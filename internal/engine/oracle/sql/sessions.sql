SELECT COUNT(*) AS total_sessions,
       SUM(CASE WHEN status = 'ACTIVE' THEN 1 ELSE 0 END) AS active_sessions,
       SUM(CASE WHEN status = 'INACTIVE' THEN 1 ELSE 0 END) AS inactive_sessions,
       SUM(CASE WHEN blocking_session IS NOT NULL THEN 1 ELSE 0 END) AS blocked_sessions,
       SUM(CASE WHEN status = 'ACTIVE' AND last_call_et >= 300 THEN 1 ELSE 0 END) AS long_running_sessions
FROM gv$session
WHERE type <> 'BACKGROUND'
  AND audsid <> TO_NUMBER(SYS_CONTEXT('USERENV', 'SESSIONID'))
