SELECT resource_name,
       current_utilization,
       max_utilization,
       limit_value
FROM v$resource_limit
WHERE resource_name IN ('processes', 'sessions')
ORDER BY resource_name
