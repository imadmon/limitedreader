package limitedreader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLimitedReaderBasicRateLimiting(t *testing.T) {
	const dataSize = 100 * 1024 // 100 KB
	const bufferSize = dataSize // one read call
	const partsAmount = 4
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, lr, bufferSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
}

func TestLimitedReaderDataHermetics(t *testing.T) {
	const dataSize = 100 * 1024 // 100 KB
	const bufferSize = dataSize // one read call
	const partsAmount = 2
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	key := "A"
	reader := strings.NewReader(strings.Repeat(key, dataSize))
	lr := NewLimitedReader(reader, int64(limit))

	start := time.Now()
	data := read(t, lr, bufferSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)

	for i := 0; i < dataSize; i++ {
		if string(data[i]) != key {
			t.Fatalf("read incorrect data, read: %v at i=%d expected: %v", data[i], i, key)
			break
		}
	}
}

func TestLimitedReaderNoLimitRead(t *testing.T) {
	const dataSize = 100 * 1024 // 100 KB
	const bufferSize = dataSize // one read call
	const limit = math.MaxInt   // large limit - no limit

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, limit)

	start := time.Now()
	read(t, lr, bufferSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), 0, 0)
}

func TestLimitedReaderMultipleReads(t *testing.T) {
	const dataSize = 100 * 1024 // 100 KB
	const bufferSize = 1024     // multiple times to call read for one limit
	const partsAmount = 2
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, lr, bufferSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
}

func TestLimitedReaderConcurrentReads(t *testing.T) {
	const dataSize = 100 * 1024 // 100 KB
	const bufferSize = 1024     // multiple times to call read for one limit
	const partsAmount = 4
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second
	const concurrentReadersAmount = 4

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < concurrentReadersAmount; i++ {
		wg.Add(1)
		go func() {
			read(t, lr, bufferSize, dataSize/concurrentReadersAmount, nil)
			wg.Done()
		}()
	}
	wg.Wait()
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
}

func TestLimitedReaderLargeRead(t *testing.T) {
	const dataSize = 1 * 1024 * 1024 * 1024 // 1 GB
	const bufferSize = 32 * 1024            // classic io.Copy
	const partsAmount = 3
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, lr, bufferSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
}

func TestLimitedReaderEOFBehavior(t *testing.T) {
	const dataSize = 1024           // 1 KB
	const bufferSize = dataSize * 2 // large buffer
	const limit = dataSize * 20     // large limit

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))

	read(t, lr, bufferSize, dataSize, nil)
	read(t, lr, bufferSize, 0, io.EOF)
}

func TestLimitedReaderUpdateLimit(t *testing.T) {
	const dataSize = 100 * 1024 // 100 KB
	const bufferSize = dataSize / 2
	const partsAmount = 4
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, lr, bufferSize, bufferSize, nil)

	lr.UpdateLimit(int64(limit) * 2) // update limit to cut time for the second half by half (minus 25% to the expected time)

	read(t, lr, bufferSize, bufferSize, nil)

	assertReadTimes(t, time.Since(start), int(float64(partsAmount)*0.75), int(float64(partsAmount)*0.75)+1)
}

func TestLimitedReaderGetTotalRead(t *testing.T) {
	const dataSize = 100 * 1024 // 100 KB
	const bufferSize = dataSize // one read call
	const partsAmount = 2
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))
	doneC := make(chan struct{}, 0)

	go func() {
		defer func() { doneC <- struct{}{} }()
		chunkSize := limit / (1000 / DefaultReadIntervalMilliseconds)
		for i := 0; i < partsAmount; i++ {
			select {
			case <-time.After(time.Second):
				currentTotalRead := lr.GetTotalRead()
				fmt.Printf("Total Read: %d , LimitAbs: %d  +- %d (chunk size)\n", currentTotalRead, limit, chunkSize)
				if currentTotalRead != int64(limit*(i+1)) &&
					currentTotalRead != int64(limit*(i+1))-chunkSize &&
					currentTotalRead != int64(limit*(i+1))+chunkSize {
					t.Fatalf("got unexpected CurrentTotalRead, read: %d expected: %d +- %d (chunk size)", currentTotalRead, limit*(i+1), chunkSize)
				}
			}
		}
	}()

	start := time.Now()
	read(t, lr, bufferSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
	<-doneC
}

