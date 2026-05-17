-- +goose Up
-- +goose StatementBegin
-- 服务器监控系统初始化（PostgreSQL + TimescaleDB）
-- 说明：建表 + Timescale hypertable/策略 + 触发器 + 预置规则

CREATE EXTENSION IF NOT EXISTS timescaledb;

-- 可选：按需设置时区
-- ALTER DATABASE current_database() SET TIMEZONE TO 'UTC';
-- ALTER DATABASE current_database() SET TIMEZONE TO 'Asia/Shanghai';

-- ---------------------------------------------------------
-- updated_at：UPDATE 时自动刷新
-- ---------------------------------------------------------

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION ensure_updated_at_trigger(table_name TEXT)
RETURNS VOID AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_trigger
        WHERE tgname = table_name || '_updated_at'
    ) THEN
        EXECUTE format(
            'CREATE TRIGGER %I BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION set_updated_at()',
            table_name || '_updated_at',
            table_name
        );
    END IF;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------
-- 枚举类型
-- ---------------------------------------------------------

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'service_type') THEN
        CREATE TYPE service_type AS ENUM ('http', 'tcp', 'ping', 'dns', 'tls');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'task_type') THEN
        CREATE TYPE task_type AS ENUM ('shell', 'http', 'script');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'target_type') THEN
        CREATE TYPE target_type AS ENUM ('all', 'group', 'server', 'custom');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'object_type') THEN
        CREATE TYPE object_type AS ENUM ('server', 'service');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'alert_status') THEN
        CREATE TYPE alert_status AS ENUM ('open', 'closed');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'notify_type') THEN
        CREATE TYPE notify_type AS ENUM ('telegram', 'email', 'webhook', 'wechat', 'slack', 'discord');
    END IF;
END
$$;

-- ---------------------------------------------------------
-- 分组（机器/业务分组）
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS groups (
    id         BIGSERIAL   PRIMARY KEY,
    name       VARCHAR(64) NOT NULL,
    remark     VARCHAR(255),
    is_deleted BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE groups IS '分组';
COMMENT ON COLUMN groups.id IS '主键';
COMMENT ON COLUMN groups.name IS '分组名';
COMMENT ON COLUMN groups.remark IS '备注';
COMMENT ON COLUMN groups.is_deleted IS '删除标记（soft delete）';
COMMENT ON COLUMN groups.created_at IS '创建时间';
COMMENT ON COLUMN groups.updated_at IS '更新时间';

SELECT ensure_updated_at_trigger('groups');

-- 默认分组（没就补一个）
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM groups WHERE name = 'default') THEN
        INSERT INTO groups (name, remark) VALUES ('default', 'default');
    END IF;
END
$$;

-- ---------------------------------------------------------
-- 服务器节点（与 agent/node 实例一一对应）
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS servers (
    id                 BIGSERIAL    PRIMARY KEY,

    name               VARCHAR(64)  NOT NULL,
    hostname           VARCHAR(128) NOT NULL,
    secret             VARCHAR(128) NOT NULL,
    tags               JSONB,

    ip                 VARCHAR(64),
    os                 VARCHAR(32),
    platform           VARCHAR(32),
    platform_version   VARCHAR(64),
    kernel_version     VARCHAR(64),
    arch               VARCHAR(32),

    location           VARCHAR(64),

    cpu_model          VARCHAR(128),
    cpu_vendor         VARCHAR(64),
    cpu_cores_physical SMALLINT,
    cpu_cores_logical  SMALLINT,
    cpu_sockets        SMALLINT     DEFAULT 1,
    cpu_mhz            DOUBLE PRECISION,

    mem_total          BIGINT,
    swap_total         BIGINT,
    disk_total         BIGINT,
    root_path          VARCHAR(256),
    root_fs_type       VARCHAR(32),

    raid_supported     BOOLEAN,
    raid_available     BOOLEAN,
    agent_version      VARCHAR(64),

    remark             VARCHAR(255),
    display_order      INTEGER      NOT NULL DEFAULT 0,
    interval_sec       INTEGER,
    is_guest_visible    BOOLEAN     NOT NULL DEFAULT FALSE,
    traffic_p95_enabled BOOLEAN     NOT NULL DEFAULT FALSE,

    is_deleted         BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT uk_servers_secret UNIQUE (secret)
);

CREATE INDEX IF NOT EXISTS idx_servers_display_order ON servers (display_order);

COMMENT ON TABLE servers IS '服务器节点';
COMMENT ON COLUMN servers.id IS '主键';
COMMENT ON COLUMN servers.name IS '显示名称';
COMMENT ON COLUMN servers.hostname IS '主机名';
COMMENT ON COLUMN servers.secret IS '节点校验密钥';
COMMENT ON COLUMN servers.tags IS '标签数组（JSON）';
COMMENT ON COLUMN servers.ip IS 'IP 地址';
COMMENT ON COLUMN servers.os IS '操作系统';
COMMENT ON COLUMN servers.platform IS '平台标识';
COMMENT ON COLUMN servers.platform_version IS '平台版本';
COMMENT ON COLUMN servers.kernel_version IS '内核版本';
COMMENT ON COLUMN servers.arch IS '架构';
COMMENT ON COLUMN servers.location IS '位置/机房';
COMMENT ON COLUMN servers.cpu_model IS 'CPU 型号';
COMMENT ON COLUMN servers.cpu_vendor IS 'CPU 厂商';
COMMENT ON COLUMN servers.cpu_cores_physical IS '物理核数';
COMMENT ON COLUMN servers.cpu_cores_logical IS '逻辑核数';
COMMENT ON COLUMN servers.cpu_sockets IS 'CPU 插槽数';
COMMENT ON COLUMN servers.cpu_mhz IS 'CPU 主频（MHz）';
COMMENT ON COLUMN servers.mem_total IS '内存总量（B）';
COMMENT ON COLUMN servers.swap_total IS 'Swap 总量（B）';
COMMENT ON COLUMN servers.disk_total IS '磁盘总量（B）';
COMMENT ON COLUMN servers.root_path IS '前端根磁盘展示入口，通常来自最大逻辑盘挂载点';
COMMENT ON COLUMN servers.root_fs_type IS '根分区文件系统';
COMMENT ON COLUMN servers.raid_supported IS '是否支持 RAID';
COMMENT ON COLUMN servers.raid_available IS 'RAID 是否可用';
COMMENT ON COLUMN servers.agent_version IS 'Agent 版本';
COMMENT ON COLUMN servers.remark IS '备注';
COMMENT ON COLUMN servers.display_order IS '展示排序权重';
COMMENT ON COLUMN servers.interval_sec IS 'Node 上报间隔（s）';
COMMENT ON COLUMN servers.is_guest_visible IS '游客可见';
COMMENT ON COLUMN servers.traffic_p95_enabled IS '是否启用95带宽统计';
COMMENT ON COLUMN servers.is_deleted IS '删除标记（soft delete）';
COMMENT ON COLUMN servers.created_at IS '创建时间';
COMMENT ON COLUMN servers.updated_at IS '更新时间';

SELECT ensure_updated_at_trigger('servers');

