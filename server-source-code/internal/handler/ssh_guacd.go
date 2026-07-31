package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/PatchMon/PatchMon/server-source-code/internal/clientip"
	hostctx "github.com/PatchMon/PatchMon/server-source-code/internal/context"
	"github.com/PatchMon/PatchMon/server-source-code/internal/database"
	"github.com/PatchMon/PatchMon/server-source-code/internal/middleware"
	"github.com/PatchMon/PatchMon/server-source-code/internal/models"
	"github.com/PatchMon/PatchMon/server-source-code/internal/notifications"
	"github.com/PatchMon/PatchMon/server-source-code/internal/store"
	"github.com/google/uuid"
	"github.com/wwt/guac"
)

var ErrSSHGuacdTicketRequired = errors.New("valid SSH guacd ticket required")

// SSHGuacdHandler handles SSH sessions rendered through guacd.
type SSHGuacdHandler struct {
	tickets        *store.SSHGuacdTicketStore
	hosts          *store.HostsStore
	users          *store.UsersStore
	permissions    *store.PermissionsStore
	guacdAddress   string
	allowedOrigins []string
	originResolver middleware.OriginResolver
	log            *slog.Logger
	db             database.DBProvider
	notify         *notifications.Emitter
	remote         *store.RemoteAccessSessionsStore
}

func NewSSHGuacdHandler(
	tickets *store.SSHGuacdTicketStore,
	hosts *store.HostsStore,
	users *store.UsersStore,
	permissions *store.PermissionsStore,
	guacdAddress string,
	corsOrigin string,
	originResolver middleware.OriginResolver,
	log *slog.Logger,
	db database.DBProvider,
	notify *notifications.Emitter,
	remote *store.RemoteAccessSessionsStore,
) *SSHGuacdHandler {
	return &SSHGuacdHandler{
		tickets:        tickets,
		hosts:          hosts,
		users:          users,
		permissions:    permissions,
		guacdAddress:   guacdAddress,
		allowedOrigins: parseAllowedOrigins(corsOrigin),
		originResolver: originResolver,
		log:            log,
		db:             db,
		notify:         notify,
		remote:         remote,
	}
}

