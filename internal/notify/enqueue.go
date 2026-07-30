package notify

type EnqueueStatus string

const (
	EnqueueQueued           EnqueueStatus = "queued"
	EnqueueSkippedNoTargets EnqueueStatus = "skipped_no_targets"
)
