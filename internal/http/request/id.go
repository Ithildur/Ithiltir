package request

import (
	"errors"
	"strconv"
)

func ParseIDInt64(raw string) (int64, error) {
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	if val <= 0 {
		return 0, errors.New("id must be positive")
	}
	return val, nil
}
