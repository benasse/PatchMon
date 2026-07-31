ALTER TABLE remote_access_sessions
    ADD COLUMN IF NOT EXISTS linux_username TEXT,
    ADD COLUMN IF NOT EXISTS client_type TEXT,
    ADD COLUMN IF NOT EXISTS event_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS recording_deleted_at TIMESTAMP(3);

CREATE INDEX IF NOT EXISTS idx_remote_access_sessions_connection_mode_started
    ON remote_access_sessions (connection_mode, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_remote_access_sessions_linux_username_started
    ON remote_access_sessions (linux_username, started_at DESC);
