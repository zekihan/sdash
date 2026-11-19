package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var errInvalidMAC = errors.New("invalid MAC address")

const (
	prefsFileMode  = 0o600
	wolMagicByte   = byte(0xff)
	actionTimeout  = 9 * time.Second
	maxPrefsBytes  = 64 << 10
	wolPort        = 9
	magicPacketLen = 102
	macByteCount   = 6
	magicRepeat    = 16
)

// ---------- write actions (config-gated) ----------.

// actionsEnabled reports whether write actions are permitted at all.
func (srv *Server) actionsEnabled(w http.ResponseWriter) bool {
	if srv.deps.Config.Actions.Enabled {
		return true
	}

	writeError(w, http.StatusForbidden, "actions are disabled (set actions.enabled)")

	return false
}

func decodeAction(w http.ResponseWriter, r *http.Request, out any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxPrefsBytes))
	if err != nil || json.Unmarshal(body, out) != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")

		return false
	}

	return true
}

type argoSyncRequest struct {
	App string `json:"app"`
}

func (srv *Server) handleArgoSync(w http.ResponseWriter, r *http.Request) {
	if !srv.actionsEnabled(w) {
		return
	}

	if srv.deps.Kube == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes not configured")

		return
	}

	request := new(argoSyncRequest)
	if !decodeAction(w, r, request) {
		return
	}

	if request.App == "" {
		writeError(w, http.StatusBadRequest, "app is required")

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), actionTimeout)
	defer cancel()

	err := srv.deps.Kube.SyncArgoApplication(ctx, request.App)
	if err != nil {
		slog.WarnContext(
			r.Context(),
			"Argo sync failed",
			slog.String("app", request.App),
			slog.Any("error", err),
		)
		writeError(w, http.StatusBadGateway, "sync failed")

		return
	}

	slog.InfoContext(r.Context(), "Argo sync triggered", slog.String("app", request.App))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type restartRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (srv *Server) handleRestartDeployment(w http.ResponseWriter, r *http.Request) {
	if !srv.actionsEnabled(w) {
		return
	}

	if srv.deps.Kube == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes not configured")

		return
	}

	request := new(restartRequest)
	if !decodeAction(w, r, request) {
		return
	}

	if request.Namespace == "" || request.Name == "" {
		writeError(w, http.StatusBadRequest, "namespace and name are required")

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), actionTimeout)
	defer cancel()

	err := srv.deps.Kube.RestartDeployment(ctx, request.Namespace, request.Name)
	if err != nil {
		slog.WarnContext(r.Context(), "Rollout restart failed",
			slog.String("deployment", request.Namespace+"/"+request.Name),
			slog.Any("error", err))
		writeError(w, http.StatusBadGateway, "restart failed")

		return
	}

	slog.InfoContext(r.Context(), "Rollout restart triggered",
		slog.String("deployment", request.Namespace+"/"+request.Name))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type proxmoxStartRequest struct {
	Node string `json:"node"`
	Kind string `json:"kind"`
	VMID int    `json:"vmid"`
}

func (srv *Server) handleProxmoxStart(w http.ResponseWriter, r *http.Request) {
	if !srv.actionsEnabled(w) {
		return
	}

	if srv.deps.Proxmox == nil {
		writeError(w, http.StatusServiceUnavailable, "proxmox not configured")

		return
	}

	request := new(proxmoxStartRequest)
	if !decodeAction(w, r, request) {
		return
	}

	if request.Node == "" || request.VMID <= 0 {
		writeError(w, http.StatusBadRequest, "node and vmid are required")

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), actionTimeout)
	defer cancel()

	err := srv.deps.Proxmox.StartGuest(ctx, request.Node, request.Kind, request.VMID)
	if err != nil {
		slog.WarnContext(r.Context(), "Proxmox guest start failed", slog.Any("error", err))
		writeError(w, http.StatusBadGateway, "start failed")

		return
	}

	slog.InfoContext(r.Context(), "Proxmox guest start triggered",
		slog.String("node", request.Node),
		slog.Int("vmid", request.VMID))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type wolRequest struct {
	Name string `json:"name"`
}

func (srv *Server) handleWakeOnLan(w http.ResponseWriter, r *http.Request) {
	if !srv.actionsEnabled(w) {
		return
	}

	request := new(wolRequest)
	if !decodeAction(w, r, request) {
		return
	}

	for _, host := range srv.deps.Config.Actions.WakeOnLan {
		if host.Name != request.Name {
			continue
		}

		err := sendWakeOnLan(host.MAC, host.Broadcast)
		if err != nil {
			slog.WarnContext(r.Context(), "Wake-on-LAN failed",
				slog.String("host", host.Name),
				slog.Any("error", err))
			writeError(w, http.StatusBadGateway, "wake-on-lan failed")

			return
		}

		slog.InfoContext(r.Context(), "Wake-on-LAN packet sent", slog.String("host", host.Name))
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

		return
	}

	writeError(w, http.StatusNotFound, "unknown wake host")
}

