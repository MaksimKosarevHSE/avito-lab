package create

import (
	"github.com/google/uuid"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
	"github.com/MaksimKosarevHSE/avito-lab/internal/usecase/position/create"
)

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

func toCreateInput(request api.CreateTripPositionJSONRequestBody, tripID uuid.UUID) create.Input {
	return create.Input{
		TripID:     tripID,
		Latitude:   request.Coordinates.Latitude,
		Longitude:  request.Coordinates.Longitude,
		RecordedAt: request.RecordedAt,
	}
}
