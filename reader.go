package limitedreader

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type LimitedReader struct {
	reader          io.ReadCloser
	cfg             Config
	limit           atomic.Int64
	totalRead       atomic.Int64
	lastElapsed     int64
	timeSlept       int64
	timeAccumulated int64
	lock            sync.Mutex
	clock           Clock
	ctx             context.Context
}

func NewLimitedReader(reader io.Reader, limit int64, opts ...Option) *LimitedReader {
	return NewLimitedReadCloser(io.NopCloser(reader), limit, opts...)
}

func NewLimitedReadCloser(reader io.ReadCloser, limit int64, opts ...Option) *LimitedReader {
	lr := &LimitedReader{
		reader: reader,
		cfg:    Config{DefaultReadIntervalMilliseconds},
		clock:  realClock{},
		ctx:    context.Background(),
	}

	lr.limit.Store(limit)
	lr.totalRead.Store(0)

	for _, opt := range opts {
		opt(lr)
	}

	return lr
}

func (lr *LimitedReader) Read(p []byte) (n int, err error) {
	var iterTotalRead int64
	chunkSize := int64(len(p))

	lr.lock.Lock()
	defer lr.lock.Unlock()

	for !lr.isContextCanceled() && iterTotalRead < chunkSize {
		limit := lr.limit.Load()
		if limit <= 0 {
			limit = lr.waitUntilValidLimitIsSet()
		}

		// the limit set to per second
		limit = limit / (1000 / lr.cfg.ReadIntervalMilliseconds)

		allowedBytes := limit
		chunkSizeLeft := chunkSize - iterTotalRead
		if chunkSizeLeft < allowedBytes {
			allowedBytes = chunkSizeLeft
		}

		lr.sleep(allowedBytes, limit)

		n, err = lr.reader.Read(p[iterTotalRead : iterTotalRead+allowedBytes : iterTotalRead+allowedBytes])
		iterTotalRead += int64(n)
		lr.totalRead.Add(int64(n))
		if err != nil {
			break
		}
	}

	if err == nil && lr.isContextCanceled() {
		err = context.Canceled
	}

	return int(iterTotalRead), err
}

func (lr *LimitedReader) waitUntilValidLimitIsSet() int64 {
	var limit int64
	for limit <= 0 && !lr.isContextCanceled() {
		lr.clock.Sleep(time.Duration(lr.cfg.ReadIntervalMilliseconds) * time.Millisecond)
		limit = lr.limit.Load()
	}

	return limit
}

func (lr *LimitedReader) sleep(allowedBytes, iterLimit int64) {
	expectedTime := allowedBytes * lr.cfg.ReadIntervalMilliseconds * int64(time.Millisecond) / iterLimit

	now := lr.clock.Now().UnixNano()
	elapsed := now - lr.lastElapsed - lr.timeSlept
	if elapsed > int64(time.Second) {
		elapsed = 0
		lr.lastElapsed = now
		lr.timeSlept = 0
		lr.timeAccumulated = 0
	}

	sleepTime := lr.timeAccumulated - (elapsed - expectedTime)
	if sleepTime > 0 {
		lr.clock.Sleep(time.Duration(sleepTime))
		lr.timeAccumulated = 0
		if elapsed == 0 {
			lr.timeSlept += sleepTime
		} else {
			lr.timeSlept = 0
			lr.lastElapsed = now + sleepTime
		}
	} else {
		lr.timeAccumulated = sleepTime
		lr.timeSlept = 0
		lr.lastElapsed = now
	}
}

func (lr *LimitedReader) isContextCanceled() bool {
	select {
	case <-lr.ctx.Done():
		return true
	default:
		return false
	}
}

func (lr *LimitedReader) Close() error {
	return lr.reader.Close()
}

func (lr *LimitedReader) UpdateLimit(newLimit int64) {
	lr.limit.Store(newLimit)
}

func (lr *LimitedReader) GetTotalRead() int64 {
	return lr.totalRead.Load()
}
