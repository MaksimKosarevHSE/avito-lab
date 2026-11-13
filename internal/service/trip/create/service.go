package create

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
)

const idempotencyFingerprintVersion int16 = 1

type Service struct {
	tripRepository TripRepository
	txManager      TxManager
	idempotencyTTL time.Duration
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
	idempotencyTTL time.Duration,
	options ...Option,
) *Service {
	service := &Service{
		txManager:      txManager,
		tripRepository: tripRepository,
		idempotencyTTL: idempotencyTTL,
	}
	for _, option := range options {
		option(service)
	}

	return service
}

func (s *Service) Create(ctx context.Context, input CreateInput) (CreateResult, error) {
	now := time.Now().UTC().Truncate(time.Microsecond)

	created, err := trip.NewTrip(
		uuid.New(),
		input.UserID,
		input.DriverID,
		input.StartLatitude,
		input.StartLongitude,
		input.EndLatitude,
		input.EndLongitude,
		input.Price,
		now,
	)
	if err != nil {
		return CreateResult{}, err
	}

	var requestFingerprint []byte
	if input.IdempotencyKey != nil {
		requestFingerprint = createInputFingerprint(input)
	}

	var result CreateResult
	reason := "trip created"

	err = s.txManager.Do(ctx, func(ctx context.Context) error {
		if input.IdempotencyKey != nil {
			claimed, err := s.tripRepository.ClaimIdempotencyKey(ctx, trip.IdempotencyRecord{
				Key:                *input.IdempotencyKey,
				FingerprintVersion: idempotencyFingerprintVersion,
				RequestFingerprint: requestFingerprint,
				TripID:             created.ID,
			}, s.idempotencyTTL)
			if err != nil {
				return fmt.Errorf("claim idempotency key: %w", err)
			}

			if !claimed {
				record, err := s.tripRepository.GetIdempotencyKey(ctx, *input.IdempotencyKey)
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
					return trip.ErrIdempotencyConflict
				}

				replayed, err := s.tripRepository.GetByID(ctx, record.TripID)
				if errors.Is(err, trip.ErrTripNotFound) {
					return errors.New("idempotency record references missing trip")
				}
				if err != nil {
					return fmt.Errorf("get idempotent trip: %w", err)
				}

				result = CreateResult{Trip: replayed, Replayed: true}
				return nil
			}
		}

		if err := s.tripRepository.Create(ctx, *created); err != nil {
			return fmt.Errorf("create trip: %w", err)
		}

		if err := s.tripRepository.AddStatusHistory(ctx, trip.StatusHistory{
			TripID:    created.ID,
			ToStatus:  trip.StatusActive,
			Reason:    &reason,
			ChangedAt: now,
		}); err != nil {
			return fmt.Errorf("add initial trip status history: %w", err)
		}

		result = CreateResult{Trip: *created}
		return nil
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("create trip transaction: %w", err)
	}
	if !result.Replayed {
		if s.metrics != nil {
			s.metrics.TripCreated(ctx)
		}
		if s.logger != nil {
			s.logger.InfoContext(ctx, "trip created", "trip_id", result.Trip.ID)
		}
	}

	return result, nil
}

type CreateInput struct {
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	IdempotencyKey *uuid.UUID
}

type CreateResult struct {
	Trip     trip.Trip
	Replayed bool
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

	var price uint64
	if input.Price > 0 {
		price = uint64(input.Price)
	}
	binary.BigEndian.PutUint64(data[offset:], price)
	sum := sha256.Sum256(data)
	return sum[:]
}
