package traffic

import (
	"fmt"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type materializationKind string

const (
	materializationUsage materializationKind = "usage"
	materializationFacts materializationKind = "facts"

	trafficMaterializationChunk   = time.Hour
	trafficMaterializationOverlap = trafficBackfillWindow
	trafficUsageRepairChunk       = 6 * time.Hour
)

func lockMaterializationProgress(tx *gorm.DB, kind materializationKind) (model.TrafficMaterializationProgress, error) {
	var progress model.TrafficMaterializationProgress
	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("kind = ?", string(kind)).
		Take(&progress).Error
	if err != nil {
		return model.TrafficMaterializationProgress{}, fmt.Errorf("lock traffic %s progress: %w", kind, err)
	}
	progress.ScannedUntil = trafficBucketStart(progress.ScannedUntil)
	return progress, nil
}

func nextMaterializationRange(progress model.TrafficMaterializationProgress, target, sourceFloor time.Time, overlap time.Duration) (time.Time, time.Time, bool) {
	cursor := trafficBucketStart(progress.ScannedUntil)
	target = trafficBucketStart(target)
	sourceFloor = trafficBucketStart(sourceFloor)
	if cursor.Before(sourceFloor) {
		cursor = sourceFloor
	}
	if !target.After(cursor) {
		return cursor, cursor, false
	}
	end := cursor.Add(trafficMaterializationChunk)
	if end.After(target) {
		end = target
	}
	start := cursor
	if end.Equal(target) {
		start = cursor.Add(-overlap)
	}
	if start.Before(sourceFloor) {
		start = sourceFloor
	}
	return start, end, true
}

func setMaterializationProgress(tx *gorm.DB, kind materializationKind, scannedUntil time.Time) error {
	result := tx.
		Model(&model.TrafficMaterializationProgress{}).
		Where("kind = ?", string(kind)).
		Update("scanned_until", scannedUntil.UTC())
	if result.Error != nil {
		return fmt.Errorf("advance traffic %s progress: %w", kind, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("advance traffic %s progress: row is missing", kind)
	}
	return nil
}

func resetFactsProgressForBilling(tx *gorm.DB, current, next UsageMode, ref time.Time) error {
	if current != UsageLite || next != UsageBilling {
		return nil
	}
	return setMaterializationProgress(
		tx,
		materializationFacts,
		trafficBucketStart(ref.Add(-trafficBackfillWindow)),
	)
}
