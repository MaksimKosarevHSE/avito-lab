package create

import "context"

type postitionCreator interface {
	Create(ctx context.Context, input uspos.CreateInput) (api)
}