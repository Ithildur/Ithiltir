package request

import (
	"net/http"
	"strings"

	"github.com/Ithildur/EiluneKit/auth"
)

func HasValidBearer(r *http.Request, auth auth.AccessTokenValidator) bool {
	if auth == nil || r == nil {
		return false
	}

	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		return false
	}

	_, ok, err := auth.ValidateAccessToken(r.Context(), token)
	return err == nil && ok
}

func bearerToken(header string) (string, bool) {
	header = strings.TrimSpace(header)
	if header == "" || !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" {
		return "", false
	}
	return token, true
}
