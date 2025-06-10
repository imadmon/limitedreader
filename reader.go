package limitedreader

import (
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
	lastElapsed     atomic.Int64
	timeSlept       atomic.Int64
	timeAccumulated atomic.Int64
}

func NewLimitedReader(reader io.Reader, limit int64) *LimitedReader {
	return NewLimitedReadCloser(io.NopCloser(reader), limit)
}

func NewLimitedReadCloser(reader io.ReadCloser, limit int64) *LimitedReader {
	lr := &LimitedReader{
		reader: reader,
	}

	lr.limit.Store(limit)
	lr.iterTotalRead.Store(0)
	lr.lastElapsed.Store(0)
	lr.timeSlept.Store(0)
	lr.timeAccumulated.Store(0)
	return lr
}

func (lr *LimitedReader) Read(p []byte) (n int, err error) {
	lr.iterTotalRead.Store(0)
	chunkSize := int64(len(p))
	for lr.iterTotalRead.Load() < chunkSize {
		limit := lr.limit.Load()
		if limit <= 0 {
			n, err = lr.readWithoutLimit(p[lr.iterTotalRead.Load():int(chunkSize)])
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

		n, err = lr.reader.Read(p[lr.iterTotalRead.Load():int(lr.iterTotalRead.Load()+allowedBytes)])
		lr.iterTotalRead.Add(int64(n))
		if err != nil {
			break
		}
	}

	return int(lr.iterTotalRead.Load()), err
}

func (lr *LimitedReader) readWithoutLimit(p []byte) (n int, err error) {
	return lr.reader.Read(p)
}

func (lr *LimitedReader) sleep(allowedBytes, iterLimit int64) {
	expectedTime := allowedBytes * ReadIntervalMilliseconds * int64(time.Millisecond) / iterLimit

	now := time.Now().UnixNano()
	elapsed := now - lr.lastElapsed.Load() - lr.timeSlept.Load()
	if elapsed > int64(time.Second) {
		elapsed = 0
		lr.lastElapsed.Store(now)
		lr.timeSlept.Store(0)
		lr.timeAccumulated.Store(0)
	}

	sleepTime := lr.timeAccumulated.Load() - (elapsed - expectedTime)
	if sleepTime > 0 {
		time.Sleep(time.Duration(sleepTime))
		lr.timeAccumulated.Store(0)
		if elapsed == 0 {
			lr.timeSlept.Add(sleepTime)
		} else {
			lr.timeSlept.Store(0)
			lr.lastElapsed.Store(now + sleepTime)
		}
	} else {
		lr.timeAccumulated.Store(sleepTime)
		lr.timeSlept.Store(0)
		lr.lastElapsed.Store(now)
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
