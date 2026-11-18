package finish

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
)

type Service struct {
	tripRepository TripRepository
	txManager      TxManager
	logger         *slog.Logger
	metrics        TripMetrics
}

type Option func(*Service)

func WithObservability(logger *slog.Logger, metrics TripMetrics) Option {
	return func(service *Service) {
		service.logger = logger
		service.metrics = metrics
	}
}

func NewService(
	txManager TxManager,
	tripRepository TripRepository,
	options ...Option,
) *Service {
	service := &Service{
		txManager:      txManager,
		tripRepository: tripRepository,
	}
	for _, option := range options {
		option(service)
	}

	return service
}

func (s *Service) Finish(ctx context.Context, id uuid.UUID) (trip.Trip, error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	var finished trip.Trip

	reason := "trip finished"
	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		var err error
		finished, err = s.tripRepository.GetByIDForUpdate(ctx, id)
		if err != nil {
			return fmt.Errorf("get trip for update: %w", err)
		}

		fromStatus := finished.Status
		if err := finished.Finish(now); err != nil {
			return fmt.Errorf("finish trip: %w", err)
		}

		if err := s.tripRepository.Update(ctx, finished); err != nil {
			return fmt.Errorf("update trip: %w", err)
		}

		if err := s.tripRepository.AddStatusHistory(ctx, trip.StatusHistory{
			TripID:     finished.ID,
			FromStatus: &fromStatus,
			ToStatus:   finished.Status,
			Reason:     &reason,
			ChangedAt:  now,
		}); err != nil {
			return fmt.Errorf("add completed trip status history: %w", err)
		}

		return nil
	})
	if err != nil {
		return trip.Trip{}, fmt.Errorf("finish trip transaction: %w", err)
	}
	if s.metrics != nil {
		s.metrics.TripCompleted(ctx)
	}
	if s.logger != nil {
		s.logger.InfoContext(ctx, "trip completed", "trip_id", finished.ID)
	}

	return finished, nil
}