func TestLimitedReaderUnconventionalLimitRead(t *testing.T) {
	const dataSize = 100 * 1024 // 100 KB
	const bufferSize = dataSize // one read call
	const partsAmount = 2
	const limit = dataSize/partsAmount - 3000 // unconventional limit

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, lr, dataSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
}

type mockReadCloser struct {
	closed bool
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}

func TestLimitedReadCloserClose(t *testing.T) {
	readCloser := mockReadCloser{}
	lr := NewLimitedReadCloser(&readCloser, 0)
	err := lr.Close()
	if err != nil {
		t.Fatalf("unexpected error while closing: %v", err)
	}

	if readCloser.closed == false {
		t.Fatalf("expected readCloser to be closed but wasn't")
	}
}

func TestLimitedReaderReadUnstableStream(t *testing.T) {
	const dataSize = 32 * 1024 // 32 KB buffer
	const bufferSize = 1024    // small buffer
	const partsAmount = 4
	const limit = dataSize / partsAmount

	iterLimit := limit / (1000 / DefaultReadIntervalMilliseconds)
	allowedBytes := iterLimit
	if bufferSize < allowedBytes {
		allowedBytes = bufferSize
	}
	expectedTime := allowedBytes * DefaultReadIntervalMilliseconds * int64(time.Millisecond) / iterLimit

	reader := randomSleepsReader{
		maxSleepTime: expectedTime,
	}
	lr := NewLimitedReader(reader, limit)

	start := time.Now()
	read(t, lr, bufferSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
}

type randomSleepsReader struct {
	maxSleepTime int64
}

func (r randomSleepsReader) Read(p []byte) (int, error) {
	s := rand.Int63n(r.maxSleepTime)
	time.Sleep(time.Duration(s))
	for i := range p {
		p[i] = 'A'
	}
	return len(p), nil
}

func TestLimitedReaderReadWithClock(t *testing.T) {
	const dataSize = 100 * 1024 // 100 KB
	const bufferSize = dataSize // one read call
	const partsAmount = 4
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit), WithClock(noSleepClock{}))

	start := time.Now()
	read(t, lr, bufferSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), 0, 1)
}

type noSleepClock struct{}

func (noSleepClock) Now() time.Time {
	return time.Now()
}

func (noSleepClock) Sleep(sleepTime time.Duration) {
	return
}

func TestLimitedReaderReadWithContext(t *testing.T) {
	const dataSize = 100 * 1024 // 100 KB
	const bufferSize = dataSize // one read call
	const partsAmount = 4
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	ctx, ctxCancel := context.WithCancel(context.Background())

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit), WithContext(ctx))

	ctxCancel()
	start := time.Now()
	read(t, lr, bufferSize, 0, context.Canceled)
	assertReadTimes(t, time.Since(start), 0, 1)
}

func TestLimitedReaderReadWithConfig(t *testing.T) {
	lr := NewLimitedReader(nil, 0)
	if lr.cfg.ReadIntervalMilliseconds != DefaultReadIntervalMilliseconds {
		t.Fatalf("got unexpected ReadIntervalMilliseconds, value: %d expected: %d", lr.cfg.ReadIntervalMilliseconds, DefaultReadIntervalMilliseconds)
	}

	var newReadInterval int64 = 100
	lr = NewLimitedReader(nil, 0, WithConfig(newReadInterval))
	if lr.cfg.ReadIntervalMilliseconds != newReadInterval {
		t.Fatalf("got unexpected ReadIntervalMilliseconds, value: %d expected: %d", lr.cfg.ReadIntervalMilliseconds, newReadInterval)
	}
}

