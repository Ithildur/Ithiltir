// Package virt defines the versioned cache and reporting contract for VM observations.
package virt

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"time"
)

const MaxBytes = 4 << 20

type Snapshot struct {
	Schema        int        `json:"schema"`
	Provider      string     `json:"provider"`
	Host          string     `json:"host"`
	CollectedAt   time.Time  `json:"collected_at"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	TTLSeconds    int        `json:"ttl_seconds"`
	Status        string     `json:"status"`
	Error         string     `json:"error,omitempty"`
	VMs           []VM       `json:"vms"`
}

type VM struct {
	IPsCollectedAt *time.Time `json:"ips_collected_at,omitempty"`
	IPsTTLSeconds  int        `json:"ips_ttl_seconds,omitempty"`
	IPs            []string   `json:"ips,omitempty"`
	ID             int        `json:"id"`
	Name           string     `json:"name"`
	Status         string     `json:"status"`
	QMPStatus      string     `json:"qmp_status,omitempty"`
	Template       bool       `json:"template"`
	CPUs           *int64     `json:"cpus,omitempty"`
	CPU            *float64   `json:"cpu_ratio,omitempty"`
	Memory         *int64     `json:"memory_bytes,omitempty"`
	MemoryLimit    *int64     `json:"memory_limit_bytes,omitempty"`
	DiskCapacity   *int64     `json:"disk_capacity_bytes,omitempty"`
	DiskRead       *int64     `json:"disk_read_bytes,omitempty"`
	DiskWrite      *int64     `json:"disk_write_bytes,omitempty"`
	NetIn          *int64     `json:"net_in_bytes,omitempty"`
	NetOut         *int64     `json:"net_out_bytes,omitempty"`
	Uptime         *int64     `json:"uptime_seconds,omitempty"`
}

func (s Snapshot) Validate() error {
	if s.Schema != 1 || s.Provider != "pve" || len(s.Host) > 255 || s.CollectedAt.IsZero() || s.TTLSeconds < 1 || s.TTLSeconds > 3600 {
		return errors.New("invalid virtualization snapshot metadata")
	}
	if s.Status != "ok" && s.Status != "error" {
		return errors.New("invalid virtualization status")
	}
	if len(s.Error) > 1024 || len(s.VMs) > 4096 {
		return errors.New("virtualization snapshot exceeds limits")
	}
	if s.Status == "ok" && (s.Host == "" || s.LastSuccessAt == nil || !s.LastSuccessAt.Equal(s.CollectedAt) || s.Error != "" || s.VMs == nil) {
		return errors.New("invalid successful virtualization snapshot")
	}
	if s.Status == "error" && s.Error == "" || len(s.VMs) > 0 && s.LastSuccessAt == nil {
		return errors.New("invalid failed virtualization snapshot")
	}
	if s.LastSuccessAt != nil && (s.LastSuccessAt.IsZero() || s.LastSuccessAt.After(s.CollectedAt)) {
		return errors.New("invalid virtualization success time")
	}
	seen := make(map[int]bool, len(s.VMs))
	for _, vm := range s.VMs {
		if vm.ID < 1 || seen[vm.ID] || len(vm.Name) > 255 || vm.Status == "" || len(vm.Status) > 64 || len(vm.QMPStatus) > 64 {
			return fmt.Errorf("invalid VM %d", vm.ID)
		}
		seen[vm.ID] = true
		if vm.IPsCollectedAt == nil && vm.IPsTTLSeconds != 0 || vm.IPsCollectedAt != nil && (vm.IPsCollectedAt.IsZero() || vm.IPsCollectedAt.After(s.CollectedAt) || vm.IPsTTLSeconds < 1 || vm.IPsTTLSeconds > 3600) {
			return fmt.Errorf("invalid VM %d IP freshness", vm.ID)
		}
		if len(vm.IPs) > 128 {
			return fmt.Errorf("too many VM %d IPs", vm.ID)
		}
		addresses := make(map[netip.Addr]bool, len(vm.IPs))
		for _, raw := range vm.IPs {
			ip, err := netip.ParseAddr(raw)
			if err != nil || ip.Zone() != "" || !ip.IsGlobalUnicast() || addresses[ip.Unmap()] {
				return fmt.Errorf("invalid VM %d IP", vm.ID)
			}
			addresses[ip.Unmap()] = true
		}
		if vm.CPU != nil && (math.IsNaN(*vm.CPU) || math.IsInf(*vm.CPU, 0) || *vm.CPU < 0) {
			return fmt.Errorf("invalid VM %d CPU", vm.ID)
		}
		for _, n := range []*int64{vm.CPUs, vm.Memory, vm.MemoryLimit, vm.DiskCapacity, vm.DiskRead, vm.DiskWrite, vm.NetIn, vm.NetOut, vm.Uptime} {
			if n != nil && *n < 0 {
				return fmt.Errorf("invalid VM %d counter", vm.ID)
			}
		}
	}
	return nil
}
