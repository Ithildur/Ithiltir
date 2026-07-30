package frontcache

import (
	"encoding/json"
	"strings"

	"dash/internal/metrics"
	"dash/internal/model"
)

// frontNodeMeta contains only PostgreSQL-owned fields used by NodeView.
// Runtime samples never overwrite this payload.
type frontNodeMeta struct {
	Node             metrics.NodeMeta `json:"node"`
	System           metrics.System   `json:"system"`
	CPU              metrics.CPU      `json:"cpu"`
	MemoryTotalBytes int64            `json:"memory_total_bytes"`
	SwapTotalBytes   int64            `json:"swap_total_bytes"`
	RootPath         string           `json:"root_path,omitempty"`
	RootFSType       string           `json:"root_fs_type,omitempty"`
}

type frontNodeProjection struct {
	Node        metrics.NodeView
	Meta        frontNodeMeta
	MemoryTotal int64
	SwapTotal   int64
}

func frontNodeMetaFromServer(server model.Server) frontNodeMeta {
	static := metrics.BuildNodeView(server, metrics.NodeReport{}, 0)
	static.System.UptimeText = ""
	static.CPU.UsageRatio = 0
	static.CPU.Load = metrics.CPULoad{}
	return frontNodeMeta{
		Node:             static.Node,
		System:           static.System,
		CPU:              static.CPU,
		MemoryTotalBytes: static.Memory.TotalBytes,
		SwapTotalBytes:   static.Memory.SwapTotalBytes,
		RootPath:         strings.TrimSpace(stringValue(server.RootPath)),
		RootFSType:       strings.TrimSpace(stringValue(server.RootFSType)),
	}
}

func frontNodeMetaFromView(node metrics.NodeView) frontNodeMeta {
	return frontNodeMeta{
		Node:             node.Node,
		System:           metrics.System{OSFamily: node.System.OSFamily, Platform: node.System.Platform, PlatformVersion: node.System.PlatformVersion, KernelVersion: node.System.KernelVersion, Arch: node.System.Arch},
		CPU:              metrics.CPU{ModelName: node.CPU.ModelName, CoresPhysical: node.CPU.CoresPhysical, CoresLogical: node.CPU.CoresLogical, Sockets: node.CPU.Sockets},
		MemoryTotalBytes: node.Memory.TotalBytes,
		SwapTotalBytes:   node.Memory.SwapTotalBytes,
	}
}

func frontNodeProjectionFromView(node metrics.NodeView) frontNodeProjection {
	return frontNodeProjection{
		Node:        node,
		Meta:        frontNodeMetaFromView(node),
		MemoryTotal: node.Memory.TotalBytes,
		SwapTotal:   node.Memory.SwapTotalBytes,
	}
}

func frontRuntimePayload(projection frontNodeProjection) (string, []byte, error) {
	node := projection.Node
	id, err := normalizeFrontNodeID(node.Node.ID)
	if err != nil {
		return "", nil, err
	}
	node.Node = metrics.NodeMeta{ID: id}
	node.System.OSFamily = ""
	node.System.Platform = ""
	node.System.PlatformVersion = ""
	node.System.KernelVersion = ""
	node.System.Arch = ""
	node.CPU.ModelName = ""
	node.CPU.CoresPhysical = 0
	node.CPU.CoresLogical = 0
	node.CPU.Sockets = 0
	node.Memory.TotalBytes = projection.MemoryTotal
	node.Memory.SwapTotalBytes = projection.SwapTotal
	node.Disk.Smart = nil
	node.Disk.TemperatureDevices = nil
	node.Thermal = nil
	raw, err := json.Marshal(node)
	if err != nil {
		return "", nil, err
	}
	return id, raw, nil
}

func frontMetadataPayload(meta frontNodeMeta) (string, []byte, error) {
	id, err := normalizeFrontNodeID(meta.Node.ID)
	if err != nil {
		return "", nil, err
	}
	meta.Node.ID = id
	raw, err := json.Marshal(meta)
	if err != nil {
		return "", nil, err
	}
	return id, raw, nil
}

func decodeFrontRuntime(raw []byte, wantID string) (metrics.NodeView, error) {
	var node metrics.NodeView
	if err := json.Unmarshal(raw, &node); err != nil {
		return node, err
	}
	id, err := normalizeFrontNodeID(node.Node.ID)
	if err != nil || id != wantID {
		return node, errCorruptFrontSnapshot
	}
	node.Node.ID = id
	return node, nil
}

func decodeFrontMetadata(raw []byte, wantID string) (frontNodeMeta, error) {
	var meta frontNodeMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return meta, err
	}
	id, err := normalizeFrontNodeID(meta.Node.ID)
	if err != nil || id != wantID {
		return meta, errCorruptFrontSnapshot
	}
	meta.Node.ID = id
	return meta, nil
}

func composeFrontNode(runtime metrics.NodeView, meta frontNodeMeta) metrics.NodeView {
	runtime.Node = meta.Node
	runtime.System.OSFamily = meta.System.OSFamily
	runtime.System.Platform = meta.System.Platform
	runtime.System.PlatformVersion = meta.System.PlatformVersion
	runtime.System.KernelVersion = meta.System.KernelVersion
	runtime.System.Arch = meta.System.Arch
	runtime.CPU.ModelName = meta.CPU.ModelName
	runtime.CPU.CoresPhysical = meta.CPU.CoresPhysical
	runtime.CPU.CoresLogical = meta.CPU.CoresLogical
	runtime.CPU.Sockets = meta.CPU.Sockets
	if runtime.Memory.TotalBytes == 0 {
		runtime.Memory.TotalBytes = meta.MemoryTotalBytes
	}
	if runtime.Memory.SwapTotalBytes == 0 {
		runtime.Memory.SwapTotalBytes = meta.SwapTotalBytes
	}
	if meta.RootPath != "" && meta.RootFSType != "" {
		for i := range runtime.Disk.Mounts {
			if strings.TrimSpace(runtime.Disk.Mounts[i].Mountpoint) == meta.RootPath && strings.TrimSpace(runtime.Disk.Mounts[i].FSType) == "" {
				runtime.Disk.Mounts[i].FSType = meta.RootFSType
			}
		}
	}
	return runtime
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
