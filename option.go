package limitedreader

type Option func(*LimitedReader)

func WithClock(c Clock) Option {
	return func(lr *LimitedReader) {
		lr.clock = c
	}
}
