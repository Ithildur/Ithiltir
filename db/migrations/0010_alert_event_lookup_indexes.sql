-- +goose Up

CREATE INDEX IF NOT EXISTS idx_alert_events_type_status_server_time
    ON alert_events (object_type, status, object_id, last_trigger_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_alert_events_type_status_time
    ON alert_events (object_type, status, last_trigger_at DESC, id DESC);
