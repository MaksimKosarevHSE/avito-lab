package create

import (
	"github.com/MaksimKosarevHSE/avito-lab/api"
	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	createuc "github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/create"
)

func toCreateInput(
	body api.CreateTripJSONRequestBody,
	params api.CreateTripParams,
) createuc.CreateInput {
	return createuc.CreateInput{
		UserID:         body.UserId,
		DriverID:       body.DriverId,
		StartLatitude:  body.StartPoint.Latitude,
		StartLongitude: body.StartPoint.Longitude,
		EndLatitude:    body.EndPoint.Latitude,
		EndLongitude:   body.EndPoint.Longitude,
		Price:          body.Price,
		IdempotencyKey: params.IdempotencyKey,
	}
}

func toAPITrip(value domaintrip.Trip) api.Trip {
	return api.Trip{
		Id:       value.ID,
		UserId:   value.UserID,
		DriverId: value.DriverID,
		StartPoint: api.Coordinates{
			Latitude:  value.StartLatitude,
			Longitude: value.StartLongitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  value.EndLatitude,
			Longitude: value.EndLongitude,
		},
		Price:          value.Price,
		Status:         api.TripStatus(value.Status),
		StartedAt:      value.StartedAt,
		FinishedAt:     value.FinishedAt,
		LastPositionAt: nil,
	}
}
