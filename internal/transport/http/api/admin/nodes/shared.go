package nodes

import (
	"dash/internal/nodetags"

	"gorm.io/datatypes"
)

func normalizeSubmittedTags(raw []byte) (datatypes.JSON, error) {
	encoded, err := nodetags.NormalizeJSON(raw)
	if err != nil {
		return nil, errInvalidNodeTags
	}
	return datatypes.JSON(encoded), nil
}

func parseNodeTags(raw []byte) ([]string, error) {
	tags, err := nodetags.Parse(raw)
	if err != nil {
		return nil, errInvalidNodeTags
	}
	if tags == nil {
		return []string{}, nil
	}
	return tags, nil
}
