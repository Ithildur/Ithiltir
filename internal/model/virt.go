package model

import (
	"gorm.io/datatypes"
	"time"
)

// ServerVirt contains only the latest VM observation, independently of host metrics.
type ServerVirt struct {
	ServerID    int64 `gorm:"primaryKey"`
	CollectedAt time.Time
	ReceivedAt  time.Time
	Snapshot    datatypes.JSON
}

func (ServerVirt) TableName() string { return "server_virt" }
