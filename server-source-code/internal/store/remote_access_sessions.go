package store

import (
	"context"
	"strings"
	"time"

	"github.com/PatchMon/PatchMon/server-source-code/internal/database"
	"github.com/PatchMon/PatchMon/server-source-code/internal/db"
	"github.com/PatchMon/PatchMon/server-source-code/internal/safeconv"
	"github.com/google/uuid"
)

const (
	RemoteAccessProtocolSSH = "ssh"
	RemoteAccessProtocolRDP = "rdp"

	RemoteAccessStatusConnecting        = "connecting"
	RemoteAccessStatusConnected         = "connected"
	RemoteAccessStatusFailed            = "failed"
	RemoteAccessStatusClosed            = "closed"
	RemoteAccessStatusTimeout           = "timeout"
	RemoteAccessStatusAgentDisconnected = "agent_disconnected"

	RecordingStatusNotRequested = "not_requested"
)

// RemoteAccessSessionsStore manages audited SSH/RDP remote access sessions.
type RemoteAccessSessionsStore struct {
	db database.DBProvider
}

// NewRemoteAccessSessionsStore creates a remote access session store.
func NewRemoteAccessSessionsStore(db database.DBProvider) *RemoteAccessSessionsStore {
	return &RemoteAccessSessionsStore{db: db}
}