func TestLimitedReaderZeroLimitBehavior(t *testing.T) {
	const dataSize = 100 * 1024 // 100 KB
	const bufferSize = dataSize // one read call
	const limit = 0
	const smallLimit = 2 // smaller then 1000 / lr.cfg.ReadIntervalMilliseconds default 20
	const delayLimit = 1 // should take delayLimit*2 seconds

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))
	doneC := make(chan struct{})

	start := time.Now()
	go func() {
		read(t, lr, bufferSize, dataSize, nil)
		doneC <- struct{}{}
	}()

	time.Sleep(delayLimit * time.Second)
	lr.UpdateLimit(smallLimit)
	time.Sleep(delayLimit * time.Second)
	lr.UpdateLimit(dataSize * 2)
	<-doneC

	assertReadTimes(t, time.Since(start), delayLimit*2, delayLimit*2+1)
}

func BenchmarkLimitedReaderNewLimitedReader(b *testing.B) {
	b.ReportAllocs()
	reader := infiniteReader{}
	for i := 0; i < b.N; i++ {
		_ = NewLimitedReader(reader, 0)
	}
}

func BenchmarkLimitedReaderThroughputAccuracy(b *testing.B) {
	const bufferSize = 32 * 1024   // 32 KB buffer
	const limit = 10 * 1024 * 1024 // 10 MB limit

	reader := infiniteReader{}
	lr := NewLimitedReader(reader, limit)
	buffer := make([]byte, bufferSize)

	var totalBytes int64
	start := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n, err := lr.Read(buffer)
		totalBytes += int64(n)
		if err != nil {
			b.Fatalf("read failed: %v", err)
		}
	}

	b.StopTimer()
	elapsed := time.Since(start)
	totalBytesMB := float64(totalBytes) / 1024.0 / 1024.0
	b.ReportMetric(totalBytesMB, "MB_read")
	b.ReportMetric(totalBytesMB/elapsed.Seconds(), "MB_read_per_second")
}

func BenchmarkLimitedReaderMaxThroughput(b *testing.B) {
	const bufferSize = 32 * 1024 // 32 KB buffer
	const limit = math.MaxInt    // large limit - no limit

	reader := infiniteReader{}
	lr := NewLimitedReader(reader, limit)
	buffer := make([]byte, bufferSize)

	var totalBytes int64
	start := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n, err := lr.Read(buffer)
		totalBytes += int64(n)
		if err != nil {
			b.Fatalf("read failed: %v", err)
		}
	}

	b.StopTimer()
	elapsed := time.Since(start)
	totalBytesMB := float64(totalBytes) / 1024.0 / 1024.0
	b.ReportMetric(totalBytesMB, "MB_read")
	b.ReportMetric(totalBytesMB/elapsed.Seconds(), "MB_read_per_second")
}

type infiniteReader struct{}

func (infiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'A'
	}
	return len(p), nil
}

func read(t *testing.T, reader *LimitedReader, bufferSize, expectedDataSize int, expectedErr error) []byte {
	dataSize := expectedDataSize
	if dataSize == 0 {
		dataSize = 1
	}

	data := make([]byte, dataSize)
	var err error
	var n int
	var total int
	for total < dataSize {
		readCapacity := total + bufferSize
		if readCapacity > dataSize {
			readCapacity = dataSize
		}

		n, err = reader.Read(data[total:dataSize])
		total += n
		if err != nil {
			break
		}
	}

	if err != expectedErr {
		t.Fatalf("unexpected error: %v, expected error: %v", err, expectedErr)
	}

	if total != expectedDataSize {
		t.Fatalf("read incomplete data, read: %d expected: %d", total, expectedDataSize)
	}

	return data
}

func assertReadTimes(t *testing.T, elapsed time.Duration, minTimeInSeconds, maxTimeInSeconds int) {
	fmt.Printf("Took %v\n", elapsed)
	minTime := time.Duration(minTimeInSeconds) * time.Second
	maxTime := time.Duration(maxTimeInSeconds) * time.Second
	if elapsed.Abs().Round(time.Second) < minTime { // round to second - has a deviation of up to half a second
		t.Errorf("read completed too quickly, elapsed time: %v < min time: %v", elapsed, minTime)
	} else if elapsed.Abs().Round(time.Second) > maxTime { // round to second - has a deviation of up to half a second
		t.Errorf("read completed too slow, elapsed time: %v > max time: %v", elapsed, maxTime)
	}
}
