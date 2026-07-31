package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/PatchMon/PatchMon/server-source-code/internal/store"
)

const remoteAccessRecordingRoot = "/var/lib/patchmon/ssh-recordings"

// RemoteAccessSessionsHandler serves audited remote access sessions.
type RemoteAccessSessionsHandler struct {
	sessions *store.RemoteAccessSessionsStore
}

// NewRemoteAccessSessionsHandler creates a remote access sessions handler.
func NewRemoteAccessSessionsHandler(sessions *store.RemoteAccessSessionsStore) *RemoteAccessSessionsHandler {
	return &RemoteAccessSessionsHandler{sessions: sessions}
}

// List handles GET /remote-access/sessions.
func (h *RemoteAccessSessionsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	rows, total, err := h.sessions.List(r.Context(), store.ListRemoteAccessSessionsParams{
		Protocol: r.URL.Query().Get("protocol"),
		Status:   r.URL.Query().Get("status"),
		HostID:   r.URL.Query().Get("hostId"),
		UserID:   r.URL.Query().Get("userId"),
		Search:   r.URL.Query().Get("search"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to load remote access sessions")
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"sessions": rows,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// Get handles GET /remote-access/sessions/{id}.
func (h *RemoteAccessSessionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		Error(w, http.StatusBadRequest, "Session ID required")
		return
	}
	session, err := h.sessions.Get(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "Remote access session not found")
		return
	}
	JSON(w, http.StatusOK, session)
}

// Recording handles GET /remote-access/sessions/{id}/recording.
func (h *RemoteAccessSessionsHandler) Recording(w http.ResponseWriter, r *http.Request) {
	session, err := h.recordingSession(r)
	if err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}
	recordingFile, timingFile, err := recordingFiles(session)
	if err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}
	recordingInfo, err := os.Stat(recordingFile)
	if err != nil {
		Error(w, http.StatusNotFound, "Recording file not found")
		return
	}
	timingInfo, err := os.Stat(timingFile)
	if err != nil {
		Error(w, http.StatusNotFound, "Recording timing file not found")
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"session":            session,
		"recording_size":     recordingInfo.Size(),
		"timing_size":        timingInfo.Size(),
		"recording_url":      fmt.Sprintf("/api/v1/remote-access/recordings/%s/data", url.PathEscape(session.ID)),
		"timing_url":         fmt.Sprintf("/api/v1/remote-access/recordings/%s/timing", url.PathEscape(session.ID)),
		"download_url":       fmt.Sprintf("/api/v1/remote-access/recordings/%s/download", url.PathEscape(session.ID)),
		"recording_filename": filepath.Base(recordingFile),
		"timing_filename":    filepath.Base(timingFile),
	})
}

// RecordingData serves the raw Guacamole SSH typescript file.
func (h *RemoteAccessSessionsHandler) RecordingData(w http.ResponseWriter, r *http.Request) {
	h.serveRecordingFile(w, r, false, false)
}

// RecordingTiming serves the Guacamole timing file.
func (h *RemoteAccessSessionsHandler) RecordingTiming(w http.ResponseWriter, r *http.Request) {
	h.serveRecordingFile(w, r, true, false)
}

// RecordingDownload downloads the raw typescript file.
func (h *RemoteAccessSessionsHandler) RecordingDownload(w http.ResponseWriter, r *http.Request) {
	h.serveRecordingFile(w, r, false, true)
}

func (h *RemoteAccessSessionsHandler) serveRecordingFile(w http.ResponseWriter, r *http.Request, timing, download bool) {
	session, err := h.recordingSession(r)
	if err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}
	recordingFile, timingFile, err := recordingFiles(session)
	if err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}
	filePath := recordingFile
	if timing {
		filePath = timingFile
	}
	f, err := os.Open(filePath)
	if err != nil {
		Error(w, http.StatusNotFound, "Recording file not found")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		Error(w, http.StatusNotFound, "Recording file not found")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if download {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(filePath)))
	}
	http.ServeContent(w, r, filepath.Base(filePath), info.ModTime(), f)
}

func (h *RemoteAccessSessionsHandler) recordingSession(r *http.Request) (*store.RemoteAccessSession, error) {
	id := r.PathValue("id")
	if id == "" {
		return nil, errors.New("Session ID required")
	}
	session, err := h.sessions.Get(r.Context(), id)
	if err != nil {
		return nil, errors.New("Remote access session not found")
	}
	if session.Protocol != store.RemoteAccessProtocolSSH || session.ConnectionMode != "guacd" {
		return nil, errors.New("Recording playback is only available for SSH guacd sessions")
	}
	if session.RecordingName == nil || strings.TrimSpace(*session.RecordingName) == "" {
		return nil, errors.New("Recording is not available for this session")
	}
	return session, nil
}

func recordingFiles(session *store.RemoteAccessSession) (string, string, error) {
	if session == nil || session.RecordingName == nil {
		return "", "", errors.New("Recording is not available for this session")
	}
	name := filepath.Base(strings.TrimSpace(*session.RecordingName))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "", "", errors.New("Invalid recording name")
	}
	baseRoot, err := filepath.Abs(remoteAccessRecordingRoot)
	if err != nil {
		return "", "", err
	}
	recordingFile := filepath.Join(baseRoot, name)
	timingFile := recordingFile + ".timing"
	if !strings.HasPrefix(recordingFile, baseRoot+string(filepath.Separator)) {
		return "", "", errors.New("Invalid recording path")
	}
	return recordingFile, timingFile, nil
}
