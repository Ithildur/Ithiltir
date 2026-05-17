package request

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func ParseIDInt64(r *http.Request, name string) (int64, error) {
	raw := chi.URLParam(r, name)
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	if val <= 0 {
		return 0, errors.New("id must be positive")
	}
	return val, nil
}