-- ---------------------------------------------------------
-- 服务器 - 分组（多对多）
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS server_groups (
    server_id  BIGINT      NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    group_id   BIGINT      NOT NULL REFERENCES groups (id)  ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (server_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_server_groups_group ON server_groups (group_id);

COMMENT ON TABLE server_groups IS '服务器分组关系';
COMMENT ON COLUMN server_groups.server_id IS '服务器 ID';
COMMENT ON COLUMN server_groups.group_id IS '分组 ID';
COMMENT ON COLUMN server_groups.created_at IS '创建时间';

-- ---------------------------------------------------------
-- 服务器历史指标（Timescale hypertable）
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS server_metrics (
    server_id            BIGINT      NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    collected_at         TIMESTAMPTZ NOT NULL,
    reported_at          TIMESTAMPTZ,

    cpu_usage_ratio      DOUBLE PRECISION NOT NULL DEFAULT 0,
    load1                DOUBLE PRECISION NOT NULL DEFAULT 0,
    load5                DOUBLE PRECISION NOT NULL DEFAULT 0,
    load15               DOUBLE PRECISION NOT NULL DEFAULT 0,

    cpu_user             DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_system           DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_idle             DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_iowait           DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_steal            DOUBLE PRECISION NOT NULL DEFAULT 0,

    mem_total            BIGINT           NOT NULL DEFAULT 0,
    mem_used             BIGINT           NOT NULL DEFAULT 0,
    mem_available        BIGINT           NOT NULL DEFAULT 0,
    mem_buffers          BIGINT           NOT NULL DEFAULT 0,
    mem_cached           BIGINT           NOT NULL DEFAULT 0,
    mem_used_ratio       DOUBLE PRECISION NOT NULL DEFAULT 0,

    swap_total           BIGINT           NOT NULL DEFAULT 0,
    swap_used            BIGINT           NOT NULL DEFAULT 0,
    swap_free            BIGINT           NOT NULL DEFAULT 0,
    swap_used_ratio      DOUBLE PRECISION NOT NULL DEFAULT 0,

    net_in_bytes         BIGINT           NOT NULL DEFAULT 0,
    net_out_bytes        BIGINT           NOT NULL DEFAULT 0,
    net_in_bps           DOUBLE PRECISION NOT NULL DEFAULT 0,
    net_out_bps          DOUBLE PRECISION NOT NULL DEFAULT 0,

    process_count        INTEGER          NOT NULL DEFAULT 0,
    tcp_conn             INTEGER          NOT NULL DEFAULT 0,
    udp_conn             INTEGER          NOT NULL DEFAULT 0,

    uptime_seconds       BIGINT           NOT NULL DEFAULT 0,

    raid_supported       BOOLEAN          NOT NULL DEFAULT FALSE,
    raid_available       BOOLEAN          NOT NULL DEFAULT FALSE,
    raid_overall_health  VARCHAR(16)      NOT NULL DEFAULT '',

    raid                 JSONB,
    disk_smart           JSONB,
    thermal              JSONB,

    PRIMARY KEY (server_id, collected_at)
);

CREATE INDEX IF NOT EXISTS idx_sm_server_collected_at
    ON server_metrics (server_id, collected_at DESC);

SELECT create_hypertable(
    'server_metrics',
    'collected_at',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists       => TRUE
);

ALTER TABLE server_metrics
SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'collected_at DESC',
    timescaledb.compress_segmentby = 'server_id'
);

COMMENT ON TABLE server_metrics IS '服务器时序指标';
COMMENT ON COLUMN server_metrics.server_id IS '服务器 ID';
COMMENT ON COLUMN server_metrics.collected_at IS '采集时间';
COMMENT ON COLUMN server_metrics.reported_at IS '上报时间（原始）';
COMMENT ON COLUMN server_metrics.cpu_usage_ratio IS 'CPU 使用比例（0-1）';
COMMENT ON COLUMN server_metrics.load1 IS '负载（1m）';
COMMENT ON COLUMN server_metrics.load5 IS '负载（5m）';
COMMENT ON COLUMN server_metrics.load15 IS '负载（15m）';
COMMENT ON COLUMN server_metrics.cpu_user IS 'CPU user time';
COMMENT ON COLUMN server_metrics.cpu_system IS 'CPU system time';
COMMENT ON COLUMN server_metrics.cpu_idle IS 'CPU idle time';
COMMENT ON COLUMN server_metrics.cpu_iowait IS 'CPU iowait time';
COMMENT ON COLUMN server_metrics.cpu_steal IS 'CPU steal time';
COMMENT ON COLUMN server_metrics.mem_total IS '内存总量（B）';
COMMENT ON COLUMN server_metrics.mem_used IS '已用内存（B）';
COMMENT ON COLUMN server_metrics.mem_available IS '可用内存（B）';
COMMENT ON COLUMN server_metrics.mem_buffers IS 'Buffers（B）';
COMMENT ON COLUMN server_metrics.mem_cached IS 'Cached（B）';
COMMENT ON COLUMN server_metrics.mem_used_ratio IS '内存使用比例（0-1）';
COMMENT ON COLUMN server_metrics.swap_total IS 'Swap 总量（B）';
COMMENT ON COLUMN server_metrics.swap_used IS '已用 Swap（B）';
COMMENT ON COLUMN server_metrics.swap_free IS '可用 Swap（B）';
COMMENT ON COLUMN server_metrics.swap_used_ratio IS 'Swap 使用比例（0-1）';
COMMENT ON COLUMN server_metrics.net_in_bytes IS '网络入累计（B）';
COMMENT ON COLUMN server_metrics.net_out_bytes IS '网络出累计（B）';
COMMENT ON COLUMN server_metrics.net_in_bps IS '网络入速率（B/s）';
COMMENT ON COLUMN server_metrics.net_out_bps IS '网络出速率（B/s）';
COMMENT ON COLUMN server_metrics.process_count IS '进程数';
COMMENT ON COLUMN server_metrics.tcp_conn IS 'TCP 连接数';
COMMENT ON COLUMN server_metrics.udp_conn IS 'UDP 连接数';
COMMENT ON COLUMN server_metrics.uptime_seconds IS '运行时长（s）';
COMMENT ON COLUMN server_metrics.raid_supported IS '是否支持 RAID';
COMMENT ON COLUMN server_metrics.raid_available IS 'RAID 是否可用';
COMMENT ON COLUMN server_metrics.raid_overall_health IS 'RAID 总体状态';
COMMENT ON COLUMN server_metrics.raid IS 'RAID 详情（JSON）';
COMMENT ON COLUMN server_metrics.disk_smart IS 'SMART 运行时详情（JSON）';
COMMENT ON COLUMN server_metrics.thermal IS '温度传感器运行时详情（JSON）';

-- 默认保留最近 45 天
SELECT add_retention_policy('server_metrics', INTERVAL '45 days', if_not_exists => TRUE);
SELECT add_compression_policy('server_metrics', INTERVAL '7 days', if_not_exists => TRUE);

-- ---------------------------------------------------------
-- 网卡时序指标（每接口一行）
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS nic_metrics (
    server_id                  BIGINT      NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    iface                      VARCHAR(64) NOT NULL,
    collected_at               TIMESTAMPTZ NOT NULL,

    bytes_recv                 BIGINT           NOT NULL DEFAULT 0,
    bytes_sent                 BIGINT           NOT NULL DEFAULT 0,
    recv_rate_bytes_per_sec    DOUBLE PRECISION NOT NULL DEFAULT 0,
    sent_rate_bytes_per_sec    DOUBLE PRECISION NOT NULL DEFAULT 0,

    packets_recv               BIGINT           NOT NULL DEFAULT 0,
    packets_sent               BIGINT           NOT NULL DEFAULT 0,
    recv_rate_packets_per_sec  DOUBLE PRECISION NOT NULL DEFAULT 0,
    sent_rate_packets_per_sec  DOUBLE PRECISION NOT NULL DEFAULT 0,

    err_in                     BIGINT           NOT NULL DEFAULT 0,
    err_out                    BIGINT           NOT NULL DEFAULT 0,
    drop_in                    BIGINT           NOT NULL DEFAULT 0,
    drop_out                   BIGINT           NOT NULL DEFAULT 0,

    extra                      JSONB,

    PRIMARY KEY (server_id, iface, collected_at)
);

SELECT create_hypertable(
    'nic_metrics',
    'collected_at',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists       => TRUE
);

-- 7 天后压缩；按 server_id + iface 分段
ALTER TABLE nic_metrics
SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'collected_at DESC',
    timescaledb.compress_segmentby = 'server_id, iface'
);

SELECT add_compression_policy('nic_metrics', INTERVAL '7 days', if_not_exists => TRUE);
SELECT add_retention_policy('nic_metrics', INTERVAL '45 days', if_not_exists => TRUE);

COMMENT ON TABLE nic_metrics IS '网卡时序指标';
COMMENT ON COLUMN nic_metrics.server_id IS '服务器 ID';
COMMENT ON COLUMN nic_metrics.iface IS '网卡名';
COMMENT ON COLUMN nic_metrics.collected_at IS '采集时间';
COMMENT ON COLUMN nic_metrics.bytes_recv IS '接收累计（B）';
COMMENT ON COLUMN nic_metrics.bytes_sent IS '发送累计（B）';
COMMENT ON COLUMN nic_metrics.recv_rate_bytes_per_sec IS '接收速率（B/s）';
COMMENT ON COLUMN nic_metrics.sent_rate_bytes_per_sec IS '发送速率（B/s）';
COMMENT ON COLUMN nic_metrics.packets_recv IS '接收累计（包）';
COMMENT ON COLUMN nic_metrics.packets_sent IS '发送累计（包）';
COMMENT ON COLUMN nic_metrics.recv_rate_packets_per_sec IS '接收速率（pps）';
COMMENT ON COLUMN nic_metrics.sent_rate_packets_per_sec IS '发送速率（pps）';
COMMENT ON COLUMN nic_metrics.err_in IS '接收错误（包）';
COMMENT ON COLUMN nic_metrics.err_out IS '发送错误（包）';
COMMENT ON COLUMN nic_metrics.drop_in IS '接收丢包';
COMMENT ON COLUMN nic_metrics.drop_out IS '发送丢包';
COMMENT ON COLUMN nic_metrics.extra IS '扩展字段（JSON）';

-- ---------------------------------------------------------
-- 磁盘 IO 时序指标（每设备一行）
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS disk_metrics (
    server_id                 BIGINT       NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    name                      VARCHAR(128) NOT NULL,
    ref                       VARCHAR(128) NOT NULL,
    kind                      VARCHAR(16),
    role                      VARCHAR(16),
    path                      VARCHAR(256),
    collected_at              TIMESTAMPTZ  NOT NULL,

    read_bytes                BIGINT           NOT NULL DEFAULT 0,
    write_bytes               BIGINT           NOT NULL DEFAULT 0,
    read_rate_bytes_per_sec   DOUBLE PRECISION NOT NULL DEFAULT 0,
    write_rate_bytes_per_sec  DOUBLE PRECISION NOT NULL DEFAULT 0,

    iops                      DOUBLE PRECISION NOT NULL DEFAULT 0,
    read_iops                 DOUBLE PRECISION NOT NULL DEFAULT 0,
    write_iops                DOUBLE PRECISION NOT NULL DEFAULT 0,

    util_ratio                DOUBLE PRECISION NOT NULL DEFAULT 0,
    queue_length              DOUBLE PRECISION NOT NULL DEFAULT 0,
    wait_ms                   DOUBLE PRECISION NOT NULL DEFAULT 0,
    service_ms                DOUBLE PRECISION NOT NULL DEFAULT 0,

    PRIMARY KEY (server_id, name, collected_at)
);

SELECT create_hypertable(
    'disk_metrics',
    'collected_at',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists       => TRUE
);

-- 7 天后压缩；按 server_id + name 分段
ALTER TABLE disk_metrics
SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'collected_at DESC',
    timescaledb.compress_segmentby = 'server_id, name'
);

SELECT add_compression_policy('disk_metrics', INTERVAL '7 days', if_not_exists => TRUE);
SELECT add_retention_policy('disk_metrics', INTERVAL '45 days', if_not_exists => TRUE);

