package get

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
)

type Service struct {
	positionRepository positionRepository
	tripRepository     tripRepository
}

func NewService(
	positionRepository positionRepository,
	tripRepository tripRepository,
) *Service {
	return &Service{
		positionRepository: positionRepository,
		tripRepository:     tripRepository,
	}
}

func (s *Service) GetList(ctx context.Context, tripID uuid.UUID) ([]position.Position, error) {
	if _, err := s.tripRepository.GetByID(ctx, tripID); err != nil {
		return nil, fmt.Errorf("get trip: %w", err)
	}

	pos, err := s.positionRepository.GetListByTripID(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("list trip positions: %w", err)
	}

	return pos, nil
}
