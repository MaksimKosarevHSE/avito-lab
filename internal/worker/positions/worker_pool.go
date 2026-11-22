package positions

import (
	"context"
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

type TripSupplier interface {
	Take() (SelectedTrip, bool)
	Complete(tripID uuid.UUID)
	Close()
}

type PositionPusher interface {
	PushPosition(ctx context.Context, tripID, driverID uuid.UUID) error
}

type WorkerPool struct {
	workersCount   int32
	tripSupplier   TripSupplier
	positionPusher PositionPusher
	logger         *slog.Logger
}

func NewWorkerPool(workersCount int32, tripSupplier TripSupplier, positionPusher PositionPusher, logger *slog.Logger) *WorkerPool {
	return &WorkerPool{
		workersCount:   workersCount,
		tripSupplier:   tripSupplier,
		positionPusher: positionPusher,
		logger:         logger,
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(int(wp.workersCount))
	for i := range wp.workersCount {
		go wp.worker(ctx, int(i), &wg)
	}
	result := make(chan struct{})

	go func() {
		wg.Wait()
		close(result)
	}()

	select {
	case <-ctx.Done():
		wp.tripSupplier.Close()
		<-result
	case <-result:
	}
}

func (wp *WorkerPool) Stop() {
	wp.tripSupplier.Close()
}

func (wp *WorkerPool) worker(ctx context.Context, _ int, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		trip, ok := wp.tripSupplier.Take()
		if !ok {
			return
		}
		if ctx.Err() != nil {
			return
		}

		err := wp.positionPusher.PushPosition(ctx, trip.TripID, trip.DriverID)
		wp.tripSupplier.Complete(trip.TripID)
		if err != nil {
			wp.logger.WarnContext(ctx, "push position failed", "trip_id", trip.TripID, "error", err)
		}
	}
}
