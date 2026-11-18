package trip

import (
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
