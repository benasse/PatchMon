-- name: CreateRemoteAccessSession :one
INSERT INTO remote_access_sessions (
    id,
    user_id,
    host_id,
    protocol,
    connection_mode,
    status,
    browser_ip,
    user_agent,
    proxy_session_id,
    guacd_session_id,
    recording_status,
    recording_path,
    recording_name
) VALUES (
    sqlc.arg('id')::text,
    sqlc.arg('user_id')::text,
    sqlc.arg('host_id')::text,
    sqlc.arg('protocol')::text,
    sqlc.arg('connection_mode')::text,
    'connecting',
    sqlc.narg('browser_ip')::text,
    sqlc.narg('user_agent')::text,
    sqlc.narg('proxy_session_id')::text,
    sqlc.narg('guacd_session_id')::text,
    sqlc.arg('recording_status')::text,
    sqlc.narg('recording_path')::text,
    sqlc.narg('recording_name')::text
)
RETURNING *;

-- name: GetRemoteAccessSession :one
SELECT
    ras.*,
    u.username AS user_username,
    h.friendly_name AS host_friendly_name,
    h.hostname AS host_hostname
FROM remote_access_sessions ras
JOIN users u ON u.id = ras.user_id
JOIN hosts h ON h.id = ras.host_id
WHERE ras.id = sqlc.arg('id')::text;

-- name: CountRemoteAccessSessions :one
SELECT COUNT(*)::int
FROM remote_access_sessions ras
JOIN users u ON u.id = ras.user_id
JOIN hosts h ON h.id = ras.host_id
WHERE (sqlc.arg('protocol')::text = '' OR ras.protocol = sqlc.arg('protocol')::text)
  AND (sqlc.arg('status')::text = '' OR ras.status = sqlc.arg('status')::text)
  AND (sqlc.arg('host_id')::text = '' OR ras.host_id = sqlc.arg('host_id')::text)
  AND (sqlc.arg('user_id')::text = '' OR ras.user_id = sqlc.arg('user_id')::text)
  AND (
      sqlc.arg('search')::text = ''
      OR u.username ILIKE '%' || sqlc.arg('search')::text || '%'
      OR h.friendly_name ILIKE '%' || sqlc.arg('search')::text || '%'
      OR COALESCE(h.hostname, '') ILIKE '%' || sqlc.arg('search')::text || '%'
      OR ras.connection_mode ILIKE '%' || sqlc.arg('search')::text || '%'
  );

-- name: ListRemoteAccessSessions :many
SELECT
    ras.*,
    u.username AS user_username,
    h.friendly_name AS host_friendly_name,
    h.hostname AS host_hostname
FROM remote_access_sessions ras
JOIN users u ON u.id = ras.user_id
JOIN hosts h ON h.id = ras.host_id
WHERE (sqlc.arg('protocol')::text = '' OR ras.protocol = sqlc.arg('protocol')::text)
  AND (sqlc.arg('status')::text = '' OR ras.status = sqlc.arg('status')::text)
  AND (sqlc.arg('host_id')::text = '' OR ras.host_id = sqlc.arg('host_id')::text)
  AND (sqlc.arg('user_id')::text = '' OR ras.user_id = sqlc.arg('user_id')::text)
  AND (
      sqlc.arg('search')::text = ''
      OR u.username ILIKE '%' || sqlc.arg('search')::text || '%'
      OR h.friendly_name ILIKE '%' || sqlc.arg('search')::text || '%'
      OR COALESCE(h.hostname, '') ILIKE '%' || sqlc.arg('search')::text || '%'
      OR ras.connection_mode ILIKE '%' || sqlc.arg('search')::text || '%'
  )
ORDER BY ras.started_at DESC
LIMIT sqlc.arg('row_limit')::int
OFFSET sqlc.arg('row_offset')::int;

-- name: MarkRemoteAccessSessionConnected :exec
UPDATE remote_access_sessions
SET
    status = 'connected',
    connected_at = COALESCE(connected_at, CURRENT_TIMESTAMP),
    error_message = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg('id')::text
  AND status = 'connecting';

-- name: MarkRemoteAccessSessionEnded :exec
UPDATE remote_access_sessions
SET
    status = sqlc.arg('status')::text,
    ended_at = COALESCE(ended_at, CURRENT_TIMESTAMP),
    error_message = sqlc.narg('error_message')::text,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg('id')::text
  AND ended_at IS NULL;

-- name: SetRemoteAccessSessionProxyID :exec
UPDATE remote_access_sessions
SET
    proxy_session_id = sqlc.arg('proxy_session_id')::text,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg('id')::text;

-- name: SetRemoteAccessSessionGuacdID :exec
UPDATE remote_access_sessions
SET
    guacd_session_id = sqlc.arg('guacd_session_id')::text,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg('id')::text;

-- name: SetRemoteAccessSessionRecording :exec
UPDATE remote_access_sessions
SET
    recording_status = sqlc.arg('recording_status')::text,
    recording_path = sqlc.narg('recording_path')::text,
    recording_name = sqlc.narg('recording_name')::text,
    recording_size_bytes = sqlc.narg('recording_size_bytes')::bigint,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg('id')::text;
