package request

import (
	"errors"
	"net/http"

	"github.com/Ithildur/EiluneKit/http/decoder"

	"dash/internal/transport/http/httperr"
)

func DecodeJSONOrWriteError(w http.ResponseWriter, r *http.Request, out interface{}) bool {
	err := decoder.DecodeJSONBody(r, out)
	if err != nil {
		if errors.Is(err, decoder.ErrBodyTooLarge) {
			httperr.TryWrite(w, httperr.BodyTooLarge(err))
			return false
		}
		httperr.TryWrite(w, httperr.InvalidRequest(err))
		return false
	}
	return true
}
