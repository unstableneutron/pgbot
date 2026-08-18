SELECT dest_id,
       dest_name,
       status,
       target,
       NVL(error, '')
FROM v$archive_dest
WHERE status <> 'INACTIVE'
ORDER BY dest_id
