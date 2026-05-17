package themes

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	themefs "dash/internal/theme"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

const maxThemeArchiveBytes int64 = themefs.ArchiveMaxBytes

func uploadRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/upload",
		"Upload theme package",
		routes.Func(h.uploadHandler),
	)
}

func (h *handler) uploadHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxThemeArchiveBytes)
	if err := r.ParseMultipartForm(maxThemeArchiveBytes); err != nil {
		writeInvalidPackage(w, fmt.Errorf("theme package must be a zip file up to %d MiB", maxThemeArchiveBytes>>20))
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeInvalidPackage(w, errors.New("file is required"))
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		writeInvalidPackage(w, errors.New("failed to read uploaded file"))
		return
	}

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

	active := false
	deletable := true
	if state, err := h.loadActiveThemeState(r.Context()); err == nil {
		active = pkg.Manifest.ID == state.ID
		deletable = !active
	} else {
		h.logger.Warn("failed to resolve active theme after upload", err, slog.String("theme_id", pkg.Manifest.ID))
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
