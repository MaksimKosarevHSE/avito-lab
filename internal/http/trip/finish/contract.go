package finish

import (
	"context"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/google/uuid"
)

type tripFinisher interface {
	Finish(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error)
}