func (h *SSHGuacdHandler) ServeCreateTicket(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.UserIDKey).(string)
	if userID == "" {
		JSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var req struct {
		HostID     string `json:"hostId"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		PrivateKey string `json:"privateKey"`
		Passphrase string `json:"passphrase"`
		Port       int    `json:"port,omitempty"`
		AuthMethod string `json:"authMethod,omitempty"`
		Width      int    `json:"width,omitempty"`
		Height     int    `json:"height,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}
	req.HostID = strings.TrimSpace(req.HostID)
	req.Username = strings.TrimSpace(req.Username)
	if req.HostID == "" {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "hostId is required"})
		return
	}
	if req.Username == "" {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "username is required"})
		return
	}
	if req.Port <= 0 {
		req.Port = 22
	}
	if req.Port > 65535 {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "port must be between 1 and 65535"})
		return
	}
	if req.Width < 480 {
		req.Width = 1024
	}
	if req.Height < 240 {
		req.Height = 520
	}
	if req.Width > 8192 {
		req.Width = 8192
	}
	if req.Height > 8192 {
		req.Height = 8192
	}
	if req.AuthMethod == "key" {
		if strings.TrimSpace(req.PrivateKey) == "" {
			JSON(w, http.StatusBadRequest, map[string]string{"error": "privateKey is required"})
			return
		}
	} else if req.Password == "" {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "password is required"})
		return
	}

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil || user == nil || !user.IsActive {
		JSON(w, http.StatusUnauthorized, map[string]string{"error": "User not found or inactive"})
		return
	}
	canUse, err := h.userCanUseRemoteAccess(r.Context(), user)
	if err != nil {
		h.log.Warn("ssh-guacd permission lookup failed", "user_id", userID, "role", user.Role, "error", err)
		JSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to verify permissions"})
		return
	}
	if !canUse {
		JSON(w, http.StatusForbidden, map[string]string{"error": "Access denied"})
		return
	}

	host, err := h.hosts.GetByID(r.Context(), req.HostID)
	if err != nil || host == nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": "Host not found"})
		return
	}
	targetHost := sshGuacdTargetHost(host)
	if targetHost == "" {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "Host has no IP or hostname reachable by guacd"})
		return
	}

	remoteSessionID := h.createRemoteAccessSession(r.Context(), host, user, clientip.FromRequest(r), r.UserAgent())
	if probe, err := net.DialTimeout("tcp", h.guacdAddress, guacdPreflightTimeout); err != nil {
		h.log.Warn("ssh-guacd guacd preflight failed", "addr", h.guacdAddress, "error", err)
		h.markRemoteAccessEnded(r.Context(), remoteSessionID, store.RemoteAccessStatusFailed, strPtr("guacd unavailable: "+err.Error()))
		JSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "guacd is not reachable on the PatchMon server. Install/start guacd or set GUACD_ADDRESS to a running sidecar.",
			"code":  "guacd_unavailable",
		})
		return
	} else {
		_ = probe.Close()
	}

	recordingName := "ssh-" + remoteSessionID
	recordingPath := "/var/lib/patchmon/ssh-recordings"
	ticket, err := h.tickets.CreateTicket(r.Context(), store.SSHGuacdTicketData{
		UserID:                userID,
		HostID:                host.ID,
		RemoteAccessSessionID: remoteSessionID,
		TargetHost:            targetHost,
		TargetPort:            req.Port,
		Username:              req.Username,
		Password:              req.Password,
		PrivateKey:            req.PrivateKey,
		Passphrase:            req.Passphrase,
		Width:                 req.Width,
		Height:                req.Height,
		RecordingPath:         recordingPath,
		RecordingName:         recordingName,
	})
	if err != nil {
		h.markRemoteAccessEnded(r.Context(), remoteSessionID, store.RemoteAccessStatusFailed, strPtr("Failed to create SSH guacd ticket: "+err.Error()))
		JSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create SSH guacd ticket"})
		return
	}

	if h.notify != nil {
		if d := h.db.DB(r.Context()); d != nil {
			hostName := host.FriendlyName
			if hostName == "" && host.Hostname != nil {
				hostName = *host.Hostname
			}
			h.notify.EmitEvent(r.Context(), d, hostctx.TenantHostKey(r.Context()), notifications.Event{
				Type:          "ssh_session_started",
				Severity:      "informational",
				Title:         fmt.Sprintf("SSH Session - %s", hostName),
				Message:       fmt.Sprintf("SSH guacd session initiated to host \"%s\".", hostName),
				ReferenceType: "host",
				ReferenceID:   host.ID,
				Metadata: map[string]interface{}{
					"host_id":         host.ID,
					"host_name":       hostName,
					"user_id":         userID,
					"connection_mode": "guacd",
				},
			})
		}
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"ticket":             ticket,
		"websocketTunnelUrl": "/api/v1/ssh-guacd/websocket-tunnel",
		"expiresIn":          300,
	})
}

func (h *SSHGuacdHandler) WebsocketTunnelHandler() http.Handler {
	guacWSHandler := guac.NewWebsocketServer(h.doGuacConnect)
	guacWSHandler.OnDisconnect = func(_ string, r *http.Request, tunnel guac.Tunnel) {
		if audited, ok := tunnel.(*auditedGuacTunnel); ok {
			h.markRemoteAccessEnded(r.Context(), audited.remoteAccessSessionID, store.RemoteAccessStatusClosed, nil)
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.isOriginAllowed(r) {
			h.log.Info("ssh-guacd tunnel origin rejected", "origin", r.Header.Get("Origin"))
			http.Error(w, "Forbidden: origin not allowed", http.StatusForbidden)
			return
		}
		guacWSHandler.ServeHTTP(w, r)
	})
}

