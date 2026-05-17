package httperr

import (
	"net/http"
	"strings"
)

const HeaderWarning = "X-Dash-Warning"

func WriteWarningHeader(w http.ResponseWriter, code string) {
	if w == nil {
		return
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return
	}

	w.Header().Set(HeaderWarning, code)
}
