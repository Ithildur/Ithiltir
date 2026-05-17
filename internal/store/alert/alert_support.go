package alert

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/datatypes"
)

func decodeChannelIDs(raw []byte) ([]int64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var items []int64
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(items))
	seen := make(map[int64]struct{}, len(items))
	for _, id := range items {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func marshalJSON(v any) (datatypes.JSON, error) {
	if v == nil {
		return datatypes.JSON([]byte(`{}`)), nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(raw), nil
}

func emptyToNil(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func isUniqueConstraintError(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && (constraint == "" || pgErr.ConstraintName == constraint)
}
