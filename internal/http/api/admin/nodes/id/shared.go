package nodeid

import (
	"dash/internal/nodetags"

	"gorm.io/datatypes"
)

func normalizeSubmittedTags(raw []byte) (datatypes.JSON, error) {
	encoded, err := nodetags.NormalizeJSON(raw)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(encoded), nil
}
