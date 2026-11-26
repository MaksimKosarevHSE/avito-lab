package get

import (
	"github.com/MaksimKosarevHSE/avito-lab/api"
	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
)

func toAPIPositionList(positions []position.Position) api.TripPositionsResponse {
	result := api.TripPositionsResponse{
		Positions: make([]api.TripPosition, len(positions)),
	}

	for i, el := range positions {
		result.Positions[i] = toAPIPosition(el)
	}
	return result
}

func toAPIPosition(position position.Position) api.TripPosition {
	return api.TripPosition{
		Id: position.ID,
		Coordinates: api.Coordinates{
			Latitude:  position.Latitude,
			Longitude: position.Longitude,
		},
		RecordedAt: position.RecordedAt,
		TripId:     position.TripID,
	}
}
