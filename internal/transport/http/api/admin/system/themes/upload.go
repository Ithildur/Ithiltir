package themes

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"dash/internal/config"
	themefs "dash/internal/theme"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

const maxThemeArchiveBytes int64 = themefs.ArchiveMaxBytes
const maxThemeMultipartBytes int64 = maxThemeArchiveBytes + 1<<20

func uploadRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/upload",
		"Upload theme package",
		routes.Func(h.uploadHandler),
	)
}

func (h *handler) uploadHandler(w http.ResponseWriter, r *http.Request) {
	deadline := time.Now().Add(config.ThemeUploadTimeout)
	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.logger.Warn("set theme upload read deadline failed", err)
		httperr.Write(w, http.StatusInternalServerError, "theme_storage_unavailable", "failed to initialize theme upload")
		return
	}
	if err := controller.SetWriteDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.logger.Warn("set theme upload write deadline failed", err)
		httperr.Write(w, http.StatusInternalServerError, "theme_storage_unavailable", "failed to initialize theme upload")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxThemeMultipartBytes)
	if err := r.ParseMultipartForm(maxThemeArchiveBytes); err != nil {
		writeInvalidPackage(w, fmt.Errorf("theme package must be a zip file up to %d MiB", maxThemeArchiveBytes>>20))
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, _, err := r.FormFile("file")
	if err != nil {
		writeInvalidPackage(w, errors.New("file is required"))
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, maxThemeArchiveBytes+1))
	if err != nil {
		writeInvalidPackage(w, errors.New("failed to read uploaded file"))
		return
	}
	if int64(len(raw)) > maxThemeArchiveBytes {
		writeInvalidPackage(w, fmt.Errorf("theme package must be a zip file up to %d MiB", maxThemeArchiveBytes>>20))
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	pkg, err := h.themes.InstallZip(raw)
	if err != nil {
		if errors.Is(err, themefs.ErrInvalidPackage) {
			writeInvalidPackage(w, err)
			return
		}
		h.logger.Warn("failed to install theme package", err)
		httperr.Write(w, http.StatusInternalServerError, "theme_storage_unavailable", "failed to install theme")
		return
	}
	if pkg.CleanupWarning != nil {
		h.logger.Warn(
			"theme installed but previous backup cleanup failed",
			pkg.CleanupWarning,
			slog.String("theme_id", pkg.Manifest.ID),
		)
	}

	active := false
	deletable := true
	if state, err := h.loadActiveThemeState(r.Context()); err == nil {
		selected := pkg.Manifest.ID == state.ConfiguredID
		active = selected && state.State == themefs.ActiveReady
		deletable = !selected
	} else {
		h.logger.Warn(
			"failed to resolve active theme after upload",
			err,
			slog.String("theme_id", pkg.Manifest.ID),
		)
	}
	now := time.Now()

	response.WriteJSON(w, http.StatusCreated, themeView(
		pkg.Manifest,
		false,
		active,
		deletable,
		&now,
		&now,
		pkg.HasPreview,
	))
}