// RemoteAccessSession is the API-facing session shape.
type RemoteAccessSession struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"user_id"`
	Username           string     `json:"username,omitempty"`
	HostID             string     `json:"host_id"`
	HostName           string     `json:"host_name,omitempty"`
	HostHostname       *string    `json:"host_hostname,omitempty"`
	Protocol           string     `json:"protocol"`
	ConnectionMode     string     `json:"connection_mode"`
	Status             string     `json:"status"`
	StartedAt          time.Time  `json:"started_at"`
	ConnectedAt        *time.Time `json:"connected_at,omitempty"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	DurationSeconds    *int64     `json:"duration_seconds,omitempty"`
	ErrorMessage       *string    `json:"error_message,omitempty"`
	BrowserIP          *string    `json:"browser_ip,omitempty"`
	UserAgent          *string    `json:"user_agent,omitempty"`
	ProxySessionID     *string    `json:"proxy_session_id,omitempty"`
	GuacdSessionID     *string    `json:"guacd_session_id,omitempty"`
	RecordingStatus    string     `json:"recording_status"`
	RecordingPath      *string    `json:"recording_path,omitempty"`
	RecordingName      *string    `json:"recording_name,omitempty"`
	RecordingSizeBytes *int64     `json:"recording_size_bytes,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// CreateRemoteAccessSessionParams describes a newly requested remote access session.
type CreateRemoteAccessSessionParams struct {
	ID              string
	UserID          string
	HostID          string
	Protocol        string
	ConnectionMode  string
	BrowserIP       *string
	UserAgent       *string
	ProxySessionID  *string
	GuacdSessionID  *string
	RecordingStatus string
	RecordingPath   *string
	RecordingName   *string
}

// ListRemoteAccessSessionsParams configures session listing.
type ListRemoteAccessSessionsParams struct {
	Protocol string
	Status   string
	HostID   string
	UserID   string
	Search   string
	Limit    int
	Offset   int
}

// Create creates an audit row in connecting state.
func (s *RemoteAccessSessionsStore) Create(ctx context.Context, p CreateRemoteAccessSessionParams) (*RemoteAccessSession, error) {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.RecordingStatus == "" {
		p.RecordingStatus = RecordingStatusNotRequested
	}
	row, err := s.db.DB(ctx).Queries.CreateRemoteAccessSession(ctx, db.CreateRemoteAccessSessionParams{
		ID:              p.ID,
		UserID:          p.UserID,
		HostID:          p.HostID,
		Protocol:        p.Protocol,
		ConnectionMode:  p.ConnectionMode,
		BrowserIp:       p.BrowserIP,
		UserAgent:       p.UserAgent,
		ProxySessionID:  p.ProxySessionID,
		GuacdSessionID:  p.GuacdSessionID,
		RecordingStatus: p.RecordingStatus,
		RecordingPath:   p.RecordingPath,
		RecordingName:   p.RecordingName,
	})
	if err != nil {
		return nil, err
	}
	return remoteAccessSessionFromDB(row), nil
}

// Get returns one session with display names.
func (s *RemoteAccessSessionsStore) Get(ctx context.Context, id string) (*RemoteAccessSession, error) {
	row, err := s.db.DB(ctx).Queries.GetRemoteAccessSession(ctx, id)
	if err != nil {
		return nil, err
	}
	out := remoteAccessSessionFromGetRow(row)
	return &out, nil
}

// List returns paginated sessions and total filtered count.
func (s *RemoteAccessSessionsStore) List(ctx context.Context, p ListRemoteAccessSessionsParams) ([]RemoteAccessSession, int64, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	offset := p.Offset
	if offset < 0 {
		offset = 0
	}
	search := strings.TrimSpace(p.Search)
	countParams := db.CountRemoteAccessSessionsParams{
		Protocol: p.Protocol,
		Status:   p.Status,
		HostID:   p.HostID,
		UserID:   p.UserID,
		Search:   search,
	}
	total, err := s.db.DB(ctx).Queries.CountRemoteAccessSessions(ctx, countParams)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.db.DB(ctx).Queries.ListRemoteAccessSessions(ctx, db.ListRemoteAccessSessionsParams{
		Protocol:  p.Protocol,
		Status:    p.Status,
		HostID:    p.HostID,
		UserID:    p.UserID,
		Search:    search,
		RowOffset: safeconv.ClampToInt32(offset),
		RowLimit:  safeconv.ClampToInt32(limit),
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]RemoteAccessSession, 0, len(rows))
	for _, row := range rows {
		out = append(out, remoteAccessSessionFromListRow(row))
	}
	return out, int64(total), nil
}

// MarkConnected records that the protocol connection was established.
func (s *RemoteAccessSessionsStore) MarkConnected(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	return s.db.DB(ctx).Queries.MarkRemoteAccessSessionConnected(ctx, id)
}

// MarkEnded records a terminal state. Repeated calls are ignored by the query.
func (s *RemoteAccessSessionsStore) MarkEnded(ctx context.Context, id, status string, errorMessage *string) error {
	if id == "" {
		return nil
	}
	if status == "" {
		status = RemoteAccessStatusClosed
	}
	return s.db.DB(ctx).Queries.MarkRemoteAccessSessionEnded(ctx, db.MarkRemoteAccessSessionEndedParams{
		ID:           id,
		Status:       status,
		ErrorMessage: errorMessage,
	})
}

// SetProxyID links a generated transport proxy ID to the audit row.
func (s *RemoteAccessSessionsStore) SetProxyID(ctx context.Context, id, proxySessionID string) error {
	if id == "" || proxySessionID == "" {
		return nil
	}
	return s.db.DB(ctx).Queries.SetRemoteAccessSessionProxyID(ctx, db.SetRemoteAccessSessionProxyIDParams{
		ID:             id,
		ProxySessionID: proxySessionID,
	})
}

// SetGuacdID links a guacd connection ID to the audit row.
func (s *RemoteAccessSessionsStore) SetGuacdID(ctx context.Context, id, guacdSessionID string) error {
	if id == "" || guacdSessionID == "" {
		return nil
	}
	return s.db.DB(ctx).Queries.SetRemoteAccessSessionGuacdID(ctx, db.SetRemoteAccessSessionGuacdIDParams{
		ID:             id,
		GuacdSessionID: guacdSessionID,
	})
}

// SetRecording updates recording metadata for a remote access session.
func (s *RemoteAccessSessionsStore) SetRecording(ctx context.Context, id, status string, path, name *string, sizeBytes *int64) error {
	if id == "" {
		return nil
	}
	return s.db.DB(ctx).Queries.SetRemoteAccessSessionRecording(ctx, db.SetRemoteAccessSessionRecordingParams{
		ID:                 id,
		RecordingStatus:    status,
		RecordingPath:      path,
		RecordingName:      name,
		RecordingSizeBytes: sizeBytes,
	})
}

func remoteAccessSessionFromDB(r db.RemoteAccessSession) *RemoteAccessSession {
	return &RemoteAccessSession{
		ID:                 r.ID,
		UserID:             r.UserID,
		HostID:             r.HostID,
		Protocol:           r.Protocol,
		ConnectionMode:     r.ConnectionMode,
		Status:             r.Status,
		StartedAt:          pgTime(r.StartedAt),
		ConnectedAt:        pgTimePtr(r.ConnectedAt),
		EndedAt:            pgTimePtr(r.EndedAt),
		ErrorMessage:       r.ErrorMessage,
		BrowserIP:          r.BrowserIp,
		UserAgent:          r.UserAgent,
		ProxySessionID:     r.ProxySessionID,
		GuacdSessionID:     r.GuacdSessionID,
		RecordingStatus:    r.RecordingStatus,
		RecordingPath:      r.RecordingPath,
		RecordingName:      r.RecordingName,
		RecordingSizeBytes: r.RecordingSizeBytes,
		CreatedAt:          pgTime(r.CreatedAt),
		UpdatedAt:          pgTime(r.UpdatedAt),
	}
}

func remoteAccessSessionFromGetRow(r db.GetRemoteAccessSessionRow) RemoteAccessSession {
	out := *remoteAccessSessionFromDB(db.RemoteAccessSession{
		ID:                 r.ID,
		UserID:             r.UserID,
		HostID:             r.HostID,
		Protocol:           r.Protocol,
		ConnectionMode:     r.ConnectionMode,
		Status:             r.Status,
		StartedAt:          r.StartedAt,
		ConnectedAt:        r.ConnectedAt,
		EndedAt:            r.EndedAt,
		ErrorMessage:       r.ErrorMessage,
		BrowserIp:          r.BrowserIp,
		UserAgent:          r.UserAgent,
		ProxySessionID:     r.ProxySessionID,
		GuacdSessionID:     r.GuacdSessionID,
		RecordingStatus:    r.RecordingStatus,
		RecordingPath:      r.RecordingPath,
		RecordingName:      r.RecordingName,
		RecordingSizeBytes: r.RecordingSizeBytes,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	})
	out.Username = r.UserUsername
	out.HostName = r.HostFriendlyName
	out.HostHostname = r.HostHostname
	out.DurationSeconds = remoteAccessDurationSeconds(out.StartedAt, out.ConnectedAt, out.EndedAt)
	return out
}

func remoteAccessSessionFromListRow(r db.ListRemoteAccessSessionsRow) RemoteAccessSession {
	out := *remoteAccessSessionFromDB(db.RemoteAccessSession{
		ID:                 r.ID,
		UserID:             r.UserID,
		HostID:             r.HostID,
		Protocol:           r.Protocol,
		ConnectionMode:     r.ConnectionMode,
		Status:             r.Status,
		StartedAt:          r.StartedAt,
		ConnectedAt:        r.ConnectedAt,
		EndedAt:            r.EndedAt,
		ErrorMessage:       r.ErrorMessage,
		BrowserIp:          r.BrowserIp,
		UserAgent:          r.UserAgent,
		ProxySessionID:     r.ProxySessionID,
		GuacdSessionID:     r.GuacdSessionID,
		RecordingStatus:    r.RecordingStatus,
		RecordingPath:      r.RecordingPath,
		RecordingName:      r.RecordingName,
		RecordingSizeBytes: r.RecordingSizeBytes,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	})
	out.Username = r.UserUsername
	out.HostName = r.HostFriendlyName
	out.HostHostname = r.HostHostname
	out.DurationSeconds = remoteAccessDurationSeconds(out.StartedAt, out.ConnectedAt, out.EndedAt)
	return out
}

func remoteAccessDurationSeconds(startedAt time.Time, connectedAt, endedAt *time.Time) *int64 {
	if endedAt == nil {
		return nil
	}
	start := startedAt
	if connectedAt != nil {
		start = *connectedAt
	}
	seconds := int64(endedAt.Sub(start).Seconds())
	if seconds < 0 {
		seconds = 0
	}
	return &seconds
}