type homeAssistantRequest struct {
	EntityID string `json:"entityId"`
}

func (srv *Server) handleHomeAssistantToggle(w http.ResponseWriter, r *http.Request) {
	if !srv.actionsEnabled(w) {
		return
	}

	if srv.deps.Widgets == nil {
		writeError(w, http.StatusServiceUnavailable, "widgets not configured")

		return
	}

	request := new(homeAssistantRequest)
	if !decodeAction(w, r, request) {
		return
	}

	if request.EntityID == "" {
		writeError(w, http.StatusBadRequest, "entityId is required")

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), actionTimeout)
	defer cancel()

	err := srv.deps.Widgets.ToggleHomeAssistant(ctx, request.EntityID)
	if err != nil {
		slog.WarnContext(r.Context(), "Home Assistant toggle failed",
			slog.String("entity", request.EntityID),
			slog.Any("error", err))
		writeError(w, http.StatusBadGateway, "toggle failed")

		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// sendWakeOnLan builds and broadcasts a magic packet for mac.
func sendWakeOnLan(mac, broadcast string) error {
	hardwareAddr, err := net.ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("parse MAC: %w", err)
	}

	if len(hardwareAddr) != macByteCount {
		return errInvalidMAC
	}

	if broadcast == "" {
		broadcast = "255.255.255.255"
	}

	packet := make([]byte, 0, magicPacketLen)
	for range macByteCount {
		packet = append(packet, wolMagicByte)
	}

	for range magicRepeat {
		packet = append(packet, hardwareAddr...)
	}

	addr := new(net.UDPAddr)
	addr.IP = net.ParseIP(broadcast)
	addr.Port = wolPort

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("dial broadcast: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(packet)
	if err != nil {
		return fmt.Errorf("send magic packet: %w", err)
	}

	return nil
}

// ---------- server-side preferences ----------.

// PreferenceStore persists dashboard preferences as one JSON document so
// they follow the user across browsers. Single-tenant by design.
type PreferenceStore struct {
	path string
	mu   sync.Mutex
}

// NewPreferenceStore creates a store writing to path.
func NewPreferenceStore(path string) *PreferenceStore {
	store := new(PreferenceStore)
	store.path = path

	return store
}

// Read returns the stored preferences document, or nil when none exists.
func (store *PreferenceStore) Read() (json.RawMessage, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	content, err := os.ReadFile(store.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read preferences: %w", err)
	}

	if !json.Valid(content) {
		return nil, nil
	}

	return content, nil
}

// Write atomically replaces the stored preferences document.
func (store *PreferenceStore) Write(content []byte) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	dir := filepath.Dir(store.path)

	tempFile, err := os.CreateTemp(dir, ".prefs-*")
	if err != nil {
		return fmt.Errorf("create temp preferences: %w", err)
	}

	tempPath := tempFile.Name()

	_, err = tempFile.Write(content)
	if err == nil {
		err = tempFile.Chmod(prefsFileMode)
	}

	closeErr := tempFile.Close()
	if err == nil {
		err = closeErr
	}

	if err != nil {
		_ = os.Remove(tempPath)

		return fmt.Errorf("write preferences: %w", err)
	}

	err = os.Rename(tempPath, store.path)
	if err != nil {
		_ = os.Remove(tempPath)

		return fmt.Errorf("replace preferences: %w", err)
	}

	return nil
}

func (srv *Server) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Prefs == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "prefs": nil})

		return
	}

	prefs, err := srv.deps.Prefs.Read()
	if err != nil {
		slog.WarnContext(r.Context(), "Preferences read failed", slog.Any("error", err))
		writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "prefs": nil})

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "prefs": prefs})
}

func (srv *Server) handlePutPreferences(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Prefs == nil {
		writeError(w, http.StatusServiceUnavailable, "preference sync disabled")

		return
	}

	content, err := io.ReadAll(io.LimitReader(r.Body, maxPrefsBytes))
	if err != nil || !json.Valid(content) || len(content) == 0 || content[0] != '{' {
		writeError(w, http.StatusBadRequest, "preferences must be a JSON object")

		return
	}

	err = srv.deps.Prefs.Write(content)
	if err != nil {
		slog.WarnContext(r.Context(), "Preferences write failed", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "could not store preferences")

		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
