package create

import (
	"context"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
	"github.com/MaksimKosarevHSE/avito-lab/internal/usecase/position/create"
)

type positionCreator interface {
	Create(ctx context.Context, input create.Input) (position.Position, error)
}
