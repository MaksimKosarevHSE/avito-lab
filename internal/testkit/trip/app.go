//go:build integration

package triptest

import (
	"testing"
	"time"

	triprepo "github.com/MaksimKosarevHSE/avito-lab/internal/repository/postgres/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/testkit"
	tripcreate "github.com/MaksimKosarevHSE/avito-lab/internal/usecase/trip/create"
	tripfinish "github.com/MaksimKosarevHSE/avito-lab/internal/usecase/trip/finish"
	tripget "github.com/MaksimKosarevHSE/avito-lab/internal/usecase/trip/get"
)

const idempotencyTTL = 24 * time.Hour

type App struct {
	*testkit.Postgres

	Repository    *triprepo.Repository
	CreateService *tripcreate.Service
	FinishService *tripfinish.Service
	GetService    *tripget.Service
}

func NewApp(t testing.TB) *App {
	t.Helper()

	postgres := testkit.NewPostgres(t)
	repository := triprepo.New(postgres.Database)

	return &App{
		Postgres:      postgres,
		Repository:    repository,
		CreateService: tripcreate.NewService(postgres.TxManager, repository, idempotencyTTL),
		FinishService: tripfinish.NewService(postgres.TxManager, repository),
		GetService:    tripget.NewService(repository),
	}
}
