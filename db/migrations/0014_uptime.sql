-- +goose Up

CREATE TABLE node_online (
    server_id BIGINT      NOT NULL REFERENCES servers (id),
    minute    TIMESTAMPTZ NOT NULL,
    online    BOOLEAN     NOT NULL,

    PRIMARY KEY (server_id, minute),
    CONSTRAINT node_online_minute_aligned
        CHECK (minute = date_trunc('minute', minute))
);

SELECT create_hypertable(
    'node_online',
    'minute',
    chunk_time_interval => INTERVAL '1 day',
    create_default_indexes => FALSE
);

COMMENT ON TABLE node_online IS '节点分钟级在线状态；缺少记录表示未知';
COMMENT ON COLUMN node_online.server_id IS '节点 ID';
COMMENT ON COLUMN node_online.minute IS '采样分钟，时间对齐到整分钟';
COMMENT ON COLUMN node_online.online IS '该采样分钟的在线状态';

SELECT add_retention_policy('node_online', INTERVAL '46 days');

ALTER TABLE system_settings
    ADD COLUMN uptime_guest_visible BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN uptime_warning_sla DOUBLE PRECISION NOT NULL DEFAULT 99,
    ADD COLUMN uptime_error_sla DOUBLE PRECISION NOT NULL DEFAULT 95,
    ADD CONSTRAINT system_settings_uptime_sla CHECK (
        uptime_error_sla >= 0
        AND uptime_error_sla < uptime_warning_sla
        AND uptime_warning_sla <= 100
    );

COMMENT ON COLUMN system_settings.uptime_guest_visible IS '是否向游客展示节点 uptime';
COMMENT ON COLUMN system_settings.uptime_warning_sla IS '每日 uptime 黄色警告阈值，单位为百分数';
COMMENT ON COLUMN system_settings.uptime_error_sla IS '每日 uptime 红色错误阈值，单位为百分数';
