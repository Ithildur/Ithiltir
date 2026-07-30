package groups

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxGroupNameRunes   = 64
	maxGroupRemarkRunes = 255
)

func groupName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", errors.New("name is required")
	}
	if utf8.RuneCountInString(name) > maxGroupNameRunes || hasControlRune(name) {
		return "", errors.New("name must contain at most 64 characters and no control characters")
	}
	return name, nil
}

func groupRemark(raw string) (string, error) {
	remark := strings.TrimSpace(raw)
	if utf8.RuneCountInString(remark) > maxGroupRemarkRunes || hasControlRune(remark) {
		return "", errors.New("remark must contain at most 255 characters and no control characters")
	}
	return remark, nil
}

func hasControlRune(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
