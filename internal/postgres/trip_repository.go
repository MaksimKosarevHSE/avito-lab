package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MaksimKosarevHSE/avito-lab/internal/trip"
	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const activeTripPerDriverConstraint = "trips_one_active_per_driver_idx"
const tripCreateIdempotencyTable = "trip_create_idempotency"

var tripColumns = []string{
	"id",
	"user_id",
	"driver_id",
	"start_latitude",
	"start_longitude",
	"end_latitude",
	"end_longitude",
	"price",
	"status",
	"started_at",
	"finished_at",
	"created_at",
	"updated_at",
}

type TripRepository struct {
	pool    *Pool
	builder sq.StatementBuilderType
}

func NewTripRepository(pool *Pool) *TripRepository {
	return &TripRepository{
		pool:    pool,
		builder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *TripRepository) Create(ctx context.Context, value trip.Trip) error {
	query, args, err := r.builder.
		Insert("trips").
		Columns(tripColumns...).
		Values(
			value.ID,
			value.UserID,
			value.DriverID,
			value.StartLatitude,
			value.StartLongitude,
			value.EndLatitude,
			value.EndLongitude,
			value.Price,
			string(value.Status),
			value.StartedAt,
			value.FinishedAt,
			value.CreatedAt,
			value.UpdatedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build create trip query: %w", err)
	}

	queryCtx, cancel := r.pool.queryContext(ctx)
	defer cancel()

	if _, err := executorFromContext(ctx, r.pool).Exec(queryCtx, query, args...); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == activeTripPerDriverConstraint {
			return fmt.Errorf("create trip: %w", trip.ErrDriverBusy)
		}

		return fmt.Errorf("create trip: %w", err)
	}

	return nil
}

func (r *TripRepository) GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error) {
	query, args, err := r.builder.
		Select(tripColumns...).
		From("trips").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return trip.Trip{}, fmt.Errorf("build get trip query: %w", err)
	}

	queryCtx, cancel := r.pool.queryContext(ctx)
	defer cancel()

	result, err := scanTrip(executorFromContext(ctx, r.pool).QueryRow(queryCtx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return trip.Trip{}, fmt.Errorf("get trip: %w", trip.ErrTripNotFound)
	}
	if err != nil {
		return trip.Trip{}, fmt.Errorf("get trip: %w", err)
	}

	return result, nil
}

func (r *TripRepository) Finish(
	ctx context.Context,
	id uuid.UUID,
	finishedAt time.Time,
) (trip.Trip, error) {
	query, args, err := r.builder.
		Update("trips").
		Set("status", string(trip.StatusCompleted)).
		Set("finished_at", finishedAt).
		Set("updated_at", finishedAt).
		Where(sq.Eq{
			"id":     id,
			"status": string(trip.StatusActive),
		}).
		Suffix("RETURNING " + strings.Join(tripColumns, ", ")).
		ToSql()
	if err != nil {
		return trip.Trip{}, fmt.Errorf("build finish trip query: %w", err)
	}

	queryCtx, cancel := r.pool.queryContext(ctx)

	executor := executorFromContext(ctx, r.pool)
	result, err := scanTrip(executor.QueryRow(queryCtx, query, args...))
	cancel()
	if err == nil {
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return trip.Trip{}, fmt.Errorf("finish trip: %w", err)
	}

	statusCtx, cancelStatus := r.pool.queryContext(ctx)
	defer cancelStatus()

	status, err := r.getStatus(statusCtx, executor, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return trip.Trip{}, fmt.Errorf("finish trip: %w", trip.ErrTripNotFound)
	}
	if err != nil {
		return trip.Trip{}, fmt.Errorf("get trip status after unsuccessful finish: %w", err)
	}
	if status == trip.StatusCompleted {
		return trip.Trip{}, fmt.Errorf("finish trip: %w", trip.ErrTripCompleted)
	}

	return trip.Trip{}, fmt.Errorf("finish trip: unexpected current status %q", status)
}

