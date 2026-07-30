package request

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Ithildur/EiluneKit/auth"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	"github.com/Ithildur/EiluneKit/http/routes"
)

var errAccessTokenValidatorMissing = errors.New("access token validator is required")

// OptionalBearer recognizes a valid admin token and treats every other request as a guest.
// Invalid credentials intentionally do not turn a public endpoint into an authentication endpoint.
func OptionalBearer(validator auth.AccessTokenValidator) (routes.Middleware, error) {
	if validator == nil {
		return nil, errAccessTokenValidatorMissing
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			claims, valid, err := validator.ValidateAccessToken(r.Context(), token)
			if err != nil || !valid {
				next.ServeHTTP(w, r)
				return
			}

			ctx := authjwt.WithClaims(r.Context(), claims)
			ctx = routes.WithAuthenticated(ctx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

func bearerToken(header string) (string, bool) {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	return token, token != ""
}
