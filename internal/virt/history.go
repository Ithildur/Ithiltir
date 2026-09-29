package virt

import (
	"errors"
	"slices"
	"time"
)

const HistoryMaxBytes = 2 << 20

type HistoryQuery struct {
	VMID          int    `json:"vm_id"`
	Timeframe     string `json:"timeframe"`
	Consolidation string `json:"consolidation"`
}

func (q HistoryQuery) Validate() error {
	if q.VMID < 1 || int64(q.VMID) > 4294967295 || !slices.Contains([]string{"hour", "day", "week", "month", "year"}, q.Timeframe) || !slices.Contains([]string{"AVERAGE", "MAX"}, q.Consolidation) {
		return errors.New("invalid VM history query")
	}
	return nil
}

type History struct {
	Source        string         `json:"source"`
	VMID          int            `json:"vm_id"`
	Timeframe     string         `json:"timeframe"`
	Consolidation string         `json:"consolidation"`
	CollectedAt   time.Time      `json:"collected_at"`
	Points        []HistoryPoint `json:"points"`
}

type HistoryPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	CPU         *float64  `json:"cpu_ratio,omitempty"`
	Memory      *float64  `json:"memory_bytes,omitempty"`
	MemoryLimit *float64  `json:"memory_limit_bytes,omitempty"`
	DiskRead    *float64  `json:"disk_read_bytes_per_second,omitempty"`
	DiskWrite   *float64  `json:"disk_write_bytes_per_second,omitempty"`
	NetIn       *float64  `json:"net_in_bytes_per_second,omitempty"`
	NetOut      *float64  `json:"net_out_bytes_per_second,omitempty"`
}
