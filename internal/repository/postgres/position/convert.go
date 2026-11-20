package position

import (
	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
	"github.com/jackc/pgx/v5"
)

func scanPosition(row pgx.Row) (position.Position, error) {
	var result position.Position

	err := row.Scan(
		&result.ID,
		&result.TripID,
		&result.Latitude,
		&result.Longitude,
		&result.RecordedAt,
		&result.CreatedAt,
	)
	if err != nil {
		return position.Position{}, err
	}

	result.RecordedAt = result.RecordedAt.UTC()
	result.CreatedAt = result.CreatedAt.UTC()

	return result, nil
}
