package get

import (
	"github.com/MaksimKosarevHSE/avito-lab/api"
	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
)

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
