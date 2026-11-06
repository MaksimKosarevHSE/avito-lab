package get

import (
	"context"
	"fmt"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/google/uuid"
)

type Service struct {
	tripRepository TripRepository
}

func NewService(tripRepository TripRepository) *Service {
	return &Service{
		tripRepository: tripRepository,
	}
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (trip.Trip, error) {
	result, err := s.tripRepository.GetByID(ctx, id)
	if err != nil {
		return trip.Trip{}, fmt.Errorf("get trip: %w", err)
	}

	return result, nil
}
