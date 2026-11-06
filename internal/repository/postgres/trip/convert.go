package trip

import (
	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/jackc/pgx/v5"
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

	result.Status = domaintrip.Status(status)
	return result, nil
}