COMMENT ON TABLE disk_metrics IS '磁盘 IO 时序指标';
COMMENT ON COLUMN disk_metrics.server_id IS '服务器 ID';
COMMENT ON COLUMN disk_metrics.name IS '设备名/别名';
COMMENT ON COLUMN disk_metrics.ref IS '设备引用';
COMMENT ON COLUMN disk_metrics.kind IS '类型（physical/logical/raid）';
COMMENT ON COLUMN disk_metrics.role IS '角色（primary/secondary）';
COMMENT ON COLUMN disk_metrics.path IS '设备路径';
COMMENT ON COLUMN disk_metrics.collected_at IS '采集时间';
COMMENT ON COLUMN disk_metrics.read_bytes IS '累计读取（B）';
COMMENT ON COLUMN disk_metrics.write_bytes IS '累计写入（B）';
COMMENT ON COLUMN disk_metrics.read_rate_bytes_per_sec IS '读速率（B/s）';
COMMENT ON COLUMN disk_metrics.write_rate_bytes_per_sec IS '写速率（B/s）';
COMMENT ON COLUMN disk_metrics.read_iops IS '读 IOPS';
COMMENT ON COLUMN disk_metrics.write_iops IS '写 IOPS';
COMMENT ON COLUMN disk_metrics.iops IS '总 IOPS';
COMMENT ON COLUMN disk_metrics.util_ratio IS '利用比例（0-1）';
COMMENT ON COLUMN disk_metrics.queue_length IS '队列长度';
COMMENT ON COLUMN disk_metrics.wait_ms IS '平均等待（ms）';
COMMENT ON COLUMN disk_metrics.service_ms IS '平均服务（ms）';

-- ---------------------------------------------------------
-- 磁盘容量时序指标（每逻辑盘一行）
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS disk_usage_metrics (
    server_id     BIGINT       NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    name          VARCHAR(128) NOT NULL,
    ref           VARCHAR(128) NOT NULL,
    kind          VARCHAR(16),
    mountpoint    VARCHAR(256),
    path          VARCHAR(256),
    collected_at  TIMESTAMPTZ  NOT NULL,

    total         BIGINT           NOT NULL DEFAULT 0,
    used          BIGINT           NOT NULL DEFAULT 0,
    free          BIGINT           NOT NULL DEFAULT 0,
    used_ratio    DOUBLE PRECISION NOT NULL DEFAULT 0,

    fs_type       VARCHAR(32),
    devices       TEXT[],

    health        VARCHAR(32),
    level         VARCHAR(32),

    PRIMARY KEY (server_id, name, collected_at)
);

SELECT create_hypertable(
    'disk_usage_metrics',
    'collected_at',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists       => TRUE
);

ALTER TABLE disk_usage_metrics
SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'collected_at DESC',
    timescaledb.compress_segmentby = 'server_id, name'
);

SELECT add_compression_policy('disk_usage_metrics', INTERVAL '7 days', if_not_exists => TRUE);
SELECT add_retention_policy('disk_usage_metrics', INTERVAL '45 days', if_not_exists => TRUE);

COMMENT ON TABLE disk_usage_metrics IS '磁盘容量时序指标';
COMMENT ON COLUMN disk_usage_metrics.server_id IS '服务器 ID';
COMMENT ON COLUMN disk_usage_metrics.name IS '逻辑盘名';
COMMENT ON COLUMN disk_usage_metrics.ref IS '逻辑盘引用';
COMMENT ON COLUMN disk_usage_metrics.kind IS '类型（zfs/raid/lvm/disk）';
COMMENT ON COLUMN disk_usage_metrics.mountpoint IS '挂载点';
COMMENT ON COLUMN disk_usage_metrics.path IS '路径';
COMMENT ON COLUMN disk_usage_metrics.collected_at IS '采集时间';
COMMENT ON COLUMN disk_usage_metrics.total IS '总量（B）';
COMMENT ON COLUMN disk_usage_metrics.used IS '已用（B）';
COMMENT ON COLUMN disk_usage_metrics.free IS '可用（B）';
COMMENT ON COLUMN disk_usage_metrics.used_ratio IS '使用比例（0-1）';
COMMENT ON COLUMN disk_usage_metrics.fs_type IS '文件系统';
COMMENT ON COLUMN disk_usage_metrics.devices IS '设备列表（text[]）';
COMMENT ON COLUMN disk_usage_metrics.health IS '健康状态';
COMMENT ON COLUMN disk_usage_metrics.level IS '阵列级别';

-- ---------------------------------------------------------
-- 设备目录（可选：快速列出网卡/磁盘）
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS server_devices (
    server_id      BIGINT      NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    kind           VARCHAR(8)  NOT NULL, -- 'nic' | 'disk'
    name           VARCHAR(64) NOT NULL, -- iface / device
    first_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    retired_at     TIMESTAMPTZ,
    PRIMARY KEY (server_id, kind, name)
);

CREATE INDEX IF NOT EXISTS idx_server_devices_kind ON server_devices (kind);

COMMENT ON TABLE server_devices IS '服务器设备目录';
COMMENT ON COLUMN server_devices.server_id IS '服务器 ID';
COMMENT ON COLUMN server_devices.kind IS '类型（nic/disk）';
COMMENT ON COLUMN server_devices.name IS '名称';
COMMENT ON COLUMN server_devices.first_seen_at IS '首次出现';
COMMENT ON COLUMN server_devices.last_seen_at IS '最后活跃';
COMMENT ON COLUMN server_devices.retired_at IS '下线时间';

