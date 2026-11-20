package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
)

type positionGetter interface {
	GetList(ctx context.Context, tripID uuid.UUID) ([]position.Position, error)
}
