package nodetags

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

var ErrInvalid = errors.New("tags must be a string array")

func Parse(raw []byte) ([]string, error) {
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
