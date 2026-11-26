package position

import (
	sq "github.com/Masterminds/squirrel"
)

type Repository struct {
	db      database
	builder sq.StatementBuilderType
}

func NewRepository(db database) *Repository {
	return &Repository{
		db:      db,
		builder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}
