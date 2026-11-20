-- +goose Up
ALTER TABLE trips ADD COLUMN last_position_at TIMESTAMPTZ;

CREATE INDEX trips_last_position_at_index ON trips(last_position_at) WHERE status = 'active';

-- +goose Down
ALTER TABLE trips DROP COLUMN last_position_at;
