package create

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
)

type Service struct {
	positionRepository    positionRepository
	tripRepository        tripRepository
	txManager             txManager
	positionPushCanceller positionPushCanceller
	positionMetrics       positionSaveMetrics
}

func NewService(
	positionRepository positionRepository,
	tripRepository tripRepository,
	txManager txManager,
	positionPushCanceller positionPushCanceller,
	positionMetrics positionSaveMetrics,
) *Service {
	return &Service{
		positionRepository:    positionRepository,
		tripRepository:        tripRepository,
		txManager:             txManager,
		positionPushCanceller: positionPushCanceller,
		positionMetrics:       positionMetrics,
	}
}

type Input struct {
	TripID     uuid.UUID
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
}

func (s *Service) Create(ctx context.Context, input Input) (position.Position, error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	recordedAt := input.RecordedAt.UTC().Truncate(time.Microsecond)

	if recordedAt.After(now.Add(time.Minute)) {
		return position.Position{}, fmt.Errorf(
			"%w: recorded_at must not be more than one minute in the future",
			position.ErrInvalidPosition,
		)
	}

	value, err := position.NewPosition(
		input.TripID,
		input.Latitude,
		input.Longitude,
		recordedAt,
		now,
	)
	if err != nil {
		return position.Position{}, err
	}

	var createResult position.Position

	err = s.txManager.Do(ctx, func(ctx context.Context) error {
		currTrip, err := s.tripRepository.GetByIDForUpdate(ctx, value.TripID)
		if err != nil {
			return fmt.Errorf("get trip for update: %w", err)
		}
		if currTrip.Status != trip.StatusActive {
			return trip.ErrTripCompleted
		}
		created, err := s.positionRepository.Create(ctx, *value)
		if err != nil {
			return fmt.Errorf("create position: %w", err)
		}
		createResult = created

		if err := s.tripRepository.UpdateLastPositionAt(ctx, input.TripID, value.RecordedAt); err != nil {
			return fmt.Errorf("update last position time: %w", err)
		}
		return nil
	})
	if err != nil {
		return position.Position{}, fmt.Errorf("create position transaction: %w", err)
	}
	s.positionMetrics.AddPositionSaved(ctx)
	s.positionPushCanceller.Cancel(value.TripID)
	return createResult, nil
}
