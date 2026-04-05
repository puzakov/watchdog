-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS metrics (
    id         TEXT        NOT NULL,
    mtype      TEXT        NOT NULL,
    delta      BIGINT,
    value      DOUBLE PRECISION,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, mtype),
    CONSTRAINT metrics_mtype_check CHECK (mtype IN ('gauge', 'counter')),
    CONSTRAINT metrics_value_check CHECK (
        (mtype = 'gauge' AND value IS NOT NULL AND delta IS NULL) OR
        (mtype = 'counter' AND delta IS NOT NULL AND value IS NULL)
    )
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS metrics;
-- +goose StatementEnd

