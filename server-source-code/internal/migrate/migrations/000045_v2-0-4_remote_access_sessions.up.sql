CREATE TABLE IF NOT EXISTS remote_access_sessions (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    protocol TEXT NOT NULL CHECK (protocol IN ('ssh', 'rdp')),
    connection_mode TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN (
        'connecting',
        'connected',
        'failed',
        'closed',
        'timeout',
        'agent_disconnected'
    )),
    started_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    connected_at TIMESTAMP(3),
    ended_at TIMESTAMP(3),
    error_message TEXT,
    browser_ip TEXT,
    user_agent TEXT,
    proxy_session_id TEXT,
    guacd_session_id TEXT,
    recording_status TEXT NOT NULL DEFAULT 'not_requested',
    recording_path TEXT,
    recording_name TEXT,
    recording_size_bytes BIGINT,
    created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_remote_access_sessions_started_at
    ON remote_access_sessions (started_at DESC);

CREATE INDEX IF NOT EXISTS idx_remote_access_sessions_host_started
    ON remote_access_sessions (host_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_remote_access_sessions_user_started
    ON remote_access_sessions (user_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_remote_access_sessions_protocol_status_started
    ON remote_access_sessions (protocol, status, started_at DESC);
