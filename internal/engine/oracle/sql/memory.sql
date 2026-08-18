SELECT 'SGA' AS area, SUM(value) AS bytes
FROM v$sga
UNION ALL
SELECT 'PGA' AS area, value AS bytes
FROM v$pgastat
WHERE name = 'total PGA allocated'
