package create

import (
	"context"

	createuc "github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/create"
)

type tripCreator interface {
	Create(ctx context.Context, input createuc.CreateInput) (createuc.CreateResult, error)
}
