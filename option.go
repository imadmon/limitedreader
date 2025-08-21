package limitedreader

import "context"

type Option func(*LimitedReader)

func WithClock(c Clock) Option {
	return func(lr *LimitedReader) {
		lr.clock = c
	}
}

func WithContext(ctx context.Context) Option {
	return func(lr *LimitedReader) {
		lr.ctx = ctx
	}
}

func WithConfig(readIntervalMilliseconds int64) Option {
	return func(lr *LimitedReader) {
		lr.cfg = Config{
			ReadIntervalMilliseconds: readIntervalMilliseconds,
		}
	}
}
