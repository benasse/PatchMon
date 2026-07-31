# Remote Access Recording Implementation Plan

This document tracks the implementation plan for audited and recorded remote
access sessions in PatchMon. The goal is to capture who accessed which host,
when the connection started and ended, what state the connection reached, and,
where supported, the recorded terminal or desktop stream.

## Scope

- SSH sessions, direct and agent-proxy mode.
- SSH sessions routed through guacd, to use Guacamole's built-in SSH recording.
- Existing RDP sessions routed through guacd.
- A shared audit model for SSH and RDP remote access.
- A PatchMon UI for listing sessions, inspecting status, and accessing
  recordings when available.

## Target Architecture

PatchMon should treat SSH and RDP as variants of one remote access session
model:

- The browser requests a one-time remote access ticket.
- The server creates a `remote_access_sessions` row before the protocol tunnel
  is opened.
- The server updates that row as the connection moves through `connecting`,
  `connected`, `failed`, `closed`, `timeout`, or `agent_disconnected`.
- For guacd-backed sessions, the server passes recording parameters to guacd
  and stores the expected recording path/name on the session row.
- The frontend displays the same audit list for SSH and RDP, with protocol and
  recording-specific details.

## Data Model

Add a `remote_access_sessions` table:

```sql
CREATE TABLE remote_access_sessions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL,
    host_id UUID NOT NULL,
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
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    connected_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    error_message TEXT,
    browser_ip TEXT,
    user_agent TEXT,
    proxy_session_id TEXT,
    guacd_session_id TEXT,
    recording_status TEXT NOT NULL DEFAULT 'not_requested',
    recording_path TEXT,
    recording_name TEXT,
    recording_size_bytes BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Recommended indexes:

- `(tenant_id, started_at DESC)`
- `(tenant_id, host_id, started_at DESC)`
- `(tenant_id, user_id, started_at DESC)`
- `(tenant_id, protocol, status, started_at DESC)`

## Backend Plan

1. Add SQL migrations, sqlc queries, and a `RemoteAccessSessionsStore`.
2. Record SSH ticket creation in `SshTicketHandler` as `connecting`.
3. Update SSH direct sessions in `SshTerminalWSHandler`:
   - set `connected` after `session.Shell()` succeeds;
   - set `failed` on dial/auth/PTY errors;
   - set `closed` when the WebSocket or SSH session ends.
4. Update SSH agent-proxy sessions:
   - set `connected` on `ssh_proxy_connected`;
   - set `failed` on `ssh_proxy_error`;
   - set `closed` on `ssh_proxy_closed`.
5. Update RDP sessions in `RDPHandler`:
   - create the audit row during ticket creation;
   - set `connected` after guacd handshake succeeds;
   - set `failed` on guacd/agent errors;
   - set `closed` when the tunnel closes.
6. Add API endpoints:
   - `GET /api/v1/remote-access/sessions`
   - `GET /api/v1/remote-access/sessions/{id}`
   - `GET /api/v1/remote-access/sessions/{id}/recording`
   - `GET /api/v1/remote-access/sessions/{id}/recording/timing`
7. Enforce permissions with `can_use_remote_access` for session creation and a
   stricter admin/manage-hosts permission for browsing all recordings.

## SSH via guacd Plan

The current SSH proxy cannot be recorded by guacd because the agent terminates
SSH itself and sends terminal text back to the browser. For guacd recording,
guacd must be the SSH protocol endpoint.

Implementation steps:

1. Add a raw TCP proxy mode for SSH, modeled on the existing RDP proxy:
   - server creates a local listener;
   - server sends `ssh_tcp_proxy` to the agent;
   - agent dials the target SSH host/port and relays base64 TCP data;
   - guacd connects to the server-local listener.
2. Add a `guacd` SSH ticket flow:
   - `Protocol = "ssh"`;
   - `hostname = "127.0.0.1"`;
   - `port = <server local proxy port>`;
   - `username`, `password`, or private-key parameters;
   - terminal dimensions and font settings.
3. Configure SSH recordings:
   - `typescript-path`;
   - `typescript-name`;
   - `create-typescript-path = true`;
   - optional `typescript-append = false`.
4. Store the generated recording path/name on `remote_access_sessions`.
5. Keep the existing xterm SSH path as a fallback until guacd SSH is stable.

## Frontend Plan

1. Add a Remote Access Sessions view under the existing remote access or host
   detail surface.
2. List sessions with filters:
   - protocol;
   - user;
   - host;
   - status;
   - date range;
   - recording availability.
3. Add a session detail panel showing:
   - user, host, protocol, connection mode;
   - start, connected, end, and duration;
   - status and error message;
   - browser IP and user-agent when available;
   - recording availability and file size.
4. For SSH recordings:
   - first iteration: allow downloading `typescript` and `.timing`;
   - second iteration: replay in PatchMon using xterm.js and timing data.
5. For RDP recordings:
   - first iteration: expose metadata and download/admin retrieval;
   - second iteration: evaluate Guacamole playback or encoded video generation.

## Storage and Retention

- Default recording root: `/var/lib/patchmon/recordings`.
- Tenant-scoped subdirectories: `<tenant-id>/<session-id>/`.
- Server must own the directory and only serve files through authenticated API
  endpoints.
- Add retention settings:
  - enable/disable recording;
  - SSH recording retention days;
  - RDP recording retention days;
  - max recording size per session where enforceable.
- Add a cleanup worker that marks missing/deleted files and removes expired
  recordings.

## Security Notes

- Do not expose filesystem paths directly to the frontend as downloadable URLs.
- Never allow path traversal in recording endpoints.
- Audit reads/downloads of recordings as separate events.
- Mask credentials from logs and database rows.
- Document customer consent/legal requirements for recorded operator sessions.
- Consider a visible in-session indicator once recording is enabled.

## Test Matrix

Target lab:

- PatchMon server: `192.168.1.7`
- PatchMon agent host: `192.168.1.13`

Initial connectivity check from this workspace on 2026-07-31:

- `ssh -o BatchMode=yes -o ConnectTimeout=5 192.168.1.7 'hostname; uname -a'`
  reached SSH but failed authentication for local user `bs`.
- `ssh -o BatchMode=yes -o ConnectTimeout=5 192.168.1.13 'hostname; uname -a'`
  reached SSH but failed authentication for local user `bs`.
- TCP checks:
  - `192.168.1.7:22` open.
  - `192.168.1.7:3000` open and serving the PatchMon web UI.
  - `192.168.1.7:4822` refused from this workspace. This can be acceptable
    when guacd is only reachable on the Docker/internal network, but it must be
    reachable from the PatchMon server process.
  - `192.168.1.13:22` open.
  - `192.168.1.13:3000` refused, as expected for an agent-only host.

Manual test prerequisites:

1. Provide an SSH username/key or password for both hosts, or install the
   workspace user's public key on both hosts.
2. Confirm the PatchMon server on `192.168.1.7` is reachable from the browser.
3. Confirm `guacd` is running on the server or as a sidecar.
4. Confirm the agent on `192.168.1.13` is connected to the server.
5. Enable `ssh-proxy-enabled: true` and, for RDP validation,
   `rdp-proxy-enabled: true` in the agent config where needed.

Validation scenarios:

1. SSH direct success:
   - open SSH direct from PatchMon UI;
   - verify session row transitions `connecting -> connected -> closed`;
   - verify user, host, start/end timestamps, and duration.
2. SSH direct failed auth:
   - attempt with invalid credentials;
   - verify status `failed` and a sanitized error message.
3. SSH agent-proxy success:
   - open SSH proxy to `localhost:22` through agent `192.168.1.13`;
   - verify status transitions and proxy session ID.
4. SSH guacd success:
   - open guacd-backed SSH through the raw TCP proxy;
   - run a harmless command such as `whoami`;
   - close the session;
   - verify `typescript` and `.timing` files exist under the expected recording
     directory and are linked to the session row.
5. RDP guacd success, where a Windows target is available:
   - open RDP session;
   - verify audit status transitions and recording metadata.
6. Agent disconnected:
   - stop the agent during a proxy session;
   - verify the session becomes `agent_disconnected` or `closed` with a clear
     error, depending on where the stream ends.
7. Permissions:
   - verify a normal remote-access user can start sessions;
   - verify only authorized roles can view/download recordings.

## Implementation Order

1. Add the shared audit table and store.
2. Wire audit status updates into the existing SSH and RDP flows.
3. Add the sessions list/detail API and frontend table.
4. Add file-backed recording metadata and download endpoints.
5. Add guacd-backed SSH using a raw TCP proxy.
6. Enable SSH recording parameters and basic download.
7. Add xterm.js replay for SSH recordings.
8. Add retention cleanup and administrative settings.

## Open Questions

- Should recording be enabled globally, per tenant, per host, or per role?
- Should users be warned before entering a recorded session?
- What is the default retention period for self-hosted installs?
- Should recording downloads require a distinct permission from remote access?
- Do we need immutable/WORM-style storage support for compliance customers?
