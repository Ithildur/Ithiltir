package node

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	return false
}