func (h *SSHGuacdHandler) doGuacConnect(r *http.Request) (guac.Tunnel, error) {
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		return nil, ErrSSHGuacdTicketRequired
	}
	data, err := h.tickets.ConsumeTicket(r.Context(), ticket)
	if err != nil {
		h.log.Info("ssh-guacd tunnel invalid ticket", "error", err)
		return nil, ErrSSHGuacdTicketRequired
	}

	user, err := h.users.GetByID(r.Context(), data.UserID)
	if err != nil || user == nil || !user.IsActive {
		h.markRemoteAccessEnded(r.Context(), data.RemoteAccessSessionID, store.RemoteAccessStatusFailed, strPtr("User revoked or inactive before SSH guacd tunnel opened"))
		return nil, ErrSSHGuacdTicketRequired
	}
	canUse, err := h.userCanUseRemoteAccess(r.Context(), user)
	if err != nil || !canUse {
		h.markRemoteAccessEnded(r.Context(), data.RemoteAccessSessionID, store.RemoteAccessStatusFailed, strPtr("User lacks remote access permission"))
		return nil, ErrSSHGuacdTicketRequired
	}

	conn, err := net.Dial("tcp", h.guacdAddress)
	if err != nil {
		h.log.Error("ssh-guacd tunnel guacd connect failed", "addr", h.guacdAddress, "error", err)
		h.markRemoteAccessEnded(r.Context(), data.RemoteAccessSessionID, store.RemoteAccessStatusFailed, strPtr("guacd connect failed: "+err.Error()))
		return nil, err
	}

	stream := guac.NewStream(conn, guac.SocketTimeout)
	config := guac.NewGuacamoleConfiguration()
	config.Protocol = "ssh"
	config.OptimalScreenWidth = data.Width
	config.OptimalScreenHeight = data.Height
	config.Parameters = map[string]string{
		"hostname":               data.TargetHost,
		"port":                   strconv.Itoa(data.TargetPort),
		"username":               data.Username,
		"terminal-type":          "xterm-256color",
		"font-name":              "monospace",
		"font-size":              "14",
		"typescript-path":        data.RecordingPath,
		"typescript-name":        data.RecordingName,
		"create-typescript-path": "true",
	}
	if data.Password != "" {
		config.Parameters["password"] = data.Password
	}
	if data.PrivateKey != "" {
		config.Parameters["private-key"] = data.PrivateKey
	}
	if data.Passphrase != "" {
		config.Parameters["passphrase"] = data.Passphrase
	}

	if err := stream.Handshake(config); err != nil {
		_ = conn.Close()
		h.log.Error("ssh-guacd tunnel handshake failed", "user_id", data.UserID, "host_id", data.HostID, "target", data.TargetHost, "error", err)
		h.markRemoteAccessEnded(r.Context(), data.RemoteAccessSessionID, store.RemoteAccessStatusFailed, strPtr("guacd SSH handshake failed: "+err.Error()))
		return nil, err
	}

	tunnel := guac.NewSimpleTunnel(stream)
	connectionID := tunnel.ConnectionID()
	h.markRemoteAccessConnected(r.Context(), data.RemoteAccessSessionID)
	h.setRemoteAccessGuacdID(r.Context(), data.RemoteAccessSessionID, connectionID)
	h.setRemoteAccessRecording(r.Context(), data.RemoteAccessSessionID, data.RecordingPath, data.RecordingName)
	h.log.Info("ssh-guacd session opened", "guacd_connection_id", connectionID, "user_id", data.UserID, "host_id", data.HostID, "target", data.TargetHost)

	return &auditedGuacTunnel{
		Tunnel:                tunnel,
		remoteAccessSessionID: data.RemoteAccessSessionID,
	}, nil
}

