package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
)

const deleteExpiredIdempotencyKeysQuery = `
WITH expired AS (
    SELECT ` + idempotencyKeyColumn + `
    FROM ` + tripCreateIdempotencyTable + `
    WHERE ` + expiresAtColumn + ` <= statement_timestamp()
    ORDER BY ` + expiresAtColumn + `, ` + idempotencyKeyColumn + `
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
DELETE FROM ` + tripCreateIdempotencyTable + ` AS records
USING expired
WHERE records.` + idempotencyKeyColumn + ` = expired.` + idempotencyKeyColumn

const claimIdempotencyKeySuffix = `ON CONFLICT (` + idempotencyKeyColumn + `) DO UPDATE SET
` + fingerprintVersionColumn + ` = EXCLUDED.` + fingerprintVersionColumn + `,
` + requestFingerprintColumn + ` = EXCLUDED.` + requestFingerprintColumn + `,
` + tripIDColumn + ` = EXCLUDED.` + tripIDColumn + `,
` + createdAtColumn + ` = EXCLUDED.` + createdAtColumn + `,
` + expiresAtColumn + ` = EXCLUDED.` + expiresAtColumn + `
WHERE ` + tripCreateIdempotencyTable + `.` + expiresAtColumn + ` <= statement_timestamp()
RETURNING ` + tripIDColumn

func (r *Repository) DeleteExpiredIdempotencyKeys(
	ctx context.Context,
	limit int,
) (int64, error) {
	if limit <= 0 {
		return 0, errors.New("idempotency cleanup limit must be positive")
	}

	var rowsAffected int64
	err := r.db.Execute(ctx, func(queryCtx context.Context, executor postgres.Executor) error {
		commandTag, execErr := executor.Exec(
			queryCtx,
			deleteExpiredIdempotencyKeysQuery,
			limit,
		)
		if execErr != nil {
			return execErr
		}

		rowsAffected = commandTag.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("delete expired idempotency keys: %w", err)
	}

	return rowsAffected, nil
}

func (r *Repository) ClaimIdempotencyKey(
	ctx context.Context,
	record domaintrip.IdempotencyRecord,
	ttl time.Duration,
) (bool, error) {
	query, args, err := r.builder.
		Insert(tripCreateIdempotencyTable).
		Columns(idempotencyColumns...).
		Values(
			record.Key,
			record.FingerprintVersion,
			record.RequestFingerprint,
			record.TripID,
			sq.Expr("statement_timestamp()"),
			sq.Expr(
				"statement_timestamp() + (? * interval '1 second')",
				ttl.Seconds(),
			),
		).
		Suffix(claimIdempotencyKeySuffix).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build claim idempotency key query: %w", err)
	}

	var tripID uuid.UUID
	err = r.db.Execute(ctx, func(queryCtx context.Context, executor postgres.Executor) error {
		return executor.QueryRow(queryCtx, query, args...).Scan(&tripID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim idempotency key: %w", err)
	}
	if tripID != record.TripID {
		return false, errors.New("claimed idempotency key returned an unexpected trip ID")
	}

	return true, nil
}

func (r *Repository) GetIdempotencyKey(
	ctx context.Context,
	key uuid.UUID,
) (domaintrip.IdempotencyRecord, error) {
	query, args, err := r.builder.
		Select(idempotencyResultColumns...).
		From(tripCreateIdempotencyTable).
		Where(sq.Eq{idempotencyKeyColumn: key}).
		ToSql()
	if err != nil {
		return domaintrip.IdempotencyRecord{}, fmt.Errorf("build get idempotency key query: %w", err)
	}

	var record domaintrip.IdempotencyRecord
	err = r.db.Execute(ctx, func(queryCtx context.Context, executor postgres.Executor) error {
		return executor.QueryRow(queryCtx, query, args...).Scan(
			&record.Key,
			&record.FingerprintVersion,
			&record.RequestFingerprint,
			&record.TripID,
		)
	})
	if err != nil {
		return domaintrip.IdempotencyRecord{}, fmt.Errorf("get idempotency key: %w", err)
	}

	return record, nil
}
