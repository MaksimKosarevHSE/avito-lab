//go:generate go tool mockgen -source=${GOFILE} -destination=mocks_test.go -package=${GOPACKAGE}_test

package get

import (
	"context"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/google/uuid"
)

type tripGetter interface {
	Get(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error)
}
