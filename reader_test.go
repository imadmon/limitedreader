package limitedreader

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestRateLimitedReader_BasicRead(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
	const bufferSize = dataSize // one read call
	const partsAmount = 4
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	ratelimitedReader := NewRateLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, ratelimitedReader, bufferSize, dataSize)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
}

func TestRateLimitedReader_DataHermetics(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
	const bufferSize = dataSize // one read call
	const partsAmount = 4
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	key := "A"
	reader := strings.NewReader(strings.Repeat(key, dataSize))
	ratelimitedReader := NewRateLimitedReader(reader, int64(limit))

	start := time.Now()
	data, err := read(t, ratelimitedReader, bufferSize, dataSize)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < dataSize; i++ {
		if string(data[i]) != key {
			t.Fatalf("read incorrect data, read: %v at i=%d expected: %v", data[i], i, key)
			break
		}
	}
}

func TestRateLimitedReader_NoLimitRead(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
	const bufferSize = dataSize // one read call
	const limit = 0             // no limit

	reader := bytes.NewReader(make([]byte, dataSize))
	ratelimitedReader := NewRateLimitedReader(reader, limit)

	start := time.Now()
	read(t, ratelimitedReader, bufferSize, dataSize)
	assertReadTimes(t, time.Since(start), 0, 0)
}

func TestRateLimitedReader_MultipleReads(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
	const bufferSize = 1024     // multiple times to call read for one limit
	const partsAmount = 2
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	ratelimitedReader := NewRateLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, ratelimitedReader, bufferSize, dataSize)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
}

func TestRateLimitedReader_LargeRead(t *testing.T) {
	const dataSize = 1 * 1024 * 1024 * 1024 // 1GB
	const bufferSize = 32 * 1024            // classic io.Copy
	const partsAmount = 4
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	ratelimitedReader := NewRateLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, ratelimitedReader, bufferSize, dataSize)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
}

func TestRateLimitedReader_EOFBehavior(t *testing.T) {
	const dataSize = 1024           // 1KB
	const bufferSize = dataSize * 2 // large buffer
	const limit = dataSize * 20     // large limit

	reader := bytes.NewReader(make([]byte, dataSize))
	ratelimitedReader := NewRateLimitedReader(reader, int64(limit))

	read(t, ratelimitedReader, bufferSize, dataSize)
	read(t, ratelimitedReader, bufferSize, 0)
}

func TestRateLimitedReader_UpdateLimit(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
	const bufferSize = dataSize / 2
	const partsAmount = 4
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	ratelimitedReader := NewRateLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, ratelimitedReader, bufferSize, bufferSize)

	ratelimitedReader.UpdateLimit(int64(limit) * 2) // update limit to cut time for the second half by half (minus 25% to the expected time)

	read(t, ratelimitedReader, bufferSize, bufferSize)

	assertReadTimes(t, time.Since(start), int(float64(partsAmount)*0.75), int(float64(partsAmount)*0.75)+1)
}

func TestRateLimitedReader_GetCurrentTotalRead(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
	const bufferSize = dataSize // one read call
	const partsAmount = 4
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	ratelimitedReader := NewRateLimitedReader(reader, int64(limit))
	doneC := make(chan struct{}, 0)

	go func() {
		defer func() { doneC <- struct{}{} }()
		chunkSize := limit / (1000 / ReadIntervalMilliseconds)
		for i := 0; i < partsAmount; i++ {
			select {
			case <-time.After(time.Second):
				currentTotalRead := ratelimitedReader.GetCurrentIterTotalRead()
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
	read(t, ratelimitedReader, bufferSize, dataSize)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
	<-doneC
}

func TestRateLimitedReader_UnconventionalLimitRead(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
	const bufferSize = dataSize // one read call
	const partsAmount = 2
	const limit = dataSize/partsAmount - 3000 // unconventional limit

	reader := bytes.NewReader(make([]byte, dataSize))
	ratelimitedReader := NewRateLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, ratelimitedReader, dataSize, dataSize)
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

func TestRateLimitedReadCloser_Close(t *testing.T) {
	readCloser := mockReadCloser{}
	ratelimitedReadCloser := NewRateLimitedReadCloser(&readCloser, 0)
	err := ratelimitedReadCloser.Close()
	if err != nil {
		t.Fatalf("unexpected error while closing: %v", err)
	}

	if readCloser.closed == false {
		t.Fatalf("expected readCloser to be closed but wasn't")
	}
}

func TestRateLimitedReader_ReadUnstableStream(t *testing.T) {
	const dataSize = 32 * 1024 // 32KB buffer
	const bufferSize = 1024    // small buffer
	const partsAmount = 4
	const limit = dataSize / partsAmount

	iterLimit := limit / (1000 / ReadIntervalMilliseconds)
	allowedBytes := iterLimit
	if bufferSize < allowedBytes {
		allowedBytes = bufferSize
	}
	expectedTime := allowedBytes * ReadIntervalMilliseconds * int64(time.Millisecond) / iterLimit

	reader := randomSleepsReader{
		maxSleepTime: expectedTime,
	}
	ratelimitedReader := NewRateLimitedReader(reader, limit)

	start := time.Now()
	read(t, ratelimitedReader, bufferSize, dataSize)
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

func TestRateLimitedReader_ReadPerformence(t *testing.T) {
	const durationInSeconds = 10
	const bufferSize = 32 * 1024    // 32KB buffer
	const limit = bufferSize * 1000 // large limit
	fmt.Printf("Duration set: %d seconds\n", durationInSeconds)

	buffer := make([]byte, bufferSize)
	var totalBytes int64

	reader := infiniteReader{}
	ratelimitedReader := NewRateLimitedReader(reader, limit) // large limit - no limit
	deadline := time.Now().Add(durationInSeconds * time.Second)

	for time.Now().Before(deadline) {
		n, err := ratelimitedReader.Read(buffer)
		if n > 0 {
			totalBytes += int64(n)
		}
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			break
		}
	}

	deviation := 0.95
	if totalBytes < int64(limit*durationInSeconds*deviation) {
		t.Fatalf("read incomplete data")
	}

	mb := float64(totalBytes) / 1024.0 / 1024.0
	fmt.Printf("MaxReadOverTimeSyntheticTest: Read %.4f MB in 10 seconds, read: %d expected at least: %d, expected with %.2f deviation: %d\n", mb, totalBytes, limit*durationInSeconds, deviation, int64(limit*durationInSeconds*deviation))
}

type infiniteReader struct{}

func (infiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'A'
	}
	return len(p), nil
}

func read(t *testing.T, reader *RateLimitedReader, bufferSize, expectedDataSize int) ([]byte, error) {
	data := make([]byte, expectedDataSize)
	total := 0
	for total < expectedDataSize {
		readCapacity := total + bufferSize
		if readCapacity > expectedDataSize {
			readCapacity = expectedDataSize
		}

		n, err := reader.Read(data[total:readCapacity])
		total += n
		if err != nil {
			if err != io.EOF {
				t.Fatalf("unexpected error: %v", err)
				return nil, err
			}
			break
		}
	}

	if total != expectedDataSize {
		t.Fatalf("read incomplete data, read: %d expected: %d", total, expectedDataSize)
	}

	return data, nil
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
