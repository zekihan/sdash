package server

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const mediaTimeout = 9 * time.Second

const keyAvailable = "available"

func (srv *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Widgets == nil {
		writeJSON(w, http.StatusOK, map[string]any{keyAvailable: false, "items": []any{}})

		return
	}

	writeJSON(w, http.StatusOK, srv.deps.Widgets.Calendar(r.Context()))
}

func (srv *Server) handleMediaRail(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Widgets == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			keyAvailable: false, "jellyfin": []any{}, "immich": []any{},
		})

		return
	}

	writeJSON(w, http.StatusOK, srv.deps.Widgets.MediaRail(r.Context()))
}

func (srv *Server) handleJellyfinImage(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Widgets == nil {
		http.NotFound(w, r)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), mediaTimeout)
	defer cancel()

	content, contentType, err := srv.deps.Widgets.JellyfinImage(ctx, r.PathValue("id"))
	if err != nil {
		slog.DebugContext(r.Context(), "Jellyfin image failed", slog.Any("error", err))
		http.NotFound(w, r)

		return
	}

	writeImage(w, content, contentType)
}

func (srv *Server) handleImmichThumbnail(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Widgets == nil {
		http.NotFound(w, r)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), mediaTimeout)
	defer cancel()

	content, contentType, err := srv.deps.Widgets.ImmichThumbnail(
		ctx,
		r.PathValue("id"),
		r.URL.Query().Get("size"),
	)
	if err != nil {
		slog.DebugContext(r.Context(), "Immich thumbnail failed", slog.Any("error", err))
		http.NotFound(w, r)

		return
	}

	writeImage(w, content, contentType)
}

func writeImage(w http.ResponseWriter, content []byte, contentType string) {
	// Only known image types are passed through; anything else downloads.
	switch contentType {
	case "image/jpeg", "image/png", "image/webp", "image/gif", "image/avif":
	default:
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.Header().Set("Cache-Control", "private, max-age=3600")

	//nolint:gosec // content type allowlisted above; bytes come from the configured upstream
	_, err := w.Write(
		content,
	)
	if err != nil {
		slog.Debug("Image write failed", slog.Any("error", err))
	}
}

func (srv *Server) handleArrLookup(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Widgets == nil {
		writeJSON(w, http.StatusOK, []any{})

		return
	}

	arrType := r.URL.Query().Get("type")
	term := r.URL.Query().Get("term")

	if term == "" {
		writeJSON(w, http.StatusOK, []any{})

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), mediaTimeout)
	defer cancel()

	results, err := srv.deps.Widgets.Lookup(ctx, arrType, term)
	if err != nil {
		slog.WarnContext(r.Context(), "Arr lookup failed",
			slog.String("type", arrType),
			slog.Any("error", err))
		writeJSON(w, http.StatusOK, []any{})

		return
	}

	writeJSON(w, http.StatusOK, results)
}

type arrAddRequest struct {
	Type       string `json:"type"`
	Title      string `json:"title"`
	ExternalID int    `json:"externalId"`
}

func (srv *Server) handleArrAdd(w http.ResponseWriter, r *http.Request) {
	if !srv.actionsEnabled(w) {
		return
	}

	if srv.deps.Widgets == nil {
		writeError(w, http.StatusServiceUnavailable, "widgets not configured")

		return
	}

	request := new(arrAddRequest)
	if !decodeAction(w, r, request) {
		return
	}

	if request.ExternalID <= 0 {
		writeError(w, http.StatusBadRequest, "externalId is required")

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), mediaTimeout)
	defer cancel()

	err := srv.deps.Widgets.AddTitle(ctx, request.Type, request.ExternalID)
	if err != nil {
		slog.WarnContext(r.Context(), "Arr add failed",
			slog.String("type", request.Type),
			slog.Any("error", err))
		writeError(w, http.StatusBadGateway, "add failed")

		return
	}

	slog.InfoContext(r.Context(), "Title added",
		slog.String("type", request.Type),
		slog.String("title", request.Title))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type magnetRequest struct {
	Magnet string `json:"magnet"`
}

func (srv *Server) handleQBittorrentAdd(w http.ResponseWriter, r *http.Request) {
	if !srv.actionsEnabled(w) {
		return
	}

	if srv.deps.Widgets == nil {
		writeError(w, http.StatusServiceUnavailable, "widgets not configured")

		return
	}

	request := new(magnetRequest)
	if !decodeAction(w, r, request) {
		return
	}

	if request.Magnet == "" {
		writeError(w, http.StatusBadRequest, "magnet is required")

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), mediaTimeout)
	defer cancel()

	err := srv.deps.Widgets.AddMagnet(ctx, request.Magnet)
	if err != nil {
		slog.WarnContext(r.Context(), "qBittorrent add failed", slog.Any("error", err))
		writeError(w, http.StatusBadGateway, "add failed")

		return
	}

	slog.InfoContext(r.Context(), "Torrent added")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
