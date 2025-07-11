package limitedreader

import (
	"context"
	"io"
	"sync/atomic"
	"time"
)

var (
	ReadIntervalMilliseconds int64 = 50
)

type LimitedReader struct {
	reader          io.ReadCloser
	limit           atomic.Int64
	iterTotalRead   atomic.Int64
	lastElapsed     int64
	timeSlept       int64
	timeAccumulated int64
	clock           Clock
	ctx             context.Context
}

func NewLimitedReader(reader io.Reader, limit int64, opts ...Option) *LimitedReader {
	return NewLimitedReadCloser(io.NopCloser(reader), limit, opts...)
}

func NewLimitedReadCloser(reader io.ReadCloser, limit int64, opts ...Option) *LimitedReader {
	lr := &LimitedReader{
		reader: reader,
		clock:  realClock{},
		ctx:    context.Background(),
	}

	lr.limit.Store(limit)
	lr.iterTotalRead.Store(0)

	for _, opt := range opts {
		opt(lr)
	}

	return lr
}

func (lr *LimitedReader) Read(p []byte) (n int, err error) {
	lr.iterTotalRead.Store(0)
	chunkSize := int64(len(p))
	for !lr.isContextCanceled() && lr.iterTotalRead.Load() < chunkSize {
		limit := lr.limit.Load()
		if limit <= 0 {
			n, err = lr.readWithoutLimit(p[lr.iterTotalRead.Load():chunkSize:chunkSize])
			lr.iterTotalRead.Add(int64(n))
			return int(lr.iterTotalRead.Load()), err
		}

		// the limit set to per second
		limit = limit / (1000 / ReadIntervalMilliseconds)

		allowedBytes := limit
		chunkSizeLeft := chunkSize - lr.iterTotalRead.Load()
		if chunkSizeLeft < allowedBytes {
			allowedBytes = chunkSizeLeft
		}

		lr.sleep(allowedBytes, limit)

		n, err = lr.reader.Read(p[lr.iterTotalRead.Load() : lr.iterTotalRead.Load()+allowedBytes : lr.iterTotalRead.Load()+allowedBytes])
		lr.iterTotalRead.Add(int64(n))
		if err != nil {
			break
		}
	}

	if err == nil && lr.isContextCanceled() {
		err = context.Canceled
	}

	return int(lr.iterTotalRead.Load()), err
}

func (lr *LimitedReader) readWithoutLimit(p []byte) (n int, err error) {
	return lr.reader.Read(p)
}

func (lr *LimitedReader) sleep(allowedBytes, iterLimit int64) {
	expectedTime := allowedBytes * ReadIntervalMilliseconds * int64(time.Millisecond) / iterLimit

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

func (lr *LimitedReader) GetCurrentIterTotalRead() int64 {
	return lr.iterTotalRead.Load()
}
