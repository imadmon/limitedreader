package limitedreader

import "time"

// Clock abstracts time-related operations to allow time control in tests or simulations.
// This is useful for injecting mock time behavior, especially when writing deterrministic tests.
//
// By default, a realClock implementation is used, which simply wraps the standard time package.
// To simulate time you can inject clocks from libraries like:
//   - github.com/benbjohnson/clock
//   - github.com/andres-erbsen/clock
//
// Example usage:
// lr := NewRateLimitedReader(reader, limit, WithClock(clock.NewMock()))
type Clock interface {
	Now() time.Time
	Sleep(time.Duration)
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

func (realClock) Sleep(sleepTime time.Duration) {
	time.Sleep(sleepTime)
}
