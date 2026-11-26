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
	"github.com/MaksimKosarevHSE/avito-lab/internal/worker/positions"
)

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	return r.getByID(ctx, id, false)
}

func (r *Repository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	return r.getByID(ctx, id, true)
}

func (r *Repository) getByID(
	ctx context.Context,
	id uuid.UUID,
	forUpdate bool,
) (domaintrip.Trip, error) {
	queryBuilder := r.builder.
		Select(tripColumns...).
		From(tripsTable).
		Where(sq.Eq{idColumn: id})
	if forUpdate {
		queryBuilder = queryBuilder.Suffix("FOR UPDATE")
	}

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("build get trip query: %w", err)
	}

	var result domaintrip.Trip
	err = r.db.Execute(ctx, func(queryCtx context.Context, executor postgres.Executor) error {
		var scanErr error
		result, scanErr = scanTrip(executor.QueryRow(queryCtx, query, args...))
		return scanErr
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domaintrip.Trip{}, fmt.Errorf("get trip: %w", domaintrip.ErrTripNotFound)
	}
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("get trip: %w", err)
	}

	return result, nil
}

const fetchForPushQuery = `
SELECT id, driver_id, COALESCE(last_position_at, started_at) as last_tracked_at
FROM trips WHERE trips.status = 'active'
AND COALESCE(last_position_at, started_at) < NOW() - ($1::bigint * INTERVAL '1 microsecond')
AND ($2::timestamptz IS NULL OR ((COALESCE(last_position_at, started_at), id) > ($2::timestamptz, $3::uuid)))
ORDER BY COALESCE(last_position_at, started_at) ASC, id ASC
LIMIT $4
`

func (r *Repository) FetchForPushQuery(
	ctx context.Context,
	positionStaleAfter time.Duration,
	positionScanBatchSize int32,
	ptr *positions.FetchForPushPtr, // optional
) (positions.FetchForPushResult, error) {
	var ptrTime any
	var ptrID any
	if ptr != nil {
		ptrTime = ptr.LastTrackedAt
		ptrID = ptr.TripID
	}

	result := make([]positions.SelectedTrip, 0)
	err := r.db.Execute(ctx, func(queryCtx context.Context, executor postgres.Executor) error {
		rows, err := executor.Query(
			queryCtx,
			fetchForPushQuery,
			positionStaleAfter.Microseconds(),
			ptrTime,
			ptrID,
			positionScanBatchSize,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			row, err := ScanTripForPush(rows)
			if err != nil {
				return err
			}

			result = append(result, row)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return positions.FetchForPushResult{}, err
	}

	var nextPtr *positions.FetchForPushPtr = nil
	if len(result) > 0 {
		last := result[len(result)-1]
		nextPtr = &positions.FetchForPushPtr{
			LastTrackedAt: last.LastTrackedAt,
			TripID:        last.TripID,
		}
	}

	return positions.FetchForPushResult{
		SelectedTrips: result,
		NextPtr:       nextPtr,
	}, nil
}
