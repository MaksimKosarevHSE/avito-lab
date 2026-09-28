package trip

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type Repository interface {
	Create(ctx context.Context, trip Trip) error
	GetByID(ctx context.Context, id uuid.UUID) (Trip, error)
	Finish(ctx context.Context, id uuid.UUID, finishedAt time.Time) (Trip, error)
	AddStatusHistory(ctx context.Context, history StatusHistory) error
}

type Service struct {
	txManager  TxManager
	repository Repository
}

func NewService(txManager TxManager, repository Repository) *Service {
	return &Service{
		txManager:  txManager,
		repository: repository,
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Trip, error) {
	if err := validateCreateInput(input); err != nil {
		return Trip{}, err
	}

	now := time.Now().UTC()
	created := Trip{
		ID:             uuid.New(),
		UserID:         input.UserID,
		DriverID:       input.DriverID,
		StartLatitude:  input.StartLatitude,
		StartLongitude: input.StartLongitude,
		EndLatitude:    input.EndLatitude,
		EndLongitude:   input.EndLongitude,
		Price:          input.Price,
		Status:         StatusActive,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	reason := "trip created"
	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		if err := s.repository.Create(ctx, created); err != nil {
			return fmt.Errorf("create trip: %w", err)
		}

		if err := s.repository.AddStatusHistory(ctx, StatusHistory{
			TripID:    created.ID,
			ToStatus:  StatusActive,
			Reason:    &reason,
			ChangedAt: now,
		}); err != nil {
			return fmt.Errorf("add initial trip status history: %w", err)
		}

		return nil
	})
	if err != nil {
		return Trip{}, fmt.Errorf("create trip transaction: %w", err)
	}

	return created, nil
}

func validateCreateInput(input CreateInput) error {
	switch {
	case input.UserID == uuid.Nil:
		return fmt.Errorf("%w: user ID is required", ErrInvalidTrip)
	case input.DriverID == uuid.Nil:
		return fmt.Errorf("%w: driver ID is required", ErrInvalidTrip)
	case !validCoordinate(input.StartLatitude, -90, 90):
		return fmt.Errorf("%w: start latitude must be between -90 and 90", ErrInvalidTrip)
	case !validCoordinate(input.StartLongitude, -180, 180):
		return fmt.Errorf("%w: start longitude must be between -180 and 180", ErrInvalidTrip)
	case !validCoordinate(input.EndLatitude, -90, 90):
		return fmt.Errorf("%w: end latitude must be between -90 and 90", ErrInvalidTrip)
	case !validCoordinate(input.EndLongitude, -180, 180):
		return fmt.Errorf("%w: end longitude must be between -180 and 180", ErrInvalidTrip)
	case input.Price < 0:
		return fmt.Errorf("%w: price must not be negative", ErrInvalidTrip)
	default:
		return nil
	}
}

func validCoordinate(value, minValue, maxValue float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minValue && value <= maxValue
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Trip, error) {
	result, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return Trip{}, fmt.Errorf("get trip: %w", err)
	}

	return result, nil
}

func (s *Service) Finish(ctx context.Context, id uuid.UUID) (Trip, error) {
	now := time.Now().UTC()
	var finished Trip

	reason := "trip finished"
	fromStatus := StatusActive
	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		var err error
		finished, err = s.repository.Finish(ctx, id, now)
		if err != nil {
			return fmt.Errorf("finish trip: %w", err)
		}

		if err := s.repository.AddStatusHistory(ctx, StatusHistory{
			TripID:     id,
			FromStatus: &fromStatus,
			ToStatus:   StatusCompleted,
			Reason:     &reason,
			ChangedAt:  now,
		}); err != nil {
			return fmt.Errorf("add completed trip status history: %w", err)
		}

		return nil
	})
	if err != nil {
		return Trip{}, fmt.Errorf("finish trip transaction: %w", err)
	}

	return finished, nil
}
