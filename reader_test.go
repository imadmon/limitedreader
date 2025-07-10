package limitedreader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestLimitedReader_BasicRead(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
	const bufferSize = dataSize // one read call
	const partsAmount = 4
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, lr, bufferSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
}

func TestLimitedReader_DataHermetics(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
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

func TestLimitedReader_NoLimitRead(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
	const bufferSize = dataSize // one read call
	const limit = 0             // no limit

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, limit)

	start := time.Now()
	read(t, lr, bufferSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), 0, 0)
}

func TestLimitedReader_MultipleReads(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
	const bufferSize = 1024     // multiple times to call read for one limit
	const partsAmount = 2
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, lr, bufferSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
}

func TestLimitedReader_LargeRead(t *testing.T) {
	const dataSize = 1 * 1024 * 1024 * 1024 // 1GB
	const bufferSize = 32 * 1024            // classic io.Copy
	const partsAmount = 3
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))

	start := time.Now()
	read(t, lr, bufferSize, dataSize, nil)
	assertReadTimes(t, time.Since(start), partsAmount, partsAmount+1)
}

func TestLimitedReader_EOFBehavior(t *testing.T) {
	const dataSize = 1024           // 1KB
	const bufferSize = dataSize * 2 // large buffer
	const limit = dataSize * 20     // large limit

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))

	read(t, lr, bufferSize, dataSize, nil)
	read(t, lr, bufferSize, 0, io.EOF)
}

func TestLimitedReader_UpdateLimit(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
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

func TestLimitedReader_GetCurrentTotalRead(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
	const bufferSize = dataSize // one read call
	const partsAmount = 2
	const limit = dataSize / partsAmount // dataSize/partsAmount bytes per second

	reader := bytes.NewReader(make([]byte, dataSize))
	lr := NewLimitedReader(reader, int64(limit))
	doneC := make(chan struct{}, 0)

	go func() {
		defer func() { doneC <- struct{}{} }()
		chunkSize := limit / (1000 / ReadIntervalMilliseconds)
		for i := 0; i < partsAmount; i++ {
			select {
			case <-time.After(time.Second):
				currentTotalRead := lr.GetCurrentIterTotalRead()
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

func TestLimitedReader_UnconventionalLimitRead(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
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

func TestLimitedReadCloser_Close(t *testing.T) {
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

func TestLimitedReader_ReadUnstableStream(t *testing.T) {
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

func TestLimitedReader_ReadPerformance(t *testing.T) {
	const durationInSeconds = 5
	const bufferSize = 32 * 1024 // 32KB buffer
	const limit = math.MaxInt    // large limit - no limit
	fmt.Printf("Duration set: %d seconds\n", durationInSeconds)

	buffer := make([]byte, bufferSize)
	var totalBytes int64

	reader := infiniteReader{}
	lr := NewLimitedReader(reader, limit)

	deadline := time.Now().Add(durationInSeconds * time.Second)
	for time.Now().Before(deadline) {
		n, err := lr.Read(buffer)
		if n > 0 {
			totalBytes += int64(n)
		}
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			break
		}
	}

	totalBytesMB := float64(totalBytes) / 1024.0 / 1024.0
	fmt.Printf("MaxReadOverTimeSyntheticTest: Read %.f MB in %d seconds\n", totalBytesMB, durationInSeconds)
}

type infiniteReader struct{}

func (infiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'A'
	}
	return len(p), nil
}

func TestLimitedReader_ReadWithClock(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
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

func TestLimitedReader_ReadWithContext(t *testing.T) {
	const dataSize = 100 * 1024 // 100KB
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
