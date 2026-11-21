package trip

import (
	"github.com/MaksimKosarevHSE/avito-lab/internal/worker/positions"
	sq "github.com/Masterminds/squirrel"
)

type Repository struct {
	db      database
	builder sq.StatementBuilderType
}

func New(db database) *Repository {
	return &Repository{
		db:      db,
		builder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

var _ positions.TripBatchSelector = (*Repository)(nil)
