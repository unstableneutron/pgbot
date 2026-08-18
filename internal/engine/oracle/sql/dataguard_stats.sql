SELECT name, value, unit
FROM v$dataguard_stats
WHERE name IN ('transport lag', 'apply lag')
ORDER BY name
