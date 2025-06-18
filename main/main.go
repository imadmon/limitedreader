package main

import (
	"fmt"
	"math"
	"time"

	"github.com/imadmon/limitedreader"
)

const (
	executableFile    = "Sd"
	resultFile        = "results.json"
	durationInSeconds = 10
	bufferSize        = 32 * 1024
	limit             = math.MaxInt // large limit - no limit
)

func main() {
	fmt.Println(ReadPerformence())
}

func ReadPerformence() float64 {
	buffer := make([]byte, bufferSize)
	var totalBytes int64

	reader := infiniteReader{}
	lr := limitedreader.NewLimitedReader(reader, limit)

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
	// fmt.Printf("MaxReadOverTimeSyntheticTest: Read %.f MB in 10 seconds\n", totalBytesMB)
	return totalBytesMB
}

type infiniteReader struct{}

func (infiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'A'
	}
	return len(p), nil
}
