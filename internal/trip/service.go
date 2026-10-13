package trip

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

const idempotencyFingerprintVersion int16 = 1

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type Repository interface {
	Create(ctx context.Context, trip Trip) error
	GetByID(ctx context.Context, id uuid.UUID) (Trip, error)
	Finish(ctx context.Context, id uuid.UUID, finishedAt time.Time) (Trip, error)
	AddStatusHistory(ctx context.Context, history StatusHistory) error
	DeleteExpiredIdempotencyKeys(ctx context.Context) error
	ClaimIdempotencyKey(
		ctx context.Context,
		record IdempotencyRecord,
		ttl time.Duration,
	) (bool, error)
	GetIdempotencyKey(ctx context.Context, key uuid.UUID) (IdempotencyRecord, error)
}

type Service struct {
	txManager      TxManager
	repository     Repository
	idempotencyTTL time.Duration
}

func NewService(txManager TxManager, repository Repository, idempotencyTTL time.Duration) *Service {
	return &Service{
		txManager:      txManager,
		repository:     repository,
		idempotencyTTL: idempotencyTTL,
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (CreateResult, error) {
	if err := validateCreateInput(input); err != nil {
		return CreateResult{}, err
	}
	if input.IdempotencyKey != nil {
		if err := s.repository.DeleteExpiredIdempotencyKeys(ctx); err != nil {
			return CreateResult{}, fmt.Errorf("cleanup idempotency keys: %w", err)
		}
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
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

	var requestFingerprint []byte
	if input.IdempotencyKey != nil {
		requestFingerprint = createInputFingerprint(input)
	}

	var result CreateResult
	reason := "trip created"
	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		if input.IdempotencyKey != nil {
			claimed, err := s.repository.ClaimIdempotencyKey(ctx, IdempotencyRecord{
				Key:                *input.IdempotencyKey,
				FingerprintVersion: idempotencyFingerprintVersion,
				RequestFingerprint: requestFingerprint,
				TripID:             created.ID,
			}, s.idempotencyTTL)
			if err != nil {
				return fmt.Errorf("claim idempotency key: %w", err)
			}

			if !claimed {
				record, err := s.repository.GetIdempotencyKey(ctx, *input.IdempotencyKey)
				if err != nil {
					return fmt.Errorf("get idempotency key: %w", err)
				}
				if record.FingerprintVersion != idempotencyFingerprintVersion {
					return fmt.Errorf(
						"unsupported idempotency fingerprint version %d",
						record.FingerprintVersion,
					)
				}
				if !bytes.Equal(record.RequestFingerprint, requestFingerprint) {
					return ErrIdempotencyConflict
				}

				replayed, err := s.repository.GetByID(ctx, record.TripID)
				if errors.Is(err, ErrTripNotFound) {
					return errors.New("idempotency record references missing trip")
				}
				if err != nil {
					return fmt.Errorf("get idempotent trip: %w", err)
				}

				result = CreateResult{Trip: replayed, Replayed: true}
				return nil
			}
		}

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

		result = CreateResult{Trip: created}
		return nil
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("create trip transaction: %w", err)
	}

	return result, nil
}

func createInputFingerprint(input CreateInput) []byte {
	data := make([]byte, 72)
	offset := 0

	copy(data[offset:], input.UserID[:])
	offset += len(input.UserID)
	copy(data[offset:], input.DriverID[:])
	offset += len(input.DriverID)

	for _, coordinate := range []float64{
		input.StartLatitude,
		input.StartLongitude,
		input.EndLatitude,
		input.EndLongitude,
	} {
		var bits uint64
		if coordinate != 0 {
			bits = math.Float64bits(coordinate)
		}
		binary.BigEndian.PutUint64(data[offset:], bits)
		offset += 8
	}

	binary.BigEndian.PutUint64(data[offset:], uint64(input.Price))
	sum := sha256.Sum256(data)
	return sum[:]
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
	now := time.Now().UTC().Truncate(time.Microsecond)
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
