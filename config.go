package limitedreader

const (
	DefaultReadIntervalMilliseconds int64 = 50
)

type Config struct {
	ReadIntervalMilliseconds int64
}
