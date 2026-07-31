package store

import (
	"context"
	"errors"
	"time"

	"github.com/PatchMon/PatchMon/server-source-code/internal/database"
	"github.com/PatchMon/PatchMon/server-source-code/internal/db"
	"github.com/PatchMon/PatchMon/server-source-code/internal/safeconv"
	"github.com/jackc/pgx/v5/pgtype"
)

// SSHStore is a small compatibility layer for the pty_agent bastion code.
// It intentionally stores metadata in remote_access_sessions rather than in a
// separate ssh_sessions table.
type SSHStore struct {
	remote *RemoteAccessSessionsStore
}

func NewSSHStore(provider database.DBProvider) *SSHStore {
	return &SSHStore{remote: NewRemoteAccessSessionsStore(provider)}
}

func NewSSHRemoteAccessStore(remote *RemoteAccessSessionsStore) *SSHStore {
	return &SSHStore{remote: remote}
}

type CreateSSHSessionParams struct {
	ID, HostID, UserID, LinuxUsername, Transport, ClientIP, UserAgent string
	Recorded                                                          bool
}

type SSHSession struct {
	ID string
}

func (s *SSHStore) CreateSession(ctx context.Context, value CreateSSHSessionParams) (SSHSession, error) {
	if s == nil || s.remote == nil {
		return SSHSession{}, errors.New("ssh store is not configured")
	}
	mode := "pty_agent"
	clientType := value.Transport
	recordingStatus := RecordingStatusAvailable
	if value.Transport == "tunnel" {
		mode = "agent_tunnel"
		recordingStatus = RecordingStatusNotRequested
	}
	clientIP, userAgent, linuxUsername, clientTypePtr := optionalPtr(value.ClientIP), optionalPtr(value.UserAgent), optionalPtr(value.LinuxUsername), optionalPtr(clientType)
	recordingName := optionalPtr(value.ID)
	if mode == "agent_tunnel" {
		recordingName = nil
	}
	session, err := s.remote.Create(ctx, CreateRemoteAccessSessionParams{
		ID:              value.ID,
		UserID:          value.UserID,
		HostID:          value.HostID,
		Protocol:        RemoteAccessProtocolSSH,
		ConnectionMode:  mode,
		BrowserIP:       clientIP,
		UserAgent:       userAgent,
		LinuxUsername:   linuxUsername,
		ClientType:      clientTypePtr,
		RecordingStatus: recordingStatus,
		RecordingName:   recordingName,
	})
	if err != nil {
		return SSHSession{}, err
	}
	return SSHSession{ID: session.ID}, nil
}

func (s *SSHStore) UpdateSession(ctx context.Context, id, status, reason string, events, bytes int64) (SSHSession, error) {
	if s == nil || s.remote == nil {
		return SSHSession{}, errors.New("ssh store is not configured")
	}
	switch status {
	case "active":
		if err := s.remote.MarkConnected(ctx, id); err != nil {
			return SSHSession{}, err
		}
	case "failed":
		if err := s.remote.MarkEnded(ctx, id, RemoteAccessStatusFailed, optionalPtr(reason)); err != nil {
			return SSHSession{}, err
		}
	case "disconnected":
		if err := s.remote.MarkEnded(ctx, id, RemoteAccessStatusAgentDisconnected, optionalPtr(reason)); err != nil {
			return SSHSession{}, err
		}
	default:
		if err := s.remote.MarkEnded(ctx, id, RemoteAccessStatusClosed, optionalPtr(reason)); err != nil {
			return SSHSession{}, err
		}
	}
	if events > 0 || bytes > 0 {
		var sizePtr *int64
		if bytes > 0 {
			sizePtr = &bytes
		}
		eventPtr := &events
		_ = s.remote.SetRecordingWithEventCount(ctx, id, RecordingStatusAvailable, nil, optionalPtr(id), sizePtr, eventPtr)
	}
	return SSHSession{ID: id}, nil
}

func (s *SSHStore) ActiveCounts(ctx context.Context, userID, hostID string) (int64, int64, error) {
	if s == nil || s.remote == nil {
		return 0, 0, errors.New("ssh store is not configured")
	}
	return s.remote.ActiveCounts(ctx, userID, hostID)
}

func (s *SSHStore) ExpiredRecordings(ctx context.Context, before time.Time, limit int32) ([]SSHSession, error) {
	if s == nil || s.remote == nil {
		return nil, errors.New("ssh store is not configured")
	}
	rows, err := s.remote.db.DB(ctx).Queries.ListExpiredRemoteAccessRecordings(ctx, db.ListExpiredRemoteAccessRecordingsParams{
		StartedBefore: pgtype.Timestamp{Time: before, Valid: true},
		RowLimit:      safeconv.ClampToInt32(int(limit)),
	})
	if err != nil {
		return nil, err
	}
	out := make([]SSHSession, 0, len(rows))
	for _, row := range rows {
		out = append(out, SSHSession{ID: row.ID})
	}
	return out, nil
}

func (s *SSHStore) MarkRecordingDeleted(ctx context.Context, id string) error {
	if s == nil || s.remote == nil {
		return errors.New("ssh store is not configured")
	}
	return s.remote.MarkRecordingDeleted(ctx, id)
}

func optionalPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
