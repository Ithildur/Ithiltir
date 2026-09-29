-- +goose Up
CREATE TABLE server_virt (
    server_id BIGINT PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
    collected_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    snapshot JSONB NOT NULL
);
COMMENT ON TABLE server_virt IS 'Latest VM observation per host; independent of host metrics and uptime';

-- +goose Down
DROP TABLE server_virt;
