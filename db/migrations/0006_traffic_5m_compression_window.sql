-- +goose Up

-- Keep 5-minute traffic facts in rowstore form; retention policy handles rolling deletion.

SELECT remove_compression_policy('traffic_5m', if_exists => TRUE);

SELECT decompress_chunk(c, true)
FROM show_chunks('traffic_5m') c;

ALTER TABLE traffic_5m SET (timescaledb.compress = false);
