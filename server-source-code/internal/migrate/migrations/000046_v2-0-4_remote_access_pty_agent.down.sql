DROP INDEX IF EXISTS idx_remote_access_sessions_linux_username_started;
DROP INDEX IF EXISTS idx_remote_access_sessions_connection_mode_started;

ALTER TABLE remote_access_sessions
    DROP COLUMN IF EXISTS recording_deleted_at,
    DROP COLUMN IF EXISTS event_count,
    DROP COLUMN IF EXISTS client_type,
    DROP COLUMN IF EXISTS linux_username;