-- ---------------------------------------------------------
-- 服务监控配置
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS services (
    id             BIGSERIAL     PRIMARY KEY,
    name           VARCHAR(128)  NOT NULL,
    group_id       BIGINT        REFERENCES groups (id),

    type           service_type  NOT NULL,
    target         VARCHAR(255)  NOT NULL,
    port           INTEGER,
    region         VARCHAR(64),

    interval_sec   INTEGER       NOT NULL DEFAULT 60,
    timeout_sec    INTEGER       NOT NULL DEFAULT 5,
    retry          SMALLINT      NOT NULL DEFAULT 0,

    http_method    VARCHAR(8)             DEFAULT 'GET',
    http_headers   JSONB,
    http_body      TEXT,
    expect_status  VARCHAR(64),
    expect_keyword VARCHAR(255),

    enabled        BOOLEAN       NOT NULL DEFAULT TRUE,
    is_deleted     BOOLEAN       NOT NULL DEFAULT FALSE,
    remark         VARCHAR(255),

    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_services_type  ON services (type);
CREATE INDEX IF NOT EXISTS idx_services_group ON services (group_id);

COMMENT ON TABLE services IS '服务监控配置';
COMMENT ON COLUMN services.id IS '主键';
COMMENT ON COLUMN services.name IS '名称';
COMMENT ON COLUMN services.group_id IS '所属分组 ID';
COMMENT ON COLUMN services.type IS '类型';
COMMENT ON COLUMN services.target IS '目标（地址/域名）';
COMMENT ON COLUMN services.port IS '端口';
COMMENT ON COLUMN services.region IS '区域/标签';
COMMENT ON COLUMN services.interval_sec IS '探测间隔（s）';
COMMENT ON COLUMN services.timeout_sec IS '超时（s）';
COMMENT ON COLUMN services.retry IS '失败重试次数';
COMMENT ON COLUMN services.http_method IS 'HTTP 方法';
COMMENT ON COLUMN services.http_headers IS 'HTTP 头（JSON）';
COMMENT ON COLUMN services.http_body IS 'HTTP Body';
COMMENT ON COLUMN services.expect_status IS '期望状态码/范围';
COMMENT ON COLUMN services.expect_keyword IS '期望关键字';
COMMENT ON COLUMN services.enabled IS '是否启用';
COMMENT ON COLUMN services.is_deleted IS '删除标记（soft delete）';
COMMENT ON COLUMN services.remark IS '备注';
COMMENT ON COLUMN services.created_at IS '创建时间';
COMMENT ON COLUMN services.updated_at IS '更新时间';

SELECT ensure_updated_at_trigger('services');

-- ---------------------------------------------------------
-- 服务探测结果（Timescale hypertable）
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS service_checks (
    service_id       BIGINT      NOT NULL REFERENCES services (id) ON DELETE CASCADE,
    probe_server_id  BIGINT      REFERENCES servers (id) ON DELETE SET NULL,

    ts              TIMESTAMPTZ  NOT NULL,
    status          SMALLINT     NOT NULL,
    latency_ms      INTEGER,

    http_code       INTEGER,
    result          TEXT,

    PRIMARY KEY (service_id, ts)
);

CREATE INDEX IF NOT EXISTS idx_sc_service_ts ON service_checks (service_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_sc_ts         ON service_checks (ts DESC);

SELECT create_hypertable(
    'service_checks',
    'ts',
    chunk_time_interval => INTERVAL '7 days',
    if_not_exists       => TRUE
);

ALTER TABLE service_checks
SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'ts DESC',
    timescaledb.compress_segmentby = 'service_id'
);

COMMENT ON TABLE service_checks IS '服务探测记录';
COMMENT ON COLUMN service_checks.service_id IS '服务 ID';
COMMENT ON COLUMN service_checks.probe_server_id IS '探测节点（服务器 ID）';
COMMENT ON COLUMN service_checks.ts IS '探测时间';
COMMENT ON COLUMN service_checks.status IS '探测结果码';
COMMENT ON COLUMN service_checks.latency_ms IS '延迟（ms）';
COMMENT ON COLUMN service_checks.http_code IS 'HTTP 状态码';
COMMENT ON COLUMN service_checks.result IS '结果/错误信息';

SELECT add_retention_policy('service_checks', INTERVAL '45 days', if_not_exists => TRUE);
SELECT add_compression_policy('service_checks', INTERVAL '7 days', if_not_exists => TRUE);

-- ---------------------------------------------------------
-- 定时任务配置
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS tasks (
    id           BIGSERIAL    PRIMARY KEY,
    name         VARCHAR(128) NOT NULL,
    type         task_type    NOT NULL DEFAULT 'shell',
    cron_expr    VARCHAR(64)  NOT NULL,
    timeout_sec  INTEGER      NOT NULL DEFAULT 60,
    retries      SMALLINT     NOT NULL DEFAULT 0,

    target_type  target_type  NOT NULL DEFAULT 'all',
    group_id     BIGINT       REFERENCES groups (id),
    server_ids   JSONB,

    payload      TEXT         NOT NULL,
    enabled      BOOLEAN      NOT NULL DEFAULT TRUE,
    is_deleted   BOOLEAN      NOT NULL DEFAULT FALSE,

    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE tasks IS '定时任务';
COMMENT ON COLUMN tasks.id IS '主键';
COMMENT ON COLUMN tasks.name IS '名称';
COMMENT ON COLUMN tasks.type IS '类型';
COMMENT ON COLUMN tasks.cron_expr IS 'Cron 表达式';
COMMENT ON COLUMN tasks.timeout_sec IS '超时（s）';
COMMENT ON COLUMN tasks.retries IS '重试次数';
COMMENT ON COLUMN tasks.target_type IS '目标类型';
COMMENT ON COLUMN tasks.group_id IS '目标分组 ID';
COMMENT ON COLUMN tasks.server_ids IS '目标服务器列表（JSON）';
COMMENT ON COLUMN tasks.payload IS '任务内容';
COMMENT ON COLUMN tasks.enabled IS '是否启用';
COMMENT ON COLUMN tasks.is_deleted IS '删除标记（soft delete）';
COMMENT ON COLUMN tasks.created_at IS '创建时间';
COMMENT ON COLUMN tasks.updated_at IS '更新时间';

SELECT ensure_updated_at_trigger('tasks');

-- ---------------------------------------------------------
-- 定时任务执行日志
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS task_logs (
    id        BIGSERIAL    PRIMARY KEY,
    task_id   BIGINT       NOT NULL REFERENCES tasks (id)   ON DELETE CASCADE,
    server_id BIGINT       NOT NULL REFERENCES servers (id) ON DELETE CASCADE,

    start_at  TIMESTAMPTZ  NOT NULL,
    end_at    TIMESTAMPTZ,
    status    VARCHAR(16)  NOT NULL DEFAULT 'running',
    exit_code INTEGER,
    output    TEXT
);

CREATE INDEX IF NOT EXISTS idx_tl_task   ON task_logs (task_id, start_at DESC);
CREATE INDEX IF NOT EXISTS idx_tl_server ON task_logs (server_id, start_at DESC);

COMMENT ON TABLE task_logs IS '任务执行日志';
COMMENT ON COLUMN task_logs.id IS '主键';
COMMENT ON COLUMN task_logs.task_id IS '任务 ID';
COMMENT ON COLUMN task_logs.server_id IS '执行服务器 ID';
COMMENT ON COLUMN task_logs.start_at IS '开始时间';
COMMENT ON COLUMN task_logs.end_at IS '结束时间';
COMMENT ON COLUMN task_logs.status IS '状态';
COMMENT ON COLUMN task_logs.exit_code IS '退出码';
COMMENT ON COLUMN task_logs.output IS '输出';

-- ---------------------------------------------------------
-- 通知渠道配置
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS notify_channels (
    id         BIGSERIAL    PRIMARY KEY,
    name       VARCHAR(64)  NOT NULL,
    type       notify_type  NOT NULL,
    config     JSONB        NOT NULL,
    enabled    BOOLEAN      NOT NULL DEFAULT TRUE,
    is_deleted BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notify_type ON notify_channels (type);

COMMENT ON TABLE notify_channels IS '通知渠道';
COMMENT ON COLUMN notify_channels.id IS '主键';
COMMENT ON COLUMN notify_channels.name IS '名称';
COMMENT ON COLUMN notify_channels.type IS '类型';
COMMENT ON COLUMN notify_channels.config IS '配置（JSON）';
COMMENT ON COLUMN notify_channels.enabled IS '是否启用';
COMMENT ON COLUMN notify_channels.is_deleted IS '删除标记（soft delete）';
COMMENT ON COLUMN notify_channels.created_at IS '创建时间';
COMMENT ON COLUMN notify_channels.updated_at IS '更新时间';

SELECT ensure_updated_at_trigger('notify_channels');

-- ---------------------------------------------------------
-- 告警规则
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS alert_settings (
    id          BIGSERIAL   PRIMARY KEY,
    scope       VARCHAR(32) NOT NULL DEFAULT 'global',
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    channel_ids JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_alert_settings_scope UNIQUE (scope)
);

COMMENT ON TABLE alert_settings IS '告警设置';
COMMENT ON COLUMN alert_settings.id IS '主键';
COMMENT ON COLUMN alert_settings.scope IS '配置作用域';
COMMENT ON COLUMN alert_settings.enabled IS '是否启用告警';
COMMENT ON COLUMN alert_settings.channel_ids IS '启用的通知渠道 ID（JSON 数组）';
COMMENT ON COLUMN alert_settings.created_at IS '创建时间';
COMMENT ON COLUMN alert_settings.updated_at IS '更新时间';

SELECT ensure_updated_at_trigger('alert_settings');

-- Alert rule semantics are validated in application code (internal/alertspec)
-- so the database schema only keeps structural constraints here.
CREATE TABLE IF NOT EXISTS alert_rules (
    id               BIGSERIAL        PRIMARY KEY,
    name             VARCHAR(128)     NOT NULL,
    enabled          BOOLEAN          NOT NULL DEFAULT TRUE,
    generation       BIGINT           NOT NULL,

    metric           VARCHAR(64)      NOT NULL,
    operator         VARCHAR(8)       NOT NULL,
    threshold        DOUBLE PRECISION NOT NULL,

    duration_sec     INTEGER          NOT NULL,
    cooldown_min     INTEGER          NOT NULL DEFAULT 0,

    threshold_mode   VARCHAR(16)      NOT NULL,
    threshold_offset DOUBLE PRECISION NOT NULL,

    is_deleted       BOOLEAN          NOT NULL DEFAULT FALSE,

    created_at       TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ      NOT NULL DEFAULT now(),

    CONSTRAINT chk_alert_rules_name_not_blank
        CHECK (length(btrim(name)) > 0),
    CONSTRAINT chk_alert_rules_generation_positive
        CHECK (generation > 0),
    CONSTRAINT chk_alert_rules_cooldown_non_negative
        CHECK (cooldown_min >= 0)
);

COMMENT ON TABLE alert_rules IS '告警规则';
COMMENT ON COLUMN alert_rules.id IS '主键';
COMMENT ON COLUMN alert_rules.name IS '名称';
COMMENT ON COLUMN alert_rules.enabled IS '是否启用';
COMMENT ON COLUMN alert_rules.generation IS '规则代数';
COMMENT ON COLUMN alert_rules.metric IS '指标名';
COMMENT ON COLUMN alert_rules.operator IS '比较符';
COMMENT ON COLUMN alert_rules.threshold IS '阈值';
COMMENT ON COLUMN alert_rules.duration_sec IS '持续时间（s）';
COMMENT ON COLUMN alert_rules.cooldown_min IS '冷却时间（min）';
COMMENT ON COLUMN alert_rules.threshold_mode IS '阈值模式';
COMMENT ON COLUMN alert_rules.threshold_offset IS '阈值偏移';
COMMENT ON COLUMN alert_rules.is_deleted IS '删除标记（soft delete）';
COMMENT ON COLUMN alert_rules.created_at IS '创建时间';
COMMENT ON COLUMN alert_rules.updated_at IS '更新时间';

SELECT ensure_updated_at_trigger('alert_rules');

-- 预置规则：多次执行也不炸
INSERT INTO alert_rules (
    id, name, enabled, generation, metric, operator, threshold, duration_sec, cooldown_min, threshold_mode, threshold_offset, is_deleted
) VALUES
    (1, 'cpu_usage_ratio_high',  TRUE, 1, 'cpu.usage_ratio',       '>=', 0.9, 60, 0, 'static',    0, FALSE),
    (2, 'cpu_load1_high',        TRUE, 1, 'cpu.load1',             '>=', 0,   60, 0, 'core_plus', 1, FALSE),
    (3, 'cpu_load5_high',        TRUE, 1, 'cpu.load5',             '>=', 0,   60, 0, 'core_plus', 0, FALSE),
    (4, 'mem_used_ratio_high',   TRUE, 1, 'mem.used_ratio',        '>=', 0.9, 60, 0, 'static',    0, FALSE)
ON CONFLICT (id) DO NOTHING;

-- 序列对齐：避免后续插入撞 ID
SELECT setval(
    pg_get_serial_sequence('alert_rules', 'id'),
    GREATEST((SELECT COALESCE(MAX(id), 0) FROM alert_rules), 4),
    TRUE
);

CREATE TABLE IF NOT EXISTS alert_rule_mounts (
    rule_id    BIGINT      NOT NULL,
    server_id  BIGINT      NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    enabled    BOOLEAN     NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (rule_id, server_id),
    CONSTRAINT chk_alert_rule_mounts_rule_id_nonzero
        CHECK (rule_id <> 0)
);

CREATE INDEX IF NOT EXISTS idx_alert_rule_mounts_server
    ON alert_rule_mounts (server_id);

COMMENT ON TABLE alert_rule_mounts IS '告警规则挂载';
COMMENT ON COLUMN alert_rule_mounts.rule_id IS '规则 ID；内置规则使用负数 ID';
COMMENT ON COLUMN alert_rule_mounts.server_id IS '节点 ID';
COMMENT ON COLUMN alert_rule_mounts.enabled IS '是否挂载';
COMMENT ON COLUMN alert_rule_mounts.created_at IS '创建时间';
COMMENT ON COLUMN alert_rule_mounts.updated_at IS '更新时间';

SELECT ensure_updated_at_trigger('alert_rule_mounts');

-- ---------------------------------------------------------
-- 告警事件
-- ---------------------------------------------------------

CREATE TABLE IF NOT EXISTS alert_events (
    id                  BIGSERIAL     PRIMARY KEY,
    rule_id             BIGINT        NOT NULL,
    rule_generation     BIGINT        NOT NULL,
    rule_snapshot       JSONB         NOT NULL,

    object_type         object_type   NOT NULL,
    object_id           BIGINT        NOT NULL,

    status              alert_status  NOT NULL,

    first_trigger_at    TIMESTAMPTZ   NOT NULL,
    last_trigger_at     TIMESTAMPTZ   NOT NULL,
    closed_at           TIMESTAMPTZ,

    current_value       DOUBLE PRECISION,
    effective_threshold DOUBLE PRECISION,
    close_reason        TEXT,
    title               VARCHAR(255),
    message             TEXT
);

CREATE INDEX IF NOT EXISTS idx_ae_rule        ON alert_events (rule_id);
CREATE INDEX IF NOT EXISTS idx_ae_object      ON alert_events (object_type, object_id);
CREATE INDEX IF NOT EXISTS idx_ae_open_object ON alert_events (object_type, object_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_alert_event_open
    ON alert_events (rule_id, rule_generation, object_type, object_id)
    WHERE status = 'open';

COMMENT ON TABLE alert_events IS '告警事件';
COMMENT ON COLUMN alert_events.id IS '主键';
COMMENT ON COLUMN alert_events.rule_id IS '规则 ID；内置规则使用负数 ID';
COMMENT ON COLUMN alert_events.rule_generation IS '规则代数';
COMMENT ON COLUMN alert_events.rule_snapshot IS '规则快照';
COMMENT ON COLUMN alert_events.object_type IS '对象类型';
COMMENT ON COLUMN alert_events.object_id IS '对象 ID';
COMMENT ON COLUMN alert_events.status IS '状态';
COMMENT ON COLUMN alert_events.first_trigger_at IS '首次触发时间';
COMMENT ON COLUMN alert_events.last_trigger_at IS '最后触发时间';
COMMENT ON COLUMN alert_events.closed_at IS '关闭时间';
COMMENT ON COLUMN alert_events.current_value IS '当前值';
COMMENT ON COLUMN alert_events.effective_threshold IS '触发时生效阈值';
COMMENT ON COLUMN alert_events.close_reason IS '关闭原因';
COMMENT ON COLUMN alert_events.title IS '标题';
COMMENT ON COLUMN alert_events.message IS '内容';

CREATE TABLE IF NOT EXISTS alert_notification_outbox (
    id              BIGSERIAL     PRIMARY KEY,
    event_id        BIGINT        NOT NULL REFERENCES alert_events (id) ON DELETE CASCADE,
    transition      VARCHAR(16)   NOT NULL,
    channel_id      BIGINT        NOT NULL REFERENCES notify_channels (id),
    channel_type    notify_type   NOT NULL,
    payload         JSONB         NOT NULL,
    dedupe_key      VARCHAR(255)  NOT NULL,
    status          VARCHAR(32)   NOT NULL DEFAULT 'pending',
    attempt_count   INTEGER       NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    last_error      TEXT,
    leased_until    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    sent_at         TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_alert_notification_outbox_dedupe
    ON alert_notification_outbox (dedupe_key);

CREATE INDEX IF NOT EXISTS idx_alert_notification_outbox_pending
    ON alert_notification_outbox (status, next_attempt_at);

COMMENT ON TABLE alert_notification_outbox IS '告警通知事务型 outbox';
COMMENT ON COLUMN alert_notification_outbox.event_id IS '关联告警事件';
COMMENT ON COLUMN alert_notification_outbox.transition IS '事件边缘（opened/closed）';
COMMENT ON COLUMN alert_notification_outbox.channel_id IS '通知渠道 ID';
COMMENT ON COLUMN alert_notification_outbox.channel_type IS '通知渠道类型';
COMMENT ON COLUMN alert_notification_outbox.payload IS '通知负载';
COMMENT ON COLUMN alert_notification_outbox.dedupe_key IS '幂等键';
COMMENT ON COLUMN alert_notification_outbox.status IS '发送状态';
COMMENT ON COLUMN alert_notification_outbox.attempt_count IS '尝试次数';
COMMENT ON COLUMN alert_notification_outbox.next_attempt_at IS '下次重试时间';
COMMENT ON COLUMN alert_notification_outbox.last_error IS '最后错误';
COMMENT ON COLUMN alert_notification_outbox.leased_until IS '租约截止时间';
COMMENT ON COLUMN alert_notification_outbox.created_at IS '创建时间';
COMMENT ON COLUMN alert_notification_outbox.sent_at IS '发送完成时间';

CREATE TABLE IF NOT EXISTS alert_control_tasks (
    id            BIGSERIAL     PRIMARY KEY,
    task_type     VARCHAR(32)   NOT NULL,
    dedupe_key    VARCHAR(255)  NOT NULL,
    payload       JSONB         NOT NULL DEFAULT '{}'::jsonb,
    status        VARCHAR(16)   NOT NULL DEFAULT 'pending',
    attempt_count INTEGER       NOT NULL DEFAULT 0,
    available_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    leased_until  TIMESTAMPTZ,
    last_error    TEXT,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_alert_control_tasks_dedupe
    ON alert_control_tasks (dedupe_key);

CREATE INDEX IF NOT EXISTS idx_alert_control_tasks_pending
    ON alert_control_tasks (status, available_at);

COMMENT ON TABLE alert_control_tasks IS '告警控制任务';
COMMENT ON COLUMN alert_control_tasks.task_type IS '任务类型';
COMMENT ON COLUMN alert_control_tasks.dedupe_key IS '幂等键';
COMMENT ON COLUMN alert_control_tasks.payload IS '任务负载';
COMMENT ON COLUMN alert_control_tasks.status IS '任务状态';
COMMENT ON COLUMN alert_control_tasks.attempt_count IS '尝试次数';
COMMENT ON COLUMN alert_control_tasks.available_at IS '可执行时间';
COMMENT ON COLUMN alert_control_tasks.leased_until IS '租约截止时间';
COMMENT ON COLUMN alert_control_tasks.last_error IS '最后错误';
COMMENT ON COLUMN alert_control_tasks.created_at IS '创建时间';
COMMENT ON COLUMN alert_control_tasks.updated_at IS '更新时间';

SELECT ensure_updated_at_trigger('alert_control_tasks');

-- ---------------------------------------------------------
-- Metrics history aggregates and rollups (TimescaleDB)
-- ---------------------------------------------------------

CREATE MATERIALIZED VIEW IF NOT EXISTS server_metrics_15m
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('15 minutes', collected_at) AS bucket,
    server_id,
    avg(cpu_usage_ratio) AS cpu_usage_ratio_avg,
    min(cpu_usage_ratio) AS cpu_usage_ratio_min,
    max(cpu_usage_ratio) AS cpu_usage_ratio_max,
    last(cpu_usage_ratio, collected_at) AS cpu_usage_ratio_last,
    avg(load1) AS load1_avg,
    min(load1) AS load1_min,
    max(load1) AS load1_max,
    last(load1, collected_at) AS load1_last,
    avg(load5) AS load5_avg,
    min(load5) AS load5_min,
    max(load5) AS load5_max,
    last(load5, collected_at) AS load5_last,
    avg(load15) AS load15_avg,
    min(load15) AS load15_min,
    max(load15) AS load15_max,
    last(load15, collected_at) AS load15_last,
    avg(mem_used) AS mem_used_avg,
    min(mem_used) AS mem_used_min,
    max(mem_used) AS mem_used_max,
    last(mem_used, collected_at) AS mem_used_last,
    avg(mem_used_ratio) AS mem_used_ratio_avg,
    min(mem_used_ratio) AS mem_used_ratio_min,
    max(mem_used_ratio) AS mem_used_ratio_max,
    last(mem_used_ratio, collected_at) AS mem_used_ratio_last,
    avg(process_count) AS process_count_avg,
    min(process_count) AS process_count_min,
    max(process_count) AS process_count_max,
    last(process_count, collected_at) AS process_count_last,
    avg(net_in_bps) AS net_in_bps_avg,
    min(net_in_bps) AS net_in_bps_min,
    max(net_in_bps) AS net_in_bps_max,
    last(net_in_bps, collected_at) AS net_in_bps_last,
    avg(net_out_bps) AS net_out_bps_avg,
    min(net_out_bps) AS net_out_bps_min,
    max(net_out_bps) AS net_out_bps_max,
    last(net_out_bps, collected_at) AS net_out_bps_last,
    avg(tcp_conn) AS tcp_conn_avg,
    min(tcp_conn) AS tcp_conn_min,
    max(tcp_conn) AS tcp_conn_max,
    last(tcp_conn, collected_at) AS tcp_conn_last,
    avg(udp_conn) AS udp_conn_avg,
    min(udp_conn) AS udp_conn_min,
    max(udp_conn) AS udp_conn_max,
    last(udp_conn, collected_at) AS udp_conn_last
FROM server_metrics
GROUP BY bucket, server_id
WITH NO DATA;

CREATE MATERIALIZED VIEW IF NOT EXISTS server_online_30m
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('30 minutes', collected_at) AS bucket,
    server_id,
    count(*) AS sample_count
FROM server_metrics
GROUP BY bucket, server_id
WITH NO DATA;

CREATE MATERIALIZED VIEW IF NOT EXISTS disk_metrics_15m
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('15 minutes', collected_at) AS bucket,
    server_id,
    name,
    ref,
    avg(read_rate_bytes_per_sec) AS read_bps_avg,
    min(read_rate_bytes_per_sec) AS read_bps_min,
    max(read_rate_bytes_per_sec) AS read_bps_max,
    last(read_rate_bytes_per_sec, collected_at) AS read_bps_last,
    avg(write_rate_bytes_per_sec) AS write_bps_avg,
    min(write_rate_bytes_per_sec) AS write_bps_min,
    max(write_rate_bytes_per_sec) AS write_bps_max,
    last(write_rate_bytes_per_sec, collected_at) AS write_bps_last,
    avg(read_iops) AS read_iops_avg,
    min(read_iops) AS read_iops_min,
    max(read_iops) AS read_iops_max,
    last(read_iops, collected_at) AS read_iops_last,
    avg(write_iops) AS write_iops_avg,
    min(write_iops) AS write_iops_min,
    max(write_iops) AS write_iops_max,
    last(write_iops, collected_at) AS write_iops_last,
    avg(iops) AS iops_avg,
    min(iops) AS iops_min,
    max(iops) AS iops_max,
    last(iops, collected_at) AS iops_last
FROM disk_metrics
GROUP BY bucket, server_id, name, ref
WITH NO DATA;

CREATE MATERIALIZED VIEW IF NOT EXISTS disk_usage_metrics_15m
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('15 minutes', collected_at) AS bucket,
    server_id,
    name,
    ref,
    mountpoint,
    avg(used) AS used_bytes_avg,
    min(used) AS used_bytes_min,
    max(used) AS used_bytes_max,
    last(used, collected_at) AS used_bytes_last,
    avg(used_ratio) AS used_ratio_avg,
    min(used_ratio) AS used_ratio_min,
    max(used_ratio) AS used_ratio_max,
    last(used_ratio, collected_at) AS used_ratio_last
FROM disk_usage_metrics
GROUP BY bucket, server_id, name, ref, mountpoint
WITH NO DATA;

CREATE MATERIALIZED VIEW IF NOT EXISTS server_metrics_1h
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', collected_at) AS bucket,
    server_id,
    avg(cpu_usage_ratio) AS cpu_usage_ratio_avg,
    min(cpu_usage_ratio) AS cpu_usage_ratio_min,
    max(cpu_usage_ratio) AS cpu_usage_ratio_max,
    last(cpu_usage_ratio, collected_at) AS cpu_usage_ratio_last,
    avg(load1) AS load1_avg,
    min(load1) AS load1_min,
    max(load1) AS load1_max,
    last(load1, collected_at) AS load1_last,
    avg(load5) AS load5_avg,
    min(load5) AS load5_min,
    max(load5) AS load5_max,
    last(load5, collected_at) AS load5_last,
    avg(load15) AS load15_avg,
    min(load15) AS load15_min,
    max(load15) AS load15_max,
    last(load15, collected_at) AS load15_last,
    avg(mem_used) AS mem_used_avg,
    min(mem_used) AS mem_used_min,
    max(mem_used) AS mem_used_max,
    last(mem_used, collected_at) AS mem_used_last,
    avg(mem_used_ratio) AS mem_used_ratio_avg,
    min(mem_used_ratio) AS mem_used_ratio_min,
    max(mem_used_ratio) AS mem_used_ratio_max,
    last(mem_used_ratio, collected_at) AS mem_used_ratio_last,
    avg(process_count) AS process_count_avg,
    min(process_count) AS process_count_min,
    max(process_count) AS process_count_max,
    last(process_count, collected_at) AS process_count_last,
    avg(net_in_bps) AS net_in_bps_avg,
    min(net_in_bps) AS net_in_bps_min,
    max(net_in_bps) AS net_in_bps_max,
    last(net_in_bps, collected_at) AS net_in_bps_last,
    avg(net_out_bps) AS net_out_bps_avg,
    min(net_out_bps) AS net_out_bps_min,
    max(net_out_bps) AS net_out_bps_max,
    last(net_out_bps, collected_at) AS net_out_bps_last,
    avg(tcp_conn) AS tcp_conn_avg,
    min(tcp_conn) AS tcp_conn_min,
    max(tcp_conn) AS tcp_conn_max,
    last(tcp_conn, collected_at) AS tcp_conn_last,
    avg(udp_conn) AS udp_conn_avg,
    min(udp_conn) AS udp_conn_min,
    max(udp_conn) AS udp_conn_max,
    last(udp_conn, collected_at) AS udp_conn_last
FROM server_metrics
GROUP BY bucket, server_id
WITH NO DATA;

CREATE MATERIALIZED VIEW IF NOT EXISTS disk_metrics_1h
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', collected_at) AS bucket,
    server_id,
    name,
    ref,
    avg(read_rate_bytes_per_sec) AS read_bps_avg,
    min(read_rate_bytes_per_sec) AS read_bps_min,
    max(read_rate_bytes_per_sec) AS read_bps_max,
    last(read_rate_bytes_per_sec, collected_at) AS read_bps_last,
    avg(write_rate_bytes_per_sec) AS write_bps_avg,
    min(write_rate_bytes_per_sec) AS write_bps_min,
    max(write_rate_bytes_per_sec) AS write_bps_max,
    last(write_rate_bytes_per_sec, collected_at) AS write_bps_last,
    avg(read_iops) AS read_iops_avg,
    min(read_iops) AS read_iops_min,
    max(read_iops) AS read_iops_max,
    last(read_iops, collected_at) AS read_iops_last,
    avg(write_iops) AS write_iops_avg,
    min(write_iops) AS write_iops_min,
    max(write_iops) AS write_iops_max,
    last(write_iops, collected_at) AS write_iops_last,
    avg(iops) AS iops_avg,
    min(iops) AS iops_min,
    max(iops) AS iops_max,
    last(iops, collected_at) AS iops_last
FROM disk_metrics
GROUP BY bucket, server_id, name, ref
WITH NO DATA;

CREATE MATERIALIZED VIEW IF NOT EXISTS disk_usage_metrics_1h
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', collected_at) AS bucket,
    server_id,
    name,
    ref,
    mountpoint,
    avg(used) AS used_bytes_avg,
    min(used) AS used_bytes_min,
    max(used) AS used_bytes_max,
    last(used, collected_at) AS used_bytes_last,
    avg(used_ratio) AS used_ratio_avg,
    min(used_ratio) AS used_ratio_min,
    max(used_ratio) AS used_ratio_max,
    last(used_ratio, collected_at) AS used_ratio_last
FROM disk_usage_metrics
GROUP BY bucket, server_id, name, ref, mountpoint
WITH NO DATA;

SELECT add_continuous_aggregate_policy('server_metrics_15m',
    start_offset => INTERVAL '31 days',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes',
    if_not_exists => TRUE);

SELECT add_continuous_aggregate_policy('server_online_30m',
    start_offset => INTERVAL '31 days',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes',
    if_not_exists => TRUE);

SELECT add_continuous_aggregate_policy('disk_metrics_15m',
    start_offset => INTERVAL '31 days',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes',
    if_not_exists => TRUE);

SELECT add_continuous_aggregate_policy('disk_usage_metrics_15m',
    start_offset => INTERVAL '31 days',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes',
    if_not_exists => TRUE);

SELECT add_continuous_aggregate_policy('server_metrics_1h',
    start_offset => INTERVAL '31 days',
    end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes',
    if_not_exists => TRUE);

SELECT add_continuous_aggregate_policy('disk_metrics_1h',
    start_offset => INTERVAL '31 days',
    end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes',
    if_not_exists => TRUE);

SELECT add_continuous_aggregate_policy('disk_usage_metrics_1h',
    start_offset => INTERVAL '31 days',
    end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes',
    if_not_exists => TRUE);

-- ---------------------------------------------------------
-- Runtime settings
-- ---------------------------------------------------------

CREATE TABLE system_settings (
    id              SMALLINT     PRIMARY KEY DEFAULT 1,
    active_theme_id VARCHAR(64)  NOT NULL DEFAULT '',
    logo_url        TEXT         NOT NULL DEFAULT '/brandlogo.svg',
    page_title      TEXT         NOT NULL DEFAULT 'Ithiltir Monitor Dashboard',
    topbar_text     TEXT         NOT NULL DEFAULT 'Ithiltir Control',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_system_settings_singleton CHECK (id = 1)
);

INSERT INTO system_settings (
    id,
    active_theme_id,
    logo_url,
    page_title,
    topbar_text
) VALUES (
    1,
    '',
    '/brandlogo.svg',
    'Ithiltir Monitor Dashboard',
    'Ithiltir Control'
);

CREATE TABLE traffic_settings (
    id                  SMALLINT     PRIMARY KEY DEFAULT 1,
    guest_access_mode   VARCHAR(16)  NOT NULL DEFAULT 'disabled',
    usage_mode          VARCHAR(16)  NOT NULL DEFAULT 'lite',
    cycle_mode          VARCHAR(32)  NOT NULL DEFAULT 'calendar_month',
    billing_start_day   SMALLINT     NOT NULL DEFAULT 1,
    billing_anchor_date VARCHAR(10)  NOT NULL DEFAULT '',
    billing_timezone    VARCHAR(64)  NOT NULL DEFAULT '',
    direction_mode      VARCHAR(16)  NOT NULL DEFAULT 'out',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_traffic_settings_singleton CHECK (id = 1),
    CONSTRAINT chk_traffic_settings_guest_access CHECK (guest_access_mode IN ('disabled', 'by_node')),
    CONSTRAINT chk_traffic_settings_usage CHECK (usage_mode IN ('lite', 'billing')),
    CONSTRAINT chk_traffic_settings_cycle CHECK (cycle_mode IN ('calendar_month', 'whmcs_compatible', 'clamp_to_month_end')),
    CONSTRAINT chk_traffic_settings_billing_start_day CHECK (billing_start_day BETWEEN 1 AND 31),
    CONSTRAINT chk_traffic_settings_direction CHECK (direction_mode IN ('in', 'out', 'split'))
);

INSERT INTO traffic_settings (
    id,
    guest_access_mode,
    usage_mode,
    cycle_mode,
    billing_start_day,
    billing_anchor_date,
    billing_timezone,
    direction_mode
) VALUES (
    1,
    'disabled',
    'lite',
    'calendar_month',
    1,
    '',
    '',
    'out'
);

COMMENT ON TABLE system_settings IS '运行时系统设置';
COMMENT ON COLUMN system_settings.id IS '固定单行ID';
COMMENT ON COLUMN system_settings.active_theme_id IS '当前启用主题ID';
COMMENT ON COLUMN system_settings.logo_url IS '站点Logo URL';
COMMENT ON COLUMN system_settings.page_title IS '页面标题';
COMMENT ON COLUMN system_settings.topbar_text IS '管理端顶部栏文本';
COMMENT ON COLUMN system_settings.created_at IS '创建时间';
COMMENT ON COLUMN system_settings.updated_at IS '更新时间';

SELECT ensure_updated_at_trigger('system_settings');

CREATE TABLE IF NOT EXISTS metric_settings (
    id                        SMALLINT     PRIMARY KEY DEFAULT 1,
    history_guest_access_mode VARCHAR(16)  NOT NULL DEFAULT 'disabled',
    created_at                TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_metric_settings_singleton CHECK (id = 1),
    CONSTRAINT chk_metric_settings_history_guest_access
        CHECK (history_guest_access_mode IN ('disabled', 'by_node'))
);

COMMENT ON TABLE metric_settings IS '指标历史设置';
COMMENT ON COLUMN metric_settings.id IS '固定单行ID';
COMMENT ON COLUMN metric_settings.history_guest_access_mode IS '访客历史指标访问模式';
COMMENT ON COLUMN metric_settings.created_at IS '创建时间';
COMMENT ON COLUMN metric_settings.updated_at IS '更新时间';

SELECT ensure_updated_at_trigger('metric_settings');

COMMENT ON TABLE traffic_settings IS '流量统计设置';
COMMENT ON COLUMN traffic_settings.id IS '固定单行ID';
COMMENT ON COLUMN traffic_settings.guest_access_mode IS '访客流量访问模式';
COMMENT ON COLUMN traffic_settings.usage_mode IS '流量用量模式';
COMMENT ON COLUMN traffic_settings.cycle_mode IS '账期模式';
COMMENT ON COLUMN traffic_settings.billing_start_day IS '账期起始日';
COMMENT ON COLUMN traffic_settings.billing_anchor_date IS 'WHMCS兼容账期锚点日期';
COMMENT ON COLUMN traffic_settings.billing_timezone IS '账期时区';
COMMENT ON COLUMN traffic_settings.direction_mode IS '流量方向统计模式';
COMMENT ON COLUMN traffic_settings.created_at IS '创建时间';
COMMENT ON COLUMN traffic_settings.updated_at IS '更新时间';

SELECT ensure_updated_at_trigger('traffic_settings');

CREATE TABLE IF NOT EXISTS traffic_month_usage (
    server_id                  BIGINT       NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    iface                      VARCHAR(64)  NOT NULL,
    cycle_mode                 VARCHAR(32)  NOT NULL,
    billing_start_day          SMALLINT     NOT NULL,
    timezone                   VARCHAR(64)  NOT NULL,
    cycle_start                TIMESTAMPTZ  NOT NULL,
    cycle_end                  TIMESTAMPTZ  NOT NULL,
    covered_until              TIMESTAMPTZ  NOT NULL,
    last_collected_at          TIMESTAMPTZ  NOT NULL,

    in_bytes                   BIGINT       NOT NULL DEFAULT 0,
    out_bytes                  BIGINT       NOT NULL DEFAULT 0,
    in_peak_bytes_per_sec      DOUBLE PRECISION NOT NULL DEFAULT 0,
    out_peak_bytes_per_sec     DOUBLE PRECISION NOT NULL DEFAULT 0,
    sample_count               INTEGER      NOT NULL DEFAULT 0,
    gap_count                  INTEGER      NOT NULL DEFAULT 0,
    reset_count                INTEGER      NOT NULL DEFAULT 0,

    created_at                 TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ  NOT NULL DEFAULT now(),

    PRIMARY KEY (server_id, iface, cycle_mode, billing_start_day, cycle_start, cycle_end)
);

CREATE INDEX IF NOT EXISTS idx_traffic_month_usage_server_cycle
    ON traffic_month_usage (server_id, cycle_start DESC);

COMMENT ON TABLE traffic_month_usage IS '轻量月度网卡用量统计表';
COMMENT ON COLUMN traffic_month_usage.iface IS '网卡名；不持久化 all 聚合行';
COMMENT ON COLUMN traffic_month_usage.covered_until IS '统计覆盖到的时间';
COMMENT ON COLUMN traffic_month_usage.last_collected_at IS '最后处理的原始采样时间';
COMMENT ON COLUMN traffic_month_usage.in_bytes IS '入站字节数';
COMMENT ON COLUMN traffic_month_usage.out_bytes IS '出站字节数';
COMMENT ON COLUMN traffic_month_usage.in_peak_bytes_per_sec IS '入站估算峰值速率（B/s）';
COMMENT ON COLUMN traffic_month_usage.out_peak_bytes_per_sec IS '出站估算峰值速率（B/s）';

SELECT ensure_updated_at_trigger('traffic_month_usage');

CREATE TABLE IF NOT EXISTS traffic_5m (
    server_id                  BIGINT       NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    iface                      VARCHAR(64)  NOT NULL,
    bucket                     TIMESTAMPTZ  NOT NULL,

    in_bytes                   BIGINT       NOT NULL DEFAULT 0,
    out_bytes                  BIGINT       NOT NULL DEFAULT 0,
    covered_seconds            DOUBLE PRECISION NOT NULL DEFAULT 0,

    in_rate_bytes_per_sec      DOUBLE PRECISION NOT NULL DEFAULT 0,
    out_rate_bytes_per_sec     DOUBLE PRECISION NOT NULL DEFAULT 0,
    in_peak_bytes_per_sec      DOUBLE PRECISION NOT NULL DEFAULT 0,
    out_peak_bytes_per_sec     DOUBLE PRECISION NOT NULL DEFAULT 0,

    sample_count               INTEGER      NOT NULL DEFAULT 0,
    gap_count                  INTEGER      NOT NULL DEFAULT 0,
    reset_count                INTEGER      NOT NULL DEFAULT 0,

    created_at                 TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ  NOT NULL DEFAULT now(),

    PRIMARY KEY (server_id, iface, bucket)
);

SELECT create_hypertable(
    'traffic_5m',
    'bucket',
    chunk_time_interval => INTERVAL '7 days',
    if_not_exists       => TRUE
);

ALTER TABLE traffic_5m
SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'bucket DESC',
    timescaledb.compress_segmentby = 'server_id, iface'
);

SELECT add_compression_policy('traffic_5m', INTERVAL '30 days', if_not_exists => TRUE);
SELECT add_retention_policy('traffic_5m', INTERVAL '45 days', if_not_exists => TRUE);

COMMENT ON TABLE traffic_5m IS '5分钟网卡流量统计事实表';
COMMENT ON COLUMN traffic_5m.server_id IS '服务器 ID';
COMMENT ON COLUMN traffic_5m.iface IS '网卡名';
COMMENT ON COLUMN traffic_5m.bucket IS '5分钟时间桶';
COMMENT ON COLUMN traffic_5m.in_bytes IS '入站字节数';
COMMENT ON COLUMN traffic_5m.out_bytes IS '出站字节数';
COMMENT ON COLUMN traffic_5m.covered_seconds IS '有效差值覆盖秒数';
COMMENT ON COLUMN traffic_5m.in_rate_bytes_per_sec IS '入站平均速率（B/s）';
COMMENT ON COLUMN traffic_5m.out_rate_bytes_per_sec IS '出站平均速率（B/s）';
COMMENT ON COLUMN traffic_5m.in_peak_bytes_per_sec IS '入站峰值速率（B/s）';
COMMENT ON COLUMN traffic_5m.out_peak_bytes_per_sec IS '出站峰值速率（B/s）';
COMMENT ON COLUMN traffic_5m.sample_count IS '有效样本数';
COMMENT ON COLUMN traffic_5m.gap_count IS '疑似缺口数量';
COMMENT ON COLUMN traffic_5m.reset_count IS '计数器重置数量';

SELECT ensure_updated_at_trigger('traffic_5m');

CREATE TABLE IF NOT EXISTS traffic_monthly (
    server_id                     BIGINT       NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    iface                         VARCHAR(64)  NOT NULL,
    cycle_mode                    VARCHAR(32)  NOT NULL,
    billing_start_day             SMALLINT     NOT NULL,
    timezone                      VARCHAR(64)  NOT NULL,
    cycle_start                   TIMESTAMPTZ  NOT NULL,
    cycle_end                     TIMESTAMPTZ  NOT NULL,
    status                        VARCHAR(16)  NOT NULL DEFAULT 'sealed',
    effective_start               TIMESTAMPTZ  NOT NULL,
    effective_end                 TIMESTAMPTZ  NOT NULL,
    covered_until                 TIMESTAMPTZ  NOT NULL,
    generated_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    sealed_at                     TIMESTAMPTZ,

    in_bytes                      BIGINT       NOT NULL DEFAULT 0,
    out_bytes                     BIGINT       NOT NULL DEFAULT 0,
    p95_enabled                   BOOLEAN      NOT NULL DEFAULT FALSE,
    in_p95_bytes_per_sec          DOUBLE PRECISION NOT NULL DEFAULT 0,
    out_p95_bytes_per_sec         DOUBLE PRECISION NOT NULL DEFAULT 0,
    in_peak_bytes_per_sec         DOUBLE PRECISION NOT NULL DEFAULT 0,
    out_peak_bytes_per_sec        DOUBLE PRECISION NOT NULL DEFAULT 0,

    sample_count                  INTEGER      NOT NULL DEFAULT 0,
    expected_sample_count         INTEGER      NOT NULL DEFAULT 0,
    coverage_ratio                DOUBLE PRECISION NOT NULL DEFAULT 0,
    gap_count                     INTEGER      NOT NULL DEFAULT 0,
    reset_count                   INTEGER      NOT NULL DEFAULT 0,

    created_at                    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                    TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT traffic_monthly_status_check CHECK (status IN ('grace', 'sealed', 'stale')),
    PRIMARY KEY (server_id, iface, cycle_mode, billing_start_day, cycle_start, cycle_end)
);

CREATE INDEX IF NOT EXISTS idx_traffic_monthly_server_cycle
    ON traffic_monthly (server_id, cycle_start DESC);

COMMENT ON TABLE traffic_monthly IS '月度网卡流量统计快照';
COMMENT ON COLUMN traffic_monthly.iface IS '网卡名';
COMMENT ON COLUMN traffic_monthly.cycle_mode IS '账期模式';
COMMENT ON COLUMN traffic_monthly.billing_start_day IS '月度起始日';
COMMENT ON COLUMN traffic_monthly.timezone IS '账期时区';
COMMENT ON COLUMN traffic_monthly.cycle_start IS '账期开始';
COMMENT ON COLUMN traffic_monthly.cycle_end IS '账期结束';
COMMENT ON COLUMN traffic_monthly.status IS '快照状态：grace / sealed / stale';
COMMENT ON COLUMN traffic_monthly.effective_start IS '有效统计窗口开始';
COMMENT ON COLUMN traffic_monthly.effective_end IS '有效统计窗口结束';
COMMENT ON COLUMN traffic_monthly.covered_until IS '统计覆盖到的时间';
COMMENT ON COLUMN traffic_monthly.generated_at IS '快照生成时间';
COMMENT ON COLUMN traffic_monthly.sealed_at IS '快照封存时间';
COMMENT ON COLUMN traffic_monthly.p95_enabled IS '快照是否包含95带宽';
COMMENT ON COLUMN traffic_monthly.in_p95_bytes_per_sec IS '入站95带宽（B/s）';
COMMENT ON COLUMN traffic_monthly.out_p95_bytes_per_sec IS '出站95带宽（B/s）';

SELECT ensure_updated_at_trigger('traffic_monthly');

-- Current metrics projection for fast frontend cache rebuild.
-- History remains the source of time-series truth; these tables store only the latest accepted sample.

CREATE TABLE IF NOT EXISTS server_current_metrics (
    server_id            BIGINT      PRIMARY KEY REFERENCES servers (id) ON DELETE CASCADE,
    collected_at         TIMESTAMPTZ NOT NULL,
    reported_at          TIMESTAMPTZ,

    cpu_usage_ratio      DOUBLE PRECISION NOT NULL DEFAULT 0,
    load1                DOUBLE PRECISION NOT NULL DEFAULT 0,
    load5                DOUBLE PRECISION NOT NULL DEFAULT 0,
    load15               DOUBLE PRECISION NOT NULL DEFAULT 0,

    cpu_user             DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_system           DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_idle             DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_iowait           DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_steal            DOUBLE PRECISION NOT NULL DEFAULT 0,

    mem_total            BIGINT           NOT NULL DEFAULT 0,
    mem_used             BIGINT           NOT NULL DEFAULT 0,
    mem_available        BIGINT           NOT NULL DEFAULT 0,
    mem_buffers          BIGINT           NOT NULL DEFAULT 0,
    mem_cached           BIGINT           NOT NULL DEFAULT 0,
    mem_used_ratio       DOUBLE PRECISION NOT NULL DEFAULT 0,

    swap_total           BIGINT           NOT NULL DEFAULT 0,
    swap_used            BIGINT           NOT NULL DEFAULT 0,
    swap_free            BIGINT           NOT NULL DEFAULT 0,
    swap_used_ratio      DOUBLE PRECISION NOT NULL DEFAULT 0,

    net_in_bytes         BIGINT           NOT NULL DEFAULT 0,
    net_out_bytes        BIGINT           NOT NULL DEFAULT 0,
    net_in_bps           DOUBLE PRECISION NOT NULL DEFAULT 0,
    net_out_bps          DOUBLE PRECISION NOT NULL DEFAULT 0,

    process_count        INTEGER          NOT NULL DEFAULT 0,
    tcp_conn             INTEGER          NOT NULL DEFAULT 0,
    udp_conn             INTEGER          NOT NULL DEFAULT 0,

    uptime_seconds       BIGINT           NOT NULL DEFAULT 0,

    raid_supported       BOOLEAN          NOT NULL DEFAULT FALSE,
    raid_available       BOOLEAN          NOT NULL DEFAULT FALSE,
    raid_overall_health  VARCHAR(16)      NOT NULL DEFAULT '',

    raid                 JSONB,
    disk_smart           JSONB,
    thermal              JSONB,

    created_at           TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ      NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_server_current_metrics_collected_at
    ON server_current_metrics (collected_at DESC);

COMMENT ON TABLE server_current_metrics IS '服务器当前指标投影';
COMMENT ON COLUMN server_current_metrics.server_id IS '服务器 ID';
COMMENT ON COLUMN server_current_metrics.collected_at IS '当前态对应的历史采集时间';
COMMENT ON COLUMN server_current_metrics.reported_at IS '上报时间（原始）';
COMMENT ON COLUMN server_current_metrics.disk_smart IS '当前 SMART 运行时详情（JSON）';
COMMENT ON COLUMN server_current_metrics.thermal IS '当前温度传感器运行时详情（JSON）';

SELECT ensure_updated_at_trigger('server_current_metrics');

CREATE TABLE IF NOT EXISTS server_current_disk_metrics (
    server_id                 BIGINT       NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    name                      VARCHAR(128) NOT NULL,
    ref                       VARCHAR(128) NOT NULL DEFAULT '',
    kind                      VARCHAR(16)  NOT NULL DEFAULT '',
    role                      VARCHAR(16)  NOT NULL DEFAULT '',
    path                      VARCHAR(256) NOT NULL DEFAULT '',
    collected_at              TIMESTAMPTZ  NOT NULL,

    read_bytes                BIGINT           NOT NULL DEFAULT 0,
    write_bytes               BIGINT           NOT NULL DEFAULT 0,
    read_rate_bytes_per_sec   DOUBLE PRECISION NOT NULL DEFAULT 0,
    write_rate_bytes_per_sec  DOUBLE PRECISION NOT NULL DEFAULT 0,

    iops                      DOUBLE PRECISION NOT NULL DEFAULT 0,
    read_iops                 DOUBLE PRECISION NOT NULL DEFAULT 0,
    write_iops                DOUBLE PRECISION NOT NULL DEFAULT 0,

    util_ratio                DOUBLE PRECISION NOT NULL DEFAULT 0,
    queue_length              DOUBLE PRECISION NOT NULL DEFAULT 0,
    wait_ms                   DOUBLE PRECISION NOT NULL DEFAULT 0,
    service_ms                DOUBLE PRECISION NOT NULL DEFAULT 0,

    created_at                TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ      NOT NULL DEFAULT now(),

    PRIMARY KEY (server_id, name)
);

COMMENT ON TABLE server_current_disk_metrics IS '磁盘 IO 当前指标投影';
COMMENT ON COLUMN server_current_disk_metrics.collected_at IS '当前态对应的历史采集时间';

SELECT ensure_updated_at_trigger('server_current_disk_metrics');

CREATE TABLE IF NOT EXISTS server_current_disk_usage_metrics (
    server_id     BIGINT       NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    name          VARCHAR(128) NOT NULL,
    ref           VARCHAR(128) NOT NULL DEFAULT '',
    kind          VARCHAR(16)  NOT NULL DEFAULT '',
    mountpoint    VARCHAR(256) NOT NULL DEFAULT '',
    path          VARCHAR(256) NOT NULL DEFAULT '',
    collected_at  TIMESTAMPTZ  NOT NULL,

    total         BIGINT           NOT NULL DEFAULT 0,
    used          BIGINT           NOT NULL DEFAULT 0,
    free          BIGINT           NOT NULL DEFAULT 0,
    used_ratio    DOUBLE PRECISION NOT NULL DEFAULT 0,

    fs_type       VARCHAR(32)      NOT NULL DEFAULT '',
    devices       TEXT[],

    health        VARCHAR(32)      NOT NULL DEFAULT '',
    level         VARCHAR(32)      NOT NULL DEFAULT '',

    created_at    TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ      NOT NULL DEFAULT now(),

    PRIMARY KEY (server_id, name)
);

COMMENT ON TABLE server_current_disk_usage_metrics IS '磁盘容量当前指标投影';
COMMENT ON COLUMN server_current_disk_usage_metrics.collected_at IS '当前态对应的历史采集时间';

SELECT ensure_updated_at_trigger('server_current_disk_usage_metrics');

CREATE TABLE IF NOT EXISTS server_current_nic_metrics (
    server_id                  BIGINT      NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    iface                      VARCHAR(64) NOT NULL,
    collected_at               TIMESTAMPTZ NOT NULL,

    bytes_recv                 BIGINT           NOT NULL DEFAULT 0,
    bytes_sent                 BIGINT           NOT NULL DEFAULT 0,
    recv_rate_bytes_per_sec    DOUBLE PRECISION NOT NULL DEFAULT 0,
    sent_rate_bytes_per_sec    DOUBLE PRECISION NOT NULL DEFAULT 0,

    packets_recv               BIGINT           NOT NULL DEFAULT 0,
    packets_sent               BIGINT           NOT NULL DEFAULT 0,
    recv_rate_packets_per_sec  DOUBLE PRECISION NOT NULL DEFAULT 0,
    sent_rate_packets_per_sec  DOUBLE PRECISION NOT NULL DEFAULT 0,

    err_in                     BIGINT           NOT NULL DEFAULT 0,
    err_out                    BIGINT           NOT NULL DEFAULT 0,
    drop_in                    BIGINT           NOT NULL DEFAULT 0,
    drop_out                   BIGINT           NOT NULL DEFAULT 0,

    extra                      JSONB,

    created_at                 TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ      NOT NULL DEFAULT now(),

    PRIMARY KEY (server_id, iface)
);

COMMENT ON TABLE server_current_nic_metrics IS '网卡当前指标投影';
COMMENT ON COLUMN server_current_nic_metrics.collected_at IS '当前态对应的历史采集时间';

SELECT ensure_updated_at_trigger('server_current_nic_metrics');

-- +goose StatementEnd
