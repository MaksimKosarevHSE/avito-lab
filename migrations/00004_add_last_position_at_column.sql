-- +goose Up
ALTER TABLE trips ADD COLUMN last_position_at TIMESTAMPTZ;

CREATE INDEX trips_last_tracked_at_index ON trips (
    (COALESCE(last_position_at, started_at)), id
    ) WHERE status = 'active';

-- +goose Down
ALTER TABLE trips DROP COLUMN last_position_at;
