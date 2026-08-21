-- +goose Up
-- +goose StatementBegin

-- Migration 11 widened these columns from varchar to text. Reset their
-- defaults to the final column type so upgraded and baseline schemas converge.
ALTER TABLE disk_physical_metrics
    ALTER COLUMN path SET DEFAULT ''::text;

ALTER TABLE server_current_disk_metrics
    ALTER COLUMN path SET DEFAULT ''::text;

ALTER TABLE server_current_disk_usage_metrics
    ALTER COLUMN mountpoint SET DEFAULT ''::text,
    ALTER COLUMN path SET DEFAULT ''::text;

-- Versions 1 and 11 checked trigger names globally. A same-named trigger on
-- another relation could therefore suppress an application trigger.
CREATE OR REPLACE FUNCTION ensure_updated_at_trigger(table_name TEXT)
RETURNS VOID AS $$
BEGIN
    EXECUTE format(
        'CREATE TRIGGER %I BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION set_updated_at()',
        table_name || '_updated_at',
        table_name
    );
END;
$$ LANGUAGE plpgsql;

DO $migration$
DECLARE
    target_schema TEXT := current_schema();
    table_name TEXT;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'groups', 'servers', 'services', 'tasks', 'notify_channels',
        'alert_settings', 'alert_rules', 'alert_rule_mounts',
        'alert_control_tasks', 'system_settings', 'metric_settings',
        'traffic_settings', 'traffic_month_usage', 'traffic_5m',
        'traffic_monthly', 'server_current_metrics',
        'server_current_disk_metrics', 'server_current_disk_usage_metrics',
        'server_current_nic_metrics', 'traffic_materialization_progress',
        'traffic_usage_repairs'
    ]
    LOOP
        EXECUTE format(
            'DROP TRIGGER IF EXISTS %I ON %I.%I',
            table_name || '_updated_at',
            target_schema,
            table_name
        );
        EXECUTE format(
            'CREATE TRIGGER %I BEFORE UPDATE ON %I.%I FOR EACH ROW EXECUTE FUNCTION %I.set_updated_at()',
            table_name || '_updated_at',
            target_schema,
            table_name,
            target_schema
        );
    END LOOP;
END
$migration$;

-- +goose StatementEnd
