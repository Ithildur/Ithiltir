package api

import "net/http"

const maxAPIRequestBytes int64 = 24 << 20

func apiBoundary(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxAPIRequestBytes)
		}
		next.ServeHTTP(w, r)
	})
}
