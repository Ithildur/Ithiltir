package alert

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"dash/internal/model"
	pgtest "dash/internal/testutil/postgres"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestChannelDeliveryStatus(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name string
		in   ChannelDelivery
		want ChannelDeliveryStatus
	}{
		{
			name: "disabled",
			in:   ChannelDelivery{Channel: model.NotifyChannel{Enabled: false}},
			want: ChannelDeliveryDisabled,
		},
		{
			name: "degraded after failure",
			in: ChannelDelivery{Channel: model.NotifyChannel{
				Enabled:             true,
				ConsecutiveFailures: 1,
			}},
			want: ChannelDeliveryDegraded,
		},
		{
			name: "degraded with blocked queue",
			in: ChannelDelivery{
				Channel:      model.NotifyChannel{Enabled: true},
				BlockedCount: 1,
			},
			want: ChannelDeliveryDegraded,
		},
		{
			name: "healthy",
			in: ChannelDelivery{Channel: model.NotifyChannel{
				Enabled:       true,
				LastSuccessAt: &now,
			}},
			want: ChannelDeliveryHealthy,
		},
		{
			name: "unknown before first delivery",
			in:   ChannelDelivery{Channel: model.NotifyChannel{Enabled: true}},
			want: ChannelDeliveryUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Status(); got != tt.want {
				t.Fatalf("Status() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIntegrationNotificationDeliveryState(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	st := newTestStore(t, db)

	t.Run("stale config patch cannot overwrite replacement", func(t *testing.T) {
		channel, _ := createNotificationFixture(t, db, "config-version")
		next := channel
		next.Name = "replacement-wins"
		next.Config = datatypes.JSON(`{"url":"https://replacement.example.test/hook"}`)
		if err := st.ReplaceChannel(ctx, channel.ID, channel.Revision, next); err != nil {
			t.Fatalf("ReplaceChannel() error = %v", err)
		}

		err := st.UpdateChannelConfig(
			ctx,
			channel.ID,
			channel.Revision,
			datatypes.JSON(`{"url":"https://stale.example.test/hook"}`),
		)
		if !errors.Is(err, ErrChannelVersionStale) {
			t.Fatalf("UpdateChannelConfig() error = %v, want ErrChannelVersionStale", err)
		}
		got := loadNotifyChannel(t, db, channel.ID)
		var gotConfig struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(got.Config, &gotConfig); err != nil {
			t.Fatalf("decode stored channel config: %v", err)
		}
		if got.Name != next.Name || gotConfig.URL != "https://replacement.example.test/hook" {
			t.Fatalf("stored channel = %+v, want replacement", got)
		}
		var persisted struct {
			Config       string `gorm:"column:config"`
			ConfigSealed []byte `gorm:"column:config_sealed"`
		}
		if err := db.Raw(`
			SELECT config::text AS config, config_sealed
			FROM notify_channels
			WHERE id = ?
		`, channel.ID).Scan(&persisted).Error; err != nil {
			t.Fatalf("load persisted channel config: %v", err)
		}
		if persisted.Config != "{}" || len(persisted.ConfigSealed) == 0 {
			t.Fatalf(
				"persisted channel config = %q sealed_bytes=%d, want ciphertext only",
				persisted.Config,
				len(persisted.ConfigSealed),
			)
		}

		stale := channel
		stale.Name = "stale-replacement"
		if err := st.ReplaceChannel(ctx, channel.ID, channel.Revision, stale); !errors.Is(err, ErrChannelVersionStale) {
			t.Fatalf("ReplaceChannel(stale revision) error = %v, want ErrChannelVersionStale", err)
		}
		if got := loadNotifyChannel(t, db, channel.ID); got.Name != next.Name {
			t.Fatalf("stale replacement changed channel name to %q", got.Name)
		}
	})

	t.Run("configuration update wins over stale delivery failure", func(t *testing.T) {
		channel, event := createNotificationFixture(t, db, "stale")
		now := notificationTestTime()
		row := createNotificationOutbox(t, db, event.ID, channel, model.OutboxStatusPending, now)

		leased, err := st.TakeNextNotification(ctx, now)
		if err != nil {
			t.Fatalf("TakeNextNotification() error = %v", err)
		}
		if leased == nil || leased.ID != row.ID {
			t.Fatalf("TakeNextNotification() = %+v, want id %d", leased, row.ID)
		}

		next := channel
		next.Config = datatypes.JSON(`{"url":"https://new.example.test/hook"}`)
		if err := st.ReplaceChannel(ctx, channel.ID, channel.Revision, next); err != nil {
			t.Fatalf("ReplaceChannel() error = %v", err)
		}
		if err := st.RecoverChannelNotifications(ctx, channel.ID, channel.Revision, now); err != nil {
			t.Fatalf("RecoverChannelNotifications(stale revision) error = %v", err)
		}
		if err := st.RecordChannelFailure(ctx, ChannelFailure{
			ID:        channel.ID,
			Revision:  channel.Revision,
			Code:      "webhook_http_401",
			LastError: "old endpoint rejected the test",
			FailedAt:  now,
		}); err != nil {
			t.Fatalf("RecordChannelFailure(stale revision) error = %v", err)
		}
		if got := loadNotifyChannel(t, db, channel.ID); got.LastSuccessAt != nil || got.LastFailureAt != nil {
			t.Fatalf("stale test result changed channel health: %+v", got)
		}
		if err := st.BlockNotification(ctx, NotificationFailure{
			ID:              row.ID,
			ChannelID:       channel.ID,
			ChannelRevision: channel.Revision,
			Code:            "webhook_http_401",
			LastError:       "old endpoint rejected the request",
			FailedAt:        now,
			NextAttemptAt:   now.Add(time.Hour),
		}); err != nil {
			t.Fatalf("BlockNotification() error = %v", err)
		}

		gotRow := loadNotificationOutbox(t, db, row.ID)
		if gotRow.Status != model.OutboxStatusRetry || gotRow.FailureCode != nil || gotRow.LastError != nil {
			t.Fatalf("stale failure row = %+v, want clean immediate retry", gotRow)
		}
		gotChannel := loadNotifyChannel(t, db, channel.ID)
		if gotChannel.ConsecutiveFailures != 0 || gotChannel.LastErrorCode != nil {
			t.Fatalf("stale failure changed channel health: %+v", gotChannel)
		}
		if err := db.Delete(&gotRow).Error; err != nil {
			t.Fatalf("delete stale outbox fixture: %v", err)
		}
	})

	t.Run("blocked probes recover and channel enable wakes paused queue", func(t *testing.T) {
		channel, event := createNotificationFixture(t, db, "recovery")
		now := notificationTestTime()
		first := createNotificationOutbox(t, db, event.ID, channel, model.OutboxStatusBlocked, now)
		second := createNotificationOutbox(t, db, event.ID, channel, model.OutboxStatusBlocked, now)
		paused := createNotificationOutbox(t, db, event.ID, channel, model.OutboxStatusPaused, now)

		leased, err := st.TakeNextNotification(ctx, now)
		if err != nil {
			t.Fatalf("TakeNextNotification() error = %v", err)
		}
		if leased == nil || leased.ID != first.ID {
			t.Fatalf("TakeNextNotification() = %+v, want first blocked id %d", leased, first.ID)
		}
		nextProbe := now.Add(5 * time.Minute)
		if err := st.BlockNotification(ctx, NotificationFailure{
			ID:              first.ID,
			ChannelID:       channel.ID,
			ChannelRevision: channel.Revision,
			Code:            "webhook_http_401",
			LastError:       "unauthorized",
			FailedAt:        now,
			NextAttemptAt:   nextProbe,
		}); err != nil {
			t.Fatalf("BlockNotification() error = %v", err)
		}

		first = loadNotificationOutbox(t, db, first.ID)
		second = loadNotificationOutbox(t, db, second.ID)
		if first.ProbeCount != 1 || !first.NextAttemptAt.Equal(nextProbe) {
			t.Fatalf("first blocked probe = %+v", first)
		}
		if !second.NextAttemptAt.Equal(nextProbe) {
			t.Fatalf("second next_attempt_at = %s, want %s", second.NextAttemptAt, nextProbe)
		}

		deliveries, err := st.ListChannelDeliveries(ctx)
		if err != nil {
			t.Fatalf("ListChannelDeliveries() error = %v", err)
		}
		delivery := deliveryByID(t, deliveries, channel.ID)
		if delivery.Status() != ChannelDeliveryDegraded || delivery.PendingCount != 3 || delivery.BlockedCount != 2 {
			t.Fatalf("delivery state = %+v status=%s", delivery, delivery.Status())
		}
		if delivery.NextProbeAt == nil || !delivery.NextProbeAt.Equal(nextProbe) {
			t.Fatalf("next probe = %v, want %s", delivery.NextProbeAt, nextProbe)
		}

		if err := st.SetChannelEnabled(ctx, channel.ID, false); err != nil {
			t.Fatalf("SetChannelEnabled(false) error = %v", err)
		}
		assertNotificationStatuses(t, db, []int64{first.ID, second.ID, paused.ID}, model.OutboxStatusPaused)
		if err := db.Model(&model.AlertNotificationOutbox{}).
			Where("id = ?", second.ID).
			Updates(map[string]any{
				"status":          model.OutboxStatusBlocked,
				"attempt_count":   9,
				"next_attempt_at": now.Add(time.Hour),
			}).Error; err != nil {
			t.Fatalf("simulate legacy blocked row: %v", err)
		}
		if err := st.SetChannelEnabled(ctx, channel.ID, true); err != nil {
			t.Fatalf("SetChannelEnabled(true) error = %v", err)
		}
		assertNotificationStatuses(t, db, []int64{first.ID, second.ID, paused.ID}, model.OutboxStatusRetry)
		for _, id := range []int64{first.ID, second.ID, paused.ID} {
			if got := loadNotificationOutbox(t, db, id).AttemptCount; got != 0 {
				t.Fatalf("notification %d attempt_count = %d, want reset", id, got)
			}
		}
		delivery, err = st.GetChannelDelivery(ctx, channel.ID)
		if err != nil {
			t.Fatalf("GetChannelDelivery() error = %v", err)
		}
		if delivery.Status() != ChannelDeliveryDegraded || delivery.Channel.LastErrorCode == nil {
			t.Fatalf("re-enabled delivery = %+v status=%s, want degraded until success", delivery, delivery.Status())
		}
		if delivery.NextRetryAt == nil {
			t.Fatalf("re-enabled delivery next retry = nil, want scheduled retry")
		}

		channel = loadNotifyChannel(t, db, channel.ID)
		leaseAt := notificationTestTime().Add(time.Second)
		leased, err = st.TakeNextNotification(ctx, leaseAt)
		if err != nil {
			t.Fatalf("TakeNextNotification(recovered) error = %v", err)
		}
		if leased == nil {
			t.Fatal("TakeNextNotification(recovered) returned nil")
		}
		if err := db.Model(&model.AlertNotificationOutbox{}).
			Where("id = ?", second.ID).
			Updates(map[string]any{
				"status":        model.OutboxStatusBlocked,
				"attempt_count": 9,
			}).Error; err != nil {
			t.Fatalf("simulate blocked notification before recovery: %v", err)
		}
		sentAt := notificationTestTime()
		if err := st.CompleteNotification(ctx, leased.ID, channel.ID, channel.Revision, sentAt); err != nil {
			t.Fatalf("CompleteNotification() error = %v", err)
		}
		second = loadNotificationOutbox(t, db, second.ID)
		if second.Status != model.OutboxStatusRetry || second.AttemptCount != 0 {
			t.Fatalf("recovered notification = %+v, want retry with reset attempt budget", second)
		}
		channel = loadNotifyChannel(t, db, channel.ID)
		if channel.LastSuccessAt == nil || !channel.LastSuccessAt.Equal(sentAt) || channel.ConsecutiveFailures != 0 {
			t.Fatalf("recovered channel = %+v", channel)
		}
		if channel.LastFailureAt == nil || !channel.LastFailureAt.Equal(now) {
			t.Fatalf("last failure history = %v, want %s", channel.LastFailureAt, now)
		}
	})
}

func TestIntegrationBlockedDeliveryCoalescesChannelQueue(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	st := newTestStore(t, db)
	channel, event := createNotificationFixture(t, db, "coalesce")
	now := notificationTestTime()
	probe := createNotificationOutbox(t, db, event.ID, channel, model.OutboxStatusPending, now)
	pending := createNotificationOutbox(t, db, event.ID, channel, model.OutboxStatusPending, now.Add(time.Minute))
	retry := createNotificationOutbox(t, db, event.ID, channel, model.OutboxStatusRetry, now.Add(time.Minute))

	leased, err := st.TakeNextNotification(ctx, now)
	if err != nil {
		t.Fatalf("TakeNextNotification() error = %v", err)
	}
	if leased == nil || leased.ID != probe.ID {
		t.Fatalf("TakeNextNotification() = %+v, want id %d", leased, probe.ID)
	}
	nextProbe := now.Add(5 * time.Minute)
	if err := st.BlockNotification(ctx, NotificationFailure{
		ID:              probe.ID,
		ChannelID:       channel.ID,
		ChannelRevision: channel.Revision,
		Code:            "webhook_http_401",
		LastError:       "unauthorized",
		FailedAt:        now,
		NextAttemptAt:   nextProbe,
	}); err != nil {
		t.Fatalf("BlockNotification() error = %v", err)
	}

	assertNotificationStatuses(t, db, []int64{probe.ID, pending.ID, retry.ID}, model.OutboxStatusBlocked)
	for _, id := range []int64{pending.ID, retry.ID} {
		got := loadNotificationOutbox(t, db, id)
		if !got.NextAttemptAt.Equal(nextProbe) {
			t.Fatalf("notification %d next_attempt_at = %s, want %s", id, got.NextAttemptAt, nextProbe)
		}
	}

	if err := st.EnqueueNotifications(
		ctx,
		"system:while-blocked",
		"system",
		[]model.NotifyChannel{channel},
		NotificationPayload{Title: "queued", Body: "queued"},
		now.Add(time.Minute),
	); err != nil {
		t.Fatalf("EnqueueNotifications(blocked channel) error = %v", err)
	}
	var queued model.AlertNotificationOutbox
	if err := db.Where("dedupe_key = ?", notificationDedupeKey("system:while-blocked", channel.ID)).Take(&queued).Error; err != nil {
		t.Fatalf("load notification enqueued while blocked: %v", err)
	}
	if queued.Status != model.OutboxStatusBlocked || !queued.NextAttemptAt.Equal(nextProbe) {
		t.Fatalf("notification enqueued while blocked = %+v, want blocked at %s", queued, nextProbe)
	}
}

func TestIntegrationTakeNotificationPreservesProbeAcrossCrash(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	st := newTestStore(t, db)
	channel, event := createNotificationFixture(t, db, "sending-recovery")
	now := notificationTestTime()
	row := createNotificationOutbox(t, db, event.ID, channel, model.OutboxStatusBlocked, now)
	if err := db.Model(&model.AlertNotificationOutbox{}).
		Where("id = ?", row.ID).
		Update("probe_count", 2).Error; err != nil {
		t.Fatalf("set probe count: %v", err)
	}

	taken, err := st.TakeNextNotification(ctx, now)
	if err != nil {
		t.Fatalf("TakeNextNotification() error = %v", err)
	}
	if taken == nil || taken.ID != row.ID {
		t.Fatalf("TakeNextNotification() = %+v, want id %d", taken, row.ID)
	}
	if !taken.IsProbe || taken.LeasedUntil == nil || !taken.LeasedUntil.Equal(now) {
		t.Fatalf("taken probe = %+v, want persisted probe marker at %s", taken, now)
	}
	persisted := loadNotificationOutbox(t, db, row.ID)
	if persisted.Status != model.OutboxStatusSending || !persisted.IsProbe {
		t.Fatalf("persisted in-flight probe = %+v", persisted)
	}

	recoveredAt := now.Add(time.Second)
	recovered, err := st.TakeNextNotification(ctx, recoveredAt)
	if err != nil {
		t.Fatalf("TakeNextNotification(recovered) error = %v", err)
	}
	if recovered == nil || recovered.ID != row.ID || !recovered.IsProbe {
		t.Fatalf("TakeNextNotification(recovered) = %+v, want probe id %d", recovered, row.ID)
	}

	next := recoveredAt.Add(time.Second)
	if err := st.RequeueNotification(ctx, row.ID, channel.ID, next); err != nil {
		t.Fatalf("RequeueNotification(probe) error = %v", err)
	}
	requeued := loadNotificationOutbox(t, db, row.ID)
	if requeued.Status != model.OutboxStatusBlocked || requeued.IsProbe ||
		requeued.AttemptCount != 0 || requeued.ProbeCount != 2 ||
		!requeued.NextAttemptAt.Equal(next) {
		t.Fatalf("requeued probe = %+v, want blocked with intact budgets", requeued)
	}
}

func TestIntegrationCompleteNotificationIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	st := newTestStore(t, db)
	channel, event := createNotificationFixture(t, db, "idempotent-completion")
	now := notificationTestTime()
	row := createNotificationOutbox(t, db, event.ID, channel, model.OutboxStatusPending, now)

	taken, err := st.TakeNextNotification(ctx, now)
	if err != nil {
		t.Fatalf("TakeNextNotification() error = %v", err)
	}
	if taken == nil || taken.ID != row.ID {
		t.Fatalf("TakeNextNotification() = %+v, want id %d", taken, row.ID)
	}
	if err := st.CompleteNotification(ctx, row.ID, channel.ID, channel.Revision, now); err != nil {
		t.Fatalf("CompleteNotification() error = %v", err)
	}
	if err := st.CompleteNotification(ctx, row.ID, channel.ID, channel.Revision, now); err != nil {
		t.Fatalf("CompleteNotification(retry) error = %v", err)
	}

	completed := loadNotificationOutbox(t, db, row.ID)
	if completed.Status != model.OutboxStatusSent || completed.AttemptCount != 1 {
		t.Fatalf("completed notification = %+v, want one recorded attempt", completed)
	}
	storedChannel := loadNotifyChannel(t, db, channel.ID)
	if storedChannel.LastSuccessAt == nil || !storedChannel.LastSuccessAt.Equal(now) {
		t.Fatalf("channel last_success_at = %v, want %s", storedChannel.LastSuccessAt, now)
	}
}

func TestIntegrationOperationalFailureDoesNotConsumeDeliveryBudget(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	st := newTestStore(t, db)
	channel, event := createNotificationFixture(t, db, "operational-failure")
	now := notificationTestTime()
	row := createNotificationOutbox(t, db, event.ID, channel, model.OutboxStatusPending, now)

	taken, err := st.TakeNextNotification(ctx, now)
	if err != nil {
		t.Fatalf("TakeNextNotification() error = %v", err)
	}
	if taken == nil || taken.ID != row.ID || taken.AttemptCount != 0 {
		t.Fatalf("TakeNextNotification() = %+v, want uncounted lease for %d", taken, row.ID)
	}

	next := now.Add(time.Second)
	if err := st.RequeueNotification(ctx, row.ID, channel.ID, next); err != nil {
		t.Fatalf("RequeueNotification() error = %v", err)
	}
	got := loadNotificationOutbox(t, db, row.ID)
	if got.Status != model.OutboxStatusRetry || got.AttemptCount != 0 || !got.NextAttemptAt.Equal(next) {
		t.Fatalf("requeued notification = %+v, want retry without consumed attempt", got)
	}
	if gotChannel := loadNotifyChannel(t, db, channel.ID); gotChannel.ConsecutiveFailures != 0 || gotChannel.LastFailureAt != nil {
		t.Fatalf("operational failure changed channel health: %+v", gotChannel)
	}

	taken, err = st.TakeNextNotification(ctx, next)
	if err != nil {
		t.Fatalf("TakeNextNotification(retry) error = %v", err)
	}
	if taken == nil || taken.AttemptCount != 0 {
		t.Fatalf("TakeNextNotification(retry) = %+v, want intact delivery budget", taken)
	}
	failure := NotificationFailure{
		ID:            row.ID,
		ChannelID:     channel.ID,
		Code:          "webhook_unavailable",
		LastError:     "remote unavailable",
		FailedAt:      next,
		NextAttemptAt: next.Add(time.Minute),
	}
	if err := st.RetryNotification(ctx, failure); err == nil {
		t.Fatal("RetryNotification() error = nil, want missing revision rejected")
	}
	if got := loadNotificationOutbox(t, db, row.ID); got.Status != model.OutboxStatusSending || got.AttemptCount != 0 {
		t.Fatalf("invalid delivery result changed notification: %+v", got)
	}

	failure.ChannelRevision = channel.Revision
	if err := st.RetryNotification(ctx, failure); err != nil {
		t.Fatalf("RetryNotification(valid result) error = %v", err)
	}
	got = loadNotificationOutbox(t, db, row.ID)
	if got.Status != model.OutboxStatusRetry || got.AttemptCount != 1 {
		t.Fatalf("recorded delivery failure = %+v, want one consumed attempt", got)
	}
	gotChannel := loadNotifyChannel(t, db, channel.ID)
	if gotChannel.ConsecutiveFailures != 1 || gotChannel.LastFailureAt == nil {
		t.Fatalf("recorded delivery failure did not change channel health: %+v", gotChannel)
	}

	invalid := createNotificationOutbox(t, db, event.ID, channel, model.OutboxStatusPending, now)
	taken, err = st.TakeNextNotification(ctx, now)
	if err != nil {
		t.Fatalf("TakeNextNotification(invalid payload) error = %v", err)
	}
	if taken == nil || taken.ID != invalid.ID {
		t.Fatalf("TakeNextNotification(invalid payload) = %+v, want id %d", taken, invalid.ID)
	}
	if err := st.DiscardNotification(ctx, NotificationFailure{
		ID:        invalid.ID,
		ChannelID: channel.ID,
		Code:      "payload_invalid",
		LastError: "invalid payload",
		FailedAt:  now,
	}); err != nil {
		t.Fatalf("DiscardNotification() error = %v", err)
	}
	if got := loadNotificationOutbox(t, db, invalid.ID); got.Status != model.OutboxStatusDiscarded || got.AttemptCount != 0 {
		t.Fatalf("discarded pre-delivery notification = %+v, want no consumed attempt", got)
	}
}

func TestIntegrationEnqueueSystemNotificationIsDurableAndIdempotent(t *testing.T) {
	ctx := context.Background()
	db := pgtest.NewDB(t)
	st := newTestStore(t, db)
	channel, _ := createNotificationFixture(t, db, "system")
	payload := NotificationPayload{
		Title: "Dash update available",
		Body:  "v1.2.3",
		Metadata: map[string]string{
			"kind":  "dash_update",
			"event": "available",
		},
	}
	for range 2 {
		if err := st.EnqueueNotifications(
			ctx,
			"dash-update:available:release:v1.2.3",
			"available",
			[]model.NotifyChannel{channel},
			payload,
			notificationTestTime(),
		); err != nil {
			t.Fatalf("EnqueueNotifications() error = %v", err)
		}
	}

	var rows []model.AlertNotificationOutbox
	if err := db.Where("channel_id = ?", channel.ID).Find(&rows).Error; err != nil {
		t.Fatalf("load system notifications: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("system notification count = %d, want 1", len(rows))
	}
	if rows[0].EventID != nil || rows[0].Transition != "available" {
		t.Fatalf("system notification = %+v", rows[0])
	}
}

func createNotificationFixture(t *testing.T, db *gorm.DB, suffix string) (model.NotifyChannel, model.AlertEvent) {
	t.Helper()
	now := notificationTestTime()
	channel := model.NotifyChannel{
		Name:    "channel-" + suffix,
		Type:    model.NotifyTypeWebhook,
		Config:  datatypes.JSON(`{"url":"https://example.test/hook"}`),
		Enabled: true,
	}
	if err := newTestStore(t, db).CreateChannel(context.Background(), &channel); err != nil {
		t.Fatalf("CreateChannel() error = %v", err)
	}
	channel = loadNotifyChannel(t, db, channel.ID)
	if channel.Revision < 1 {
		t.Fatalf("channel revision = %d, want at least 1", channel.Revision)
	}
	event := model.AlertEvent{
		RuleID:         now.UnixNano(),
		RuleGeneration: 1,
		RuleSnapshot:   datatypes.JSON(`{}`),
		ObjectType:     model.ObjectTypeServer,
		ObjectID:       now.UnixNano(),
		Status:         model.AlertStatusClosed,
		FirstTriggerAt: now,
		LastTriggerAt:  now,
		ClosedAt:       &now,
	}
	if err := db.Create(&event).Error; err != nil {
		t.Fatalf("Create(event) error = %v", err)
	}
	return channel, event
}

func createNotificationOutbox(
	t *testing.T,
	db *gorm.DB,
	eventID int64,
	channel model.NotifyChannel,
	status model.OutboxStatus,
	next time.Time,
) model.AlertNotificationOutbox {
	t.Helper()
	row := model.AlertNotificationOutbox{
		EventID:       &eventID,
		Transition:    "opened",
		ChannelID:     channel.ID,
		ChannelType:   channel.Type,
		Payload:       datatypes.JSON(`{"title":"test","body":"test"}`),
		DedupeKey:     uuid.NewString(),
		Status:        status,
		NextAttemptAt: next,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("Create(outbox) error = %v", err)
	}
	return row
}

func notificationTestTime() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func loadNotificationOutbox(t *testing.T, db *gorm.DB, id int64) model.AlertNotificationOutbox {
	t.Helper()
	var row model.AlertNotificationOutbox
	if err := db.First(&row, id).Error; err != nil {
		t.Fatalf("load outbox %d: %v", id, err)
	}
	return row
}

func loadNotifyChannel(t *testing.T, db *gorm.DB, id int64) model.NotifyChannel {
	t.Helper()
	channel, err := newTestStore(t, db).GetChannel(context.Background(), id)
	if err != nil {
		t.Fatalf("load channel %d: %v", id, err)
	}
	return *channel
}

func deliveryByID(t *testing.T, deliveries []ChannelDelivery, id int64) ChannelDelivery {
	t.Helper()
	for _, delivery := range deliveries {
		if delivery.Channel.ID == id {
			return delivery
		}
	}
	t.Fatalf("channel delivery %d not found", id)
	return ChannelDelivery{}
}

func assertNotificationStatuses(t *testing.T, db *gorm.DB, ids []int64, want model.OutboxStatus) {
	t.Helper()
	for _, id := range ids {
		if got := loadNotificationOutbox(t, db, id).Status; got != want {
			t.Fatalf("notification %d status = %q, want %q", id, got, want)
		}
	}
}
