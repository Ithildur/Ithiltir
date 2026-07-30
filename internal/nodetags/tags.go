package nodetags

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrInvalid = errors.New("tags must be a string array")

const (
	maxTags     = 32
	maxTagRunes = 64
)

func Parse(raw []byte) ([]string, error) {
	tags, err := decode(raw)
	if err != nil {
		return nil, err
	}
	if len(tags) > maxTags {
		return nil, fmt.Errorf("%w with at most %d items", ErrInvalid, maxTags)
	}
	for _, tag := range tags {
		if utf8.RuneCountInString(tag) > maxTagRunes || hasControl(tag) {
			return nil, fmt.Errorf("%w; each tag must contain at most %d characters and no control characters", ErrInvalid, maxTagRunes)
		}
	}
	return tags, nil
}

// ParseStored returns the valid subset of stored tags and a warning error when
// malformed or out-of-contract values were discarded.
func ParseStored(raw []byte) ([]string, error) {
	tags, err := decode(raw)
	if err != nil {
		return nil, err
	}
	valid := make([]string, 0, min(len(tags), maxTags))
	discarded := 0
	for _, tag := range tags {
		if utf8.RuneCountInString(tag) > maxTagRunes || hasControl(tag) || len(valid) >= maxTags {
			discarded++
			continue
		}
		valid = append(valid, tag)
	}
	if discarded > 0 {
		return valid, fmt.Errorf("%w; discarded %d stored tags outside current limits", ErrInvalid, discarded)
	}
	return valid, nil
}

func decode(raw []byte) ([]string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var tags []string
	if err := json.Unmarshal(raw, &tags); err != nil {
		return nil, ErrInvalid
	}
	return Clean(tags), nil
}

func hasControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func NormalizeJSON(raw []byte) ([]byte, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, ErrInvalid
	}
	tags, err := Parse(raw)
	if err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}
	return json.Marshal(tags)
}

func Clean(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