func (r *TripRepository) AddStatusHistory(ctx context.Context, history trip.StatusHistory) error {
	var fromStatus any
	if history.FromStatus != nil {
		fromStatus = string(*history.FromStatus)
	}

	var reason any
	if history.Reason != nil {
		reason = *history.Reason
	}

	query, args, err := r.builder.
		Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason", "changed_at").
		Values(
			history.TripID,
			fromStatus,
			string(history.ToStatus),
			reason,
			history.ChangedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build add trip status history query: %w", err)
	}

	queryCtx, cancel := r.pool.queryContext(ctx)
	defer cancel()

	if _, err := executorFromContext(ctx, r.pool).Exec(queryCtx, query, args...); err != nil {
		return fmt.Errorf("add trip status history: %w", err)
	}

	return nil
}

func (r *TripRepository) ClaimIdempotencyKey(
	ctx context.Context,
	record trip.IdempotencyRecord,
	ttl time.Duration,
) (bool, error) {
	query, args, err := r.builder.
		Insert(tripCreateIdempotencyTable).
		Columns(
			"idempotency_key",
			"fingerprint_version",
			"request_fingerprint",
			"trip_id",
			"created_at",
			"expires_at",
		).
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
		Suffix(`ON CONFLICT (idempotency_key) DO UPDATE SET
fingerprint_version = EXCLUDED.fingerprint_version,
request_fingerprint = EXCLUDED.request_fingerprint,
trip_id = EXCLUDED.trip_id,
created_at = EXCLUDED.created_at,
expires_at = EXCLUDED.expires_at
WHERE trip_create_idempotency.expires_at <= statement_timestamp()
RETURNING trip_id`).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build claim idempotency key query: %w", err)
	}

	queryCtx, cancel := r.pool.queryContext(ctx)
	defer cancel()

	var tripID uuid.UUID
	err = executorFromContext(ctx, r.pool).QueryRow(queryCtx, query, args...).Scan(&tripID)
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

func (r *TripRepository) GetIdempotencyKey(
	ctx context.Context,
	key uuid.UUID,
) (trip.IdempotencyRecord, error) {
	query, args, err := r.builder.
		Select(
			"idempotency_key",
			"fingerprint_version",
			"request_fingerprint",
			"trip_id",
		).
		From(tripCreateIdempotencyTable).
		Where(sq.Eq{"idempotency_key": key}).
		ToSql()
	if err != nil {
		return trip.IdempotencyRecord{}, fmt.Errorf("build get idempotency key query: %w", err)
	}

	queryCtx, cancel := r.pool.queryContext(ctx)
	defer cancel()

	var record trip.IdempotencyRecord
	err = executorFromContext(ctx, r.pool).QueryRow(queryCtx, query, args...).Scan(
		&record.Key,
		&record.FingerprintVersion,
		&record.RequestFingerprint,
		&record.TripID,
	)
	if err != nil {
		return trip.IdempotencyRecord{}, fmt.Errorf("get idempotency key: %w", err)
	}

	return record, nil
}

func (r *TripRepository) getStatus(
	ctx context.Context,
	executor dbtx,
	id uuid.UUID,
) (trip.Status, error) {
	query, args, err := r.builder.
		Select("status").
		From("trips").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return "", fmt.Errorf("build get trip status query: %w", err)
	}

	var status string
	if err := executor.QueryRow(ctx, query, args...).Scan(&status); err != nil {
		return "", err
	}

	return trip.Status(status), nil
}

func scanTrip(row pgx.Row) (trip.Trip, error) {
	var result trip.Trip
	var status string

	err := row.Scan(
		&result.ID,
		&result.UserID,
		&result.DriverID,
		&result.StartLatitude,
		&result.StartLongitude,
		&result.EndLatitude,
		&result.EndLongitude,
		&result.Price,
		&status,
		&result.StartedAt,
		&result.FinishedAt,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if err != nil {
		return trip.Trip{}, err
	}

	result.Status = trip.Status(status)
	return result, nil
}
