package get

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
	"github.com/google/uuid"
)

type Service struct {
	positionRepository positionRepository
	tripRepository     tripRepository

	logger *slog.Logger
}

func NewService(
	positionRepository positionRepository,
	tripRepository tripRepository,
	logger *slog.Logger,
) *Service {
	return &Service{
		positionRepository: positionRepository,
		tripRepository:     tripRepository,
		logger:             logger,
	}
}
func (s *Service) Get(ctx context.Context, tripID uuid.UUID) ([]position.Position, error) {
	if _, err := s.tripRepository.GetByID(ctx, tripID); err != nil {
		return nil, fmt.Errorf("get trip: %w", err)
	}

	pos, err := s.positionRepository.GetListByTripID(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("list trip positions: %w", err)

	}
	return pos, nil
}
