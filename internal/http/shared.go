package httpserver

import (
	"net/http"
	"strings"

	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

const maxAPIRequestBytes int64 = 24 << 20

func apiBoundary(next http.Handler) http.Handler {
	limited := middleware.LimitBody(maxAPIRequestBytes)(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) {
			w.Header().Set("Cache-Control", "no-store")
			limited.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func methodBoundary(routeList []routes.Route) routes.Middleware {
	// File and SPA fallbacks implement GET and HEAD without registered endpoints.
	methods := map[string]bool{http.MethodGet: true, http.MethodHead: true}
	for _, route := range routeList {
		methods[strings.ToUpper(strings.TrimSpace(route.Method))] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if methods[r.Method] {
				next.ServeHTTP(w, r)
				return
			}
			if isAPIPath(r.URL.Path) {
				response.WriteJSONError(w, http.StatusNotImplemented, "not_implemented", "method not implemented")
				return
			}
			http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
		})
	}
}

type fallback struct {
	page   http.Handler
	deploy http.Handler
}

func (f fallback) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isAPIPath(r.URL.Path) {
		response.NotFound().ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/deploy/") {
		f.deploy.ServeHTTP(w, r)
		return
	}
	f.page.ServeHTTP(w, r)
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	if isAPIPath(r.URL.Path) {
		response.MethodNotAllowed().ServeHTTP(w, r)
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}
