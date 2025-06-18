package limitedreader

import "context"

type Option func(*LimitedReader)

func WithContext(ctx context.Context) Option {
	return func(lr *LimitedReader) {
		lr.ctx = ctx
	}
}
