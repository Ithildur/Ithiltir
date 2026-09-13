package nodes

import (
	"dash/internal/nodetags"
)

func parseNodeTags(raw []byte) []string {
	tags, _ := nodetags.ParseStored(raw)
	if tags == nil {
		return []string{}
	}
	return tags
}
