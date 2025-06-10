package limitedreader

import (
	"io"
	"sync/atomic"
	"time"
)

var (
	ReadIntervalMilliseconds int64 = 50
)

type RateLimitedReader struct {
	reader          io.ReadCloser
	limit           atomic.Int64
	iterTotalRead   atomic.Int64
	lastElapsed     atomic.Int64
	timeSlept       atomic.Int64
	timeAccumulated atomic.Int64
}

func NewRateLimitedReader(reader io.Reader, limit int64) *RateLimitedReader {
	return NewRateLimitedReadCloser(io.NopCloser(reader), limit)
}

func NewRateLimitedReadCloser(reader io.ReadCloser, limit int64) *RateLimitedReader {
	r := &RateLimitedReader{
		reader: reader,
	}

	r.limit.Store(limit)
	r.iterTotalRead.Store(0)
	r.lastElapsed.Store(0)
	r.timeSlept.Store(0)
	r.timeAccumulated.Store(0)
	return r
}

func (r *RateLimitedReader) Read(p []byte) (n int, err error) {
	r.iterTotalRead.Store(0)
	chunkSize := int64(len(p))
	for r.iterTotalRead.Load() < chunkSize {
		limit := r.limit.Load()
		if limit <= 0 {
			n, err = r.readWithoutLimit(p[r.iterTotalRead.Load():int(chunkSize)])
			r.iterTotalRead.Add(int64(n))
			return int(r.iterTotalRead.Load()), err
		}

		// the limit set to per second
		limit = limit / (1000 / ReadIntervalMilliseconds)

		allowedBytes := limit
		chunkSizeLeft := chunkSize - r.iterTotalRead.Load()
		if chunkSizeLeft < allowedBytes {
			allowedBytes = chunkSizeLeft
		}

		r.sleep(allowedBytes, limit)

		n, err = r.reader.Read(p[r.iterTotalRead.Load():int(r.iterTotalRead.Load()+allowedBytes)])
		r.iterTotalRead.Add(int64(n))
		if err != nil {
			break
		}
	}

	return int(r.iterTotalRead.Load()), err
}

func (r *RateLimitedReader) readWithoutLimit(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *RateLimitedReader) sleep(allowedBytes, iterLimit int64) {
	expectedTime := allowedBytes * ReadIntervalMilliseconds * int64(time.Millisecond) / iterLimit

	now := time.Now().UnixNano()
	elapsed := now - r.lastElapsed.Load() - r.timeSlept.Load()
	if elapsed > int64(time.Second) {
		elapsed = 0
		r.lastElapsed.Store(now)
		r.timeSlept.Store(0)
		r.timeAccumulated.Store(0)
	}

	sleepTime := r.timeAccumulated.Load() - (elapsed - expectedTime)
	if sleepTime > 0 {
		time.Sleep(time.Duration(sleepTime))
		r.timeAccumulated.Store(0)
		if elapsed == 0 {
			r.timeSlept.Add(sleepTime)
		} else {
			r.timeSlept.Store(0)
			r.lastElapsed.Store(now + sleepTime)
		}
	} else {
		r.timeAccumulated.Store(sleepTime)
		r.timeSlept.Store(0)
		r.lastElapsed.Store(now)
	}
}

func (r *RateLimitedReader) Close() error {
	return r.reader.Close()
}

func (r *RateLimitedReader) UpdateLimit(newLimit int64) {
	r.limit.Store(newLimit)
}

func (r *RateLimitedReader) GetCurrentIterTotalRead() int64 {
	return r.iterTotalRead.Load()
}
