package auth

import (
	"errors"

	authhttp "github.com/Ithildur/EiluneKit/auth/http"
)

// NewHandler builds the auth HTTP handler.
func NewHandler(auth authhttp.TokenManager, opts authhttp.Options) (*authhttp.Handler, error) {
	if auth == nil {
		return nil, errors.New("auth: nil token manager")
	}
	return authhttp.NewHandler(auth, opts)
}
