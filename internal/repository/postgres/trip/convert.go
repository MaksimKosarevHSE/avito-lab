package trip

import (
	"github.com/jackc/pgx/v5"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/worker/positions"
)

func scanTrip(row pgx.Row) (domaintrip.Trip, error) {
	var result domaintrip.Trip
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
		&result.LastPositionAt,
	)
	if err != nil {
		return domaintrip.Trip{}, err
	}

	result.StartedAt = result.StartedAt.UTC()
	result.CreatedAt = result.CreatedAt.UTC()
	result.UpdatedAt = result.UpdatedAt.UTC()
	if result.FinishedAt != nil {
		finishedAt := result.FinishedAt.UTC()
		result.FinishedAt = &finishedAt
	}
	if result.LastPositionAt != nil {
		positionAt := result.LastPositionAt.UTC()
		result.LastPositionAt = &positionAt
	}

	result.Status = domaintrip.Status(status)
	return result, nil
}

func ScanTripForPush(row pgx.Row) (positions.SelectedTrip, error) {
	var result positions.SelectedTrip
	err := row.Scan(
		&result.TripID,
		&result.DriverID,
		&result.LastTrackedAt,
	)
	if err != nil {
		return positions.SelectedTrip{}, err
	}
	result.LastTrackedAt = result.LastTrackedAt.UTC()
	return result, nil
}
