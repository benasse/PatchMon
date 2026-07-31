package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	hostctx "github.com/PatchMon/PatchMon/server-source-code/internal/context"
	"github.com/PatchMon/PatchMon/server-source-code/internal/util"
	"github.com/redis/go-redis/v9"
)

var ErrInvalidSSHGuacdTicket = errors.New("invalid or expired SSH guacd ticket")

const (
	sshGuacdTicketPrefix = "ssh:guacd:ticket:"
	sshGuacdTicketTTL    = 5 * time.Minute
)

// SSHGuacdTicketStore stores one-time SSH-over-Guacamole tickets in Redis.
type SSHGuacdTicketStore struct {
	rdb *hostctx.RedisResolver
	enc *util.Encryption
}

// NewSSHGuacdTicketStore creates a ticket store for SSH guacd sessions.
func NewSSHGuacdTicketStore(rdb *hostctx.RedisResolver, enc *util.Encryption) *SSHGuacdTicketStore {
	return &SSHGuacdTicketStore{rdb: rdb, enc: enc}
}

// SSHGuacdTicketData is stored encrypted in Redis for Guacamole WebSocket auth.
type SSHGuacdTicketData struct {
	UserID                string `json:"userId"`
	HostID                string `json:"hostId"`
	RemoteAccessSessionID string `json:"remoteAccessSessionId,omitempty"`
	TargetHost            string `json:"targetHost"`
	TargetPort            int    `json:"targetPort"`
	Username              string `json:"username"`
	Password              string `json:"password,omitempty"`
	PrivateKey            string `json:"privateKey,omitempty"`
	Passphrase            string `json:"passphrase,omitempty"`
	Width                 int    `json:"width,omitempty"`
	Height                int    `json:"height,omitempty"`
	RecordingPath         string `json:"recordingPath,omitempty"`
	RecordingName         string `json:"recordingName,omitempty"`
	CreatedAt             int64  `json:"createdAt"`
}

// CreateTicket generates a one-time encrypted ticket for SSH guacd tunnel auth.
func (s *SSHGuacdTicketStore) CreateTicket(ctx context.Context, data SSHGuacdTicketData) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	ticket := hex.EncodeToString(b)
	key := hostctx.TenantKey(ctx, sshGuacdTicketPrefix+ticket)

	data.CreatedAt = time.Now().UnixMilli()
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	if s.enc == nil {
		return "", errors.New("ssh guacd ticket: encryption not available")
	}
	encrypted, err := s.enc.Encrypt(string(raw))
	if err != nil {
		return "", err
	}
	rdb := s.rdb.RDB(ctx)
	if rdb == nil {
		return "", errors.New("ssh guacd ticket: redis not available")
	}
	if err := rdb.Set(ctx, key, encrypted, sshGuacdTicketTTL).Err(); err != nil {
		return "", err
	}
	return ticket, nil
}

// ConsumeTicket validates and consumes a one-time SSH guacd ticket.
func (s *SSHGuacdTicketStore) ConsumeTicket(ctx context.Context, ticket string) (*SSHGuacdTicketData, error) {
	rdb := s.rdb.RDB(ctx)
	if rdb == nil {
		return nil, ErrInvalidSSHGuacdTicket
	}
	key := hostctx.TenantKey(ctx, sshGuacdTicketPrefix+ticket)
	encrypted, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrInvalidSSHGuacdTicket
		}
		return nil, err
	}
	if err := rdb.Del(ctx, key).Err(); err != nil {
		return nil, err
	}
	if s.enc == nil {
		return nil, errors.New("ssh guacd ticket: encryption not available")
	}
	raw, err := s.enc.Decrypt(encrypted)
	if err != nil {
		return nil, ErrInvalidSSHGuacdTicket
	}
	var data SSHGuacdTicketData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, ErrInvalidSSHGuacdTicket
	}
	return &data, nil
}
