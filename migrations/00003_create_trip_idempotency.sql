-- +goose Up
CREATE TABLE trip_create_idempotency (
    idempotency_key     UUID PRIMARY KEY,
    fingerprint_version SMALLINT NOT NULL CHECK (fingerprint_version > 0),
    request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint) = 32),
    trip_id             UUID NOT NULL UNIQUE,
    created_at          TIMESTAMPTZ NOT NULL,
    expires_at          TIMESTAMPTZ NOT NULL,

    CONSTRAINT trip_create_idempotency_expiry_check
        CHECK (expires_at > created_at),
    CONSTRAINT trip_create_idempotency_trip_fk
        FOREIGN KEY (trip_id) REFERENCES trips(id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX trip_create_idempotency_expires_at_idx
    ON trip_create_idempotency (expires_at);

-- +goose Down
DROP TABLE trip_create_idempotency;
