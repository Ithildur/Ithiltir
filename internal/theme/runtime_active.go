package theme

import "sync/atomic"

var runtimeActiveID atomic.Value
var runtimeActiveIDSet atomic.Bool

func RuntimeActiveID() (string, bool) {
	if !runtimeActiveIDSet.Load() {
		return "", false
	}
	value := runtimeActiveID.Load()
	id, ok := value.(string)
	if !ok {
		return "", false
	}
	return id, true
}

func SetRuntimeActiveID(id string) {
	normalized, err := NormalizeActiveID(id)
	if err != nil {
		normalized = DefaultID
	}
	runtimeActiveID.Store(normalized)
	runtimeActiveIDSet.Store(true)
}
