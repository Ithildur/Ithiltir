package metrics

import (
	"errors"
	"fmt"
	"strings"
)

// ValidateReport checks the semantic shape of a node metrics push body.
func ValidateReport(report NodeReport) error {
	m := report.Metrics

	if m.Disk.Physical == nil {
		return errors.New("missing disk physical")
	}
	if m.Disk.Logical == nil {
		return errors.New("missing disk logical")
	}
	if m.Disk.Filesystems == nil {
		return errors.New("missing disk filesystems")
	}
	if m.Disk.BaseIO == nil {
		return errors.New("missing disk base_io")
	}
	if m.Network == nil {
		return errors.New("missing network")
	}
	if m.Raid.Arrays == nil {
		return errors.New("missing raid arrays")
	}

	if strings.TrimSpace(m.System.Uptime) == "" {
		return errors.New("missing system uptime")
	}

	if err := validateDiskPhysical(m.Disk.Physical); err != nil {
		return err
	}
	if err := validateDiskLogical(m.Disk.Logical); err != nil {
		return err
	}
	if err := validateDiskFilesystems(m.Disk.Filesystems); err != nil {
		return err
	}
	if err := validateDiskBaseIO(m.Disk.BaseIO); err != nil {
		return err
	}
	if err := validateNetwork(m.Network); err != nil {
		return err
	}
	if err := validateRaidArrays(m.Raid.Arrays); err != nil {
		return err
	}
	if m.Disk.Smart != nil {
		if err := validateDiskSmart(*m.Disk.Smart); err != nil {
			return err
		}
	}
	if m.Thermal != nil {
		if err := validateThermal(*m.Thermal); err != nil {
			return err
		}
	}

	return nil
}

func validateDiskPhysical(items []DiskPhysicalMetrics) error {
	seenPhysical := make(map[string]struct{}, len(items))
	for i, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("missing disk physical[%d].name", i)
		}
		name := strings.TrimSpace(item.Name)
		if _, ok := seenPhysical[name]; ok {
			return fmt.Errorf("duplicate disk physical name: %s", name)
		}
		seenPhysical[name] = struct{}{}
	}
	return nil
}

func validateDiskLogical(items []DiskLogicalMetrics) error {
	seenLogical := make(map[string]struct{}, len(items))
	for i, item := range items {
		if strings.TrimSpace(item.Kind) == "" {
			return fmt.Errorf("missing disk logical[%d].kind", i)
		}
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("missing disk logical[%d].name", i)
		}
		name := strings.TrimSpace(item.Name)
		if _, ok := seenLogical[name]; ok {
			return fmt.Errorf("duplicate disk logical name: %s", name)
		}
		seenLogical[name] = struct{}{}
	}
	return nil
}

func validateDiskFilesystems(items []DiskFilesystemMetrics) error {
	for i, item := range items {
		if strings.TrimSpace(item.Path) == "" {
			return fmt.Errorf("missing disk filesystems[%d].path", i)
		}
	}
	return nil
}

func validateDiskBaseIO(items []DiskBaseIOMetrics) error {
	seenBaseIO := make(map[string]struct{}, len(items))
	for i, item := range items {
		if strings.TrimSpace(item.Kind) == "" {
			return fmt.Errorf("missing disk base_io[%d].kind", i)
		}
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("missing disk base_io[%d].name", i)
		}
		name := strings.TrimSpace(item.Name)
		if _, ok := seenBaseIO[name]; ok {
			return fmt.Errorf("duplicate disk base_io name: %s", name)
		}
		seenBaseIO[name] = struct{}{}
	}
	return nil
}

func validateDiskSmart(item DiskSmart) error {
	if strings.TrimSpace(item.Status) == "" {
		return errors.New("missing disk smart status")
	}
	if item.Devices == nil {
		return errors.New("missing disk smart devices")
	}
	for i, device := range item.Devices {
		if strings.TrimSpace(device.Name) == "" {
			return fmt.Errorf("missing disk smart devices[%d].name", i)
		}
		if strings.TrimSpace(device.Source) == "" {
			return fmt.Errorf("missing disk smart devices[%d].source", i)
		}
		if strings.TrimSpace(device.Status) == "" {
			return fmt.Errorf("missing disk smart devices[%d].status", i)
		}
	}
	return nil
}

func validateNetwork(items []NetIOMetrics) error {
	seenNetwork := make(map[string]struct{}, len(items))
	for i, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("missing network[%d].name", i)
		}
		name := strings.TrimSpace(item.Name)
		if _, ok := seenNetwork[name]; ok {
			return fmt.Errorf("duplicate network name: %s", name)
		}
		seenNetwork[name] = struct{}{}
	}
	return nil
}

func validateThermal(item Thermal) error {
	if strings.TrimSpace(item.Status) == "" {
		return errors.New("missing thermal status")
	}
	if item.Sensors == nil {
		return errors.New("missing thermal sensors")
	}
	for i, sensor := range item.Sensors {
		if strings.TrimSpace(sensor.Kind) == "" {
			return fmt.Errorf("missing thermal sensors[%d].kind", i)
		}
		if strings.TrimSpace(sensor.Name) == "" {
			return fmt.Errorf("missing thermal sensors[%d].name", i)
		}
		if strings.TrimSpace(sensor.SensorKey) == "" {
			return fmt.Errorf("missing thermal sensors[%d].sensor_key", i)
		}
		if strings.TrimSpace(sensor.Source) == "" {
			return fmt.Errorf("missing thermal sensors[%d].source", i)
		}
		if strings.TrimSpace(sensor.Status) == "" {
			return fmt.Errorf("missing thermal sensors[%d].status", i)
		}
	}
	return nil
}

func validateRaidArrays(items []RaidArray) error {
	for i, arr := range items {
		if strings.TrimSpace(arr.Name) == "" {
			return fmt.Errorf("missing raid arrays[%d].name", i)
		}
		if strings.TrimSpace(arr.Status) == "" {
			return fmt.Errorf("missing raid arrays[%d].status", i)
		}
		if strings.TrimSpace(arr.Health) == "" {
			return fmt.Errorf("missing raid arrays[%d].health", i)
		}
		if arr.Members == nil {
			return fmt.Errorf("missing raid arrays[%d].members", i)
		}
		for j, member := range arr.Members {
			if strings.TrimSpace(member.Name) == "" {
				return fmt.Errorf("missing raid arrays[%d].members[%d].name", i, j)
			}
			if strings.TrimSpace(member.State) == "" {
				return fmt.Errorf("missing raid arrays[%d].members[%d].state", i, j)
			}
		}
	}
	return nil
}
