package frontcache

import (
	"encoding/json"
	"fmt"
	"strings"

	"dash/internal/metrics"
)

type frontSmartRuntime struct {
	ReceivedAt string             `json:"received_at"`
	Smart      *metrics.DiskSmart `json:"smart"`
}

type frontThermalRuntime struct {
	ReceivedAt string           `json:"received_at"`
	Thermal    *metrics.Thermal `json:"thermal"`
}

func frontSmartPayload(node metrics.NodeView) ([]byte, bool, error) {
	if node.Disk.Smart == nil || strings.TrimSpace(node.Observation.ReceivedAt) == "" {
		return nil, false, nil
	}
	runtime := frontSmartRuntime{
		ReceivedAt: node.Observation.ReceivedAt,
		Smart:      node.Disk.Smart,
	}
	raw, err := json.Marshal(runtime)
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

func frontThermalPayload(node metrics.NodeView) ([]byte, bool, error) {
	if node.Thermal == nil || strings.TrimSpace(node.Observation.ReceivedAt) == "" {
		return nil, false, nil
	}
	runtime := frontThermalRuntime{
		ReceivedAt: node.Observation.ReceivedAt,
		Thermal:    node.Thermal,
	}
	raw, err := json.Marshal(runtime)
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

func decodeSmartRuntime(raw []byte) (*frontSmartRuntime, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var runtime frontSmartRuntime
	if err := json.Unmarshal(raw, &runtime); err != nil {
		return nil, err
	}
	if runtime.Smart == nil {
		return nil, fmt.Errorf("%w: smart missing", errCorruptFrontRuntime)
	}
	if strings.TrimSpace(runtime.ReceivedAt) == "" {
		return nil, fmt.Errorf("%w: received_at missing", errCorruptFrontRuntime)
	}
	return &runtime, nil
}

func decodeThermalRuntime(raw []byte) (*frontThermalRuntime, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var runtime frontThermalRuntime
	if err := json.Unmarshal(raw, &runtime); err != nil {
		return nil, err
	}
	if runtime.Thermal == nil {
		return nil, fmt.Errorf("%w: thermal missing", errCorruptFrontRuntime)
	}
	if strings.TrimSpace(runtime.ReceivedAt) == "" {
		return nil, fmt.Errorf("%w: received_at missing", errCorruptFrontRuntime)
	}
	return &runtime, nil
}

func applySmartRuntime(node *metrics.NodeView, runtime *frontSmartRuntime) {
	if runtime == nil {
		return
	}
	if node.Observation.ReceivedAt != runtime.ReceivedAt {
		return
	}
	node.Disk.Smart = runtime.Smart
	node.Disk.TemperatureDevices = metrics.SmartTemperatureDeviceNames(runtime.Smart)
}

func applyThermalRuntime(node *metrics.NodeView, runtime *frontThermalRuntime) {
	if runtime == nil {
		return
	}
	if node.Observation.ReceivedAt != runtime.ReceivedAt {
		return
	}
	node.Thermal = runtime.Thermal
}

func applyFrontRuntime(node *metrics.NodeView, smartRaw, thermalRaw []byte) error {
	smartRuntime, err := decodeSmartRuntime(smartRaw)
	if err != nil {
		return err
	}
	thermalRuntime, err := decodeThermalRuntime(thermalRaw)
	if err != nil {
		return err
	}
	applySmartRuntime(node, smartRuntime)
	applyThermalRuntime(node, thermalRuntime)
	return nil
}

func keepFrontRuntime(runtime map[string][]byte, nodes map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(nodes))
	for id := range nodes {
		if raw, ok := runtime[id]; ok {
			out[id] = raw
		}
	}
	return out
}

func corruptFrontRuntime(err error) error {
	return fmt.Errorf("%w: %w", errCorruptFrontSnapshot, err)
}
