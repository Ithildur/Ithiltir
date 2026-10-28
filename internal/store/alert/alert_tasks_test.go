package alert

import (
	"context"
	"testing"
	"time"

	"dash/internal/model"
	pgtest "dash/internal/testutil/postgres"
)

func TestIntegrationFailedControlTaskDoesNotBlockRequeue(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	st := newTestStore(t, db)
	const key = "full_reconcile:test"

	if err := st.EnqueueFullReconcileTask(ctx, key); err != nil {
		t.Fatalf("EnqueueFullReconcileTask() error = %v", err)
	}
	now := time.Now().UTC()
	task, err := st.TakeNextControlTask(ctx, now)
	if err != nil {
		t.Fatalf("TakeNextControlTask() error = %v", err)
	}
	if task == nil {
		t.Fatal("TakeNextControlTask() returned nil")
	}
	if task.Status != model.TaskStatusPending {
		t.Fatalf("taken control task = %+v, want pending", task)
	}
	if err := st.FailControlTask(ctx, task.ID, "invalid payload"); err != nil {
		t.Fatalf("FailControlTask() error = %v", err)
	}
	if err := st.EnqueueFullReconcileTask(ctx, key); err != nil {
		t.Fatalf("EnqueueFullReconcileTask(requeue) error = %v", err)
	}

	var tasks []model.AlertControlTask
	if err := db.Where("dedupe_key = ?", key).Order("id ASC").Find(&tasks).Error; err != nil {
		t.Fatalf("load control tasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("control task count = %d, want 2", len(tasks))
	}
	if tasks[0].Status != model.TaskStatusFailed || tasks[1].Status != model.TaskStatusPending {
		t.Fatalf("control task statuses = %q, %q", tasks[0].Status, tasks[1].Status)
	}
}
