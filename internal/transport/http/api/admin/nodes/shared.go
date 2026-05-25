package nodes

import (
	"dash/internal/nodetags"
)

func parseNodeTags(raw []byte) ([]string, error) {
	tags, err := nodetags.Parse(raw)
	if err != nil {
		return nil, err
	}
	if tags == nil {
		return []string{}, nil
	}
	return tags, nil
}
