package request

import (
	"net/http"

	"github.com/Ithildur/EiluneKit/http/decoder"

	"dash/internal/transport/http/httperr"
)

func DecodeJSONOrWriteError(w http.ResponseWriter, r *http.Request, out interface{}) bool {
	if err := decoder.DecodeJSONBody(r, out); err != nil {
		httperr.TryWrite(w, httperr.InvalidRequest(err))
		return false
	}
	return true
}
