# Limited Reader

**Deterministic, real-time friendly rate limiting for Go readers**

Built for systems that care about _tempo_, _predictability_, and _adaptability_.

</br>


## Why?

Traditional rate limiters (like token buckets) are great for handling bursts - but in real-time, bandwidth-sensitive systems, you sometimes need something stricter, more _predictable_.
</br>
This library wraps your `io.Reader` and ensures a consistent read rate over time, with optional support for **adaptive control** based on bandwidth constraints.

If you care about:
- **Real-time stability**
- **Network fairness**
- **Controlled data streaming**

This is for you.

</br>


## Features

- **Deterministic rate limiting** (not burstable)

- Simple `io.Reader` wrapping

- Supports dynamic rate adjustments

- Designed for smart bandwidth control systems

- Lightweight & dependency-free (just Go stdlib)

</br>


## Installation

```bash
go get github.com/imadmon/limitedreader
```

</br>

## Usage

```Go
package main

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/imadmon/limitedreader"
)

func main() {
	const chunkSize = 1000 * 1000
	dataSize := 32 * chunkSize
	reader := bytes.NewBuffer(make([]byte, dataSize))

	// reading 8 chunkSize will take a second
	// should take 32 / 8 = 4 seconds
	// reads interval are evenly spaced by limitedreader.ReadIntervalMilliseconds
	limit := 8 * chunkSize
	lr := limitedreader.NewLimitedReader(reader, int64(limit))

	var total int
	buffer := make([]byte, chunkSize)
	start := time.Now()
	for {
		n, err := lr.Read(buffer)
		total += n
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Error: %v\n", err)
			}
			break
		}
	}

	elapsed := time.Since(start)
	fmt.Printf("Total: %d, Elapsed: %s\n", total, elapsed)
}
```

</br>


## When should you use it?

You want **smooth data streaming** with no traffic spikes

You're feeding data into a **real-time processor**, encoder, or network stream

You're building systems that adapt their throughput over time (e.g., **smart bandwidth control**)

You're tired of fiddling with `golang.org/x/time/rate` just to get stable pacing

</br>


## Benchmarks

Want to see how it stacks up against other Go libraries like 
[`golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate), 
[`uber-go/ratelimit`](https://github.com/uber-go/ratelimit) or 
[`juju/ratelimit`](https://github.com/juju/ratelimit) ?
</br>
> Check out the `comparison & benchmark article here` - complete with graphs, scenarios, and flame.

</br>


## Looking for automatic bandwidth adjustment?

Check out [`github/imadmon/bandwidthcontroller`](https://github.com/imadmon/bandwidthcontroller) (by the same author) - a library built on top of `limitedreader`, designed for real-time adaptive systems.
</br>
It **dynamically probes bandwidth** and adjusts read rates accordingly, with a focus on reliability and minimal latency overhead.

This is especially useful for:
- IoT edge devices
- Real-time data ingestion pipelines
- Video/audio streamers
- Any system that must adapt to **changing network conditions**


</br>

## Real-world Performance Notice

> Because this library is extremely lightweight and low-level, its performance is tightly coupled with your system's hardware

While the library strives to be as efficient as possible, the **actual read throughput** you get depends heavily on:
- The **disk or network speed** of your system
- The **CPU capacity and load**
- Whether your system is virtualized or containerized
- And even the **runtime environment** (e.g., Docker vs bare metal)

This means that benchmark results will vary dramatically between:
- A minimal VPS with shared resources
- A production-grade server with a powerful CPU
- A developer machine with a local SSD and low latency

Benchmarks in this repository are designed to be comparative, not absolute - they help you compare different rate limiters **on the same machine**, not between different environments.

Basically if you're benchmarking on a Raspberry Pi - don't expect it to perform like a 128-core monster


</br>

## License

This project is licensed under the MIT License - see the [LICENSE](./LICENSE) file for details. 

Use it, fork it, build cool stuff.


</br>

## Author

Created by [@IdanMadmon](https://github.com/imadmon)
</br>
Got ideas? Want to integrate this into a bandwidth control stack? Open an issue or reach out.
