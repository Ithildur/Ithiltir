package metrics

import "strings"

var (
	smartVirtualDiskPrefixes = [...]string{"md", "dm-", "vd", "xvd"}
	smartVirtualDiskTerms    = [...]string{"raid:", ":md", "/dev/md", "/dev/dm-", "/dev/mapper", "/dev/vd", "/dev/xvd"}
	smartVirtualTerms        = [...]string{"raid", "virtual", "virtio", "qemu", "vmware", "xen", "vbox", "hyper-v"}
	smartPhysicalTerms       = [...]string{"nvme", "ata", "sata", "sat"}
)

// SmartTemperatureDeviceNames returns SMART device names eligible for disk temperature history.
func SmartTemperatureDeviceNames(smart *DiskSmart) []string {
	if smart == nil || len(smart.Devices) == 0 {
		return nil
	}
	devices := make([]string, 0, len(smart.Devices))
	seen := make(map[string]struct{}, len(smart.Devices))
	for _, device := range smart.Devices {
		name := strings.TrimSpace(device.Name)
		if name == "" || !IsSmartTemperatureDevice(device) {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		devices = append(devices, name)
	}
	if len(devices) == 0 {
		return nil
	}
	return devices
}

// IsSmartTemperatureDevice reports whether a SMART device should be stored in disk temperature history.
func IsSmartTemperatureDevice(device DiskSmartDevice) bool {
	if !validTempC(device.TempC) {
		return false
	}
	return isPhysicalSmartDevice(device)
}

func isPhysicalSmartDevice(device DiskSmartDevice) bool {
	name := strings.ToLower(strings.TrimSpace(device.Name))
	path := strings.ToLower(strings.TrimSpace(device.DevicePath))
	ref := strings.ToLower(strings.TrimSpace(device.Ref))
	if isVirtualDiskID(name) || isVirtualDiskID(path) || isVirtualDiskID(ref) {
		return false
	}

	text := strings.ToLower(strings.Join([]string{
		device.DeviceType,
		device.Protocol,
		device.Model,
	}, " "))
	if hasSmartTerm(text, smartVirtualTerms[:]) {
		return false
	}

	kind := strings.ToLower(device.Protocol + " " + device.DeviceType)
	if hasSmartTerm(kind, smartPhysicalTerms[:]) {
		return true
	}
	if strings.Contains(kind, "scsi") {
		return strings.TrimSpace(device.Serial) != "" || strings.TrimSpace(device.WWN) != ""
	}
	return false
}

func isVirtualDiskID(value string) bool {
	for _, prefix := range smartVirtualDiskPrefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return hasSmartTerm(value, smartVirtualDiskTerms[:])
}

func hasSmartTerm(value string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}
