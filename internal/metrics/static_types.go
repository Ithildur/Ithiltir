package metrics

import "time"

type StaticCPUInfo struct {
	ModelName     string  `json:"model_name"`
	VendorID      string  `json:"vendor_id"`
	Sockets       int     `json:"sockets"`
	CoresPhysical int     `json:"cores_physical"`
	CoresLogical  int     `json:"cores_logical"`
	FrequencyMhz  float64 `json:"frequency_mhz"`
}

type StaticCPU struct {
	Info StaticCPUInfo `json:"info"`
}

type StaticMemory struct {
	Total     uint64 `json:"total"`
	SwapTotal uint64 `json:"swap_total"`
}

type StaticDiskPhysical struct {
	Name       string `json:"name"`
	DevicePath string `json:"device_path,omitempty"`
	Ref        string `json:"ref,omitempty"`
}

type StaticDiskLogical struct {
	Kind        string                          `json:"kind"`
	Name        string                          `json:"name"`
	DevicePath  string                          `json:"device_path,omitempty"`
	Ref         string                          `json:"ref,omitempty"`
	Total       uint64                          `json:"total"`
	Used        uint64                          `json:"used,omitempty"`
	Free        uint64                          `json:"free,omitempty"`
	UsedRatio   float64                         `json:"used_ratio,omitempty"`
	Health      string                          `json:"health,omitempty"`
	Level       string                          `json:"level,omitempty"`
	Mountpoint  string                          `json:"mountpoint,omitempty"`
	Mountpoints map[string]StaticDiskMountpoint `json:"mountpoints,omitempty"`
	Devices     []string                        `json:"devices,omitempty"`
}

type StaticDiskFilesystem struct {
	Path        string `json:"path"`
	Device      string `json:"device,omitempty"`
	Mountpoint  string `json:"mountpoint,omitempty"`
	Total       uint64 `json:"total"`
	FsType      string `json:"fs_type"`
	InodesTotal uint64 `json:"inodes_total"`
}

type StaticDiskBaseIO struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	DevicePath string `json:"device_path,omitempty"`
	Ref        string `json:"ref,omitempty"`
	Role       string `json:"role,omitempty"`
}

type StaticDisk struct {
	Physical    []StaticDiskPhysical   `json:"physical,omitempty"`
	Logical     []StaticDiskLogical    `json:"logical,omitempty"`
	Filesystems []StaticDiskFilesystem `json:"filesystems,omitempty"`
	BaseIO      []StaticDiskBaseIO     `json:"base_io,omitempty"`
}

type StaticDiskMountpoint struct {
	FSType          string  `json:"fs_type,omitempty"`
	InodesTotal     uint64  `json:"inodes_total,omitempty"`
	InodesUsed      uint64  `json:"inodes_used,omitempty"`
	InodesFree      uint64  `json:"inodes_free,omitempty"`
	InodesUsedRatio float64 `json:"inodes_used_ratio,omitempty"`
}

type StaticSystem struct {
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	KernelVersion   string `json:"kernel_version"`
	Arch            string `json:"arch"`
}

type StaticRaidMember struct {
	Name string `json:"name"`
}

type StaticRaidArray struct {
	Name    string             `json:"name"`
	Level   string             `json:"level"`
	Devices int                `json:"devices"`
	Members []StaticRaidMember `json:"members,omitempty"`
}

type StaticRaid struct {
	Supported bool              `json:"supported"`
	Available bool              `json:"available"`
	Arrays    []StaticRaidArray `json:"arrays,omitempty"`
}

type StaticMetrics struct {
	Version               string       `json:"version"`
	Timestamp             time.Time    `json:"timestamp"`
	ReportIntervalSeconds int          `json:"report_interval_seconds"`
	CPU                   StaticCPU    `json:"cpu"`
	Memory                StaticMemory `json:"memory"`
	Disk                  StaticDisk   `json:"disk"`
	System                StaticSystem `json:"system"`
	Raid                  StaticRaid   `json:"raid"`
}
