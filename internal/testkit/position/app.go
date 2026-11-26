//go:build integration

package positiontest

import (
	"context"
	"testing"

	"github.com/google/uuid"

	positionrepo "github.com/MaksimKosarevHSE/avito-lab/internal/repository/postgres/position"
	triprepo "github.com/MaksimKosarevHSE/avito-lab/internal/repository/postgres/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/testkit"
	positioncreate "github.com/MaksimKosarevHSE/avito-lab/internal/usecase/position/create"
)

type App struct {
	*testkit.Postgres

	TripRepository     *triprepo.Repository
	PositionRepository *positionrepo.Repository
	CreateService      *positioncreate.Service
}

func NewApp(t testing.TB) *App {
	t.Helper()

	postgres := testkit.NewPostgres(t)
	tripRepository := triprepo.New(postgres.Database)
	positionRepository := positionrepo.NewRepository(postgres.Database)

	return &App{
		Postgres:           postgres,
		TripRepository:     tripRepository,
		PositionRepository: positionRepository,
		CreateService: positioncreate.NewService(
			positionRepository,
			tripRepository,
			postgres.TxManager,
			noopCanceller{},
			noopMetrics{},
		),
	}
}

type noopCanceller struct{}

func (noopCanceller) Cancel(uuid.UUID) bool {
	return false
}

type noopMetrics struct{}

func (noopMetrics) AddPositionSaved(context.Context) {}