func (h *SSHGuacdHandler) userCanUseRemoteAccess(ctx context.Context, user *models.User) (bool, error) {
	if user == nil || !user.IsActive {
		return false, nil
	}
	if user.Role == "admin" || user.Role == "superadmin" {
		return true, nil
	}
	perm, err := h.permissions.GetByRole(ctx, user.Role)
	if err != nil {
		return false, err
	}
	return perm != nil && perm.CanUseRemoteAccess && perm.CanViewHosts, nil
}

func (h *SSHGuacdHandler) isOriginAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	effectiveOrigins := h.allowedOrigins
	if h.originResolver != nil {
		if dynamicOrigin, ok := h.originResolver(r); ok && dynamicOrigin != "" {
			effectiveOrigins = []string{dynamicOrigin}
		}
	}
	for _, allowed := range effectiveOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

func (h *SSHGuacdHandler) createRemoteAccessSession(ctx context.Context, host *models.Host, user *models.User, browserIP, userAgent string) string {
	if h.remote == nil || host == nil || user == nil {
		return ""
	}
	var ipPtr *string
	if browserIP != "" {
		ipPtr = &browserIP
	}
	var uaPtr *string
	if userAgent != "" {
		uaPtr = &userAgent
	}
	session, err := h.remote.Create(ctx, store.CreateRemoteAccessSessionParams{
		UserID:          user.ID,
		HostID:          host.ID,
		Protocol:        store.RemoteAccessProtocolSSH,
		ConnectionMode:  "guacd",
		BrowserIP:       ipPtr,
		UserAgent:       uaPtr,
		RecordingStatus: "requested",
	})
	if err != nil {
		if h.log != nil {
			h.log.Warn("ssh-guacd remote access audit create failed", "host_id", host.ID, "user_id", user.ID, "error", err)
		}
		return ""
	}
	return session.ID
}

func (h *SSHGuacdHandler) markRemoteAccessConnected(ctx context.Context, id string) {
	if h.remote == nil || id == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := h.remote.MarkConnected(context.WithoutCancel(ctx), id); err != nil && h.log != nil {
		h.log.Warn("ssh-guacd remote access audit connected update failed", "session_id", id, "error", err)
	}
}

func (h *SSHGuacdHandler) markRemoteAccessEnded(ctx context.Context, id, status string, msg *string) {
	if h.remote == nil || id == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := h.remote.MarkEnded(context.WithoutCancel(ctx), id, status, msg); err != nil && h.log != nil {
		h.log.Warn("ssh-guacd remote access audit end update failed", "session_id", id, "status", status, "error", err)
	}
}

func (h *SSHGuacdHandler) setRemoteAccessGuacdID(ctx context.Context, id, guacdID string) {
	if h.remote == nil || id == "" || guacdID == "" {
		return
	}
	if err := h.remote.SetGuacdID(context.WithoutCancel(ctx), id, guacdID); err != nil && h.log != nil {
		h.log.Warn("ssh-guacd remote access audit guacd update failed", "session_id", id, "guacd_session_id", guacdID, "error", err)
	}
}

func (h *SSHGuacdHandler) setRemoteAccessRecording(ctx context.Context, id, path, name string) {
	if h.remote == nil || id == "" {
		return
	}
	if err := h.remote.SetRecording(context.WithoutCancel(ctx), id, "recording", strPtr(path), strPtr(name), nil); err != nil && h.log != nil {
		h.log.Warn("ssh-guacd remote access audit recording update failed", "session_id", id, "error", err)
	}
}

func sshGuacdTargetHost(host *models.Host) string {
	if host == nil {
		return ""
	}
	if host.IP != nil && strings.TrimSpace(*host.IP) != "" {
		return strings.TrimSpace(*host.IP)
	}
	if host.Hostname != nil && strings.TrimSpace(*host.Hostname) != "" {
		return strings.TrimSpace(*host.Hostname)
	}
	return ""
}

func newSSHGuacdRecordingName() string {
	return "ssh-" + uuid.NewString()
}
