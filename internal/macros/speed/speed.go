// Package speed implements a download-only speed test through a Vendor proxy.
package speed

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/openpaily/paily-check/internal/interfaces"
)

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// SpeedConfig carries parameters for a single Test() call.
type SpeedConfig struct {
	URL       string // download URL
	Threads   int    // parallel download goroutines (≥ 1)
	DurationS int    // test duration in seconds (≥ 1)
}

// Test downloads from cfg.URL for cfg.DurationS seconds using cfg.Threads
// parallel connections through the vendor proxy, returning the average
// download speed in kbps.
// Returns 0 on failure.
func Test(ctx context.Context, v interfaces.Vendor, cfg SpeedConfig) int {
	if v == nil || v.Status() == interfaces.VStatusNotReady {
		return 0
	}
	url := cfg.URL
	if url == "" {
		return 0
	}
	threads := cfg.Threads
	if threads < 1 {
		threads = 1
	}
	dur := cfg.DurationS
	if dur < 1 {
		dur = 8
	}

	// Per-second byte buckets, one per second of the test.
	buckets := make([]uint64, dur)

	// counters is a slice of WriteCounter pointers,  one per thread.
	counters := make([]*writeCounter, threads)
	for i := range counters {
		counters[i] = &writeCounter{}
	}

	// Start each download goroutine.
	ctx, cancel := context.WithTimeout(ctx, time.Duration(dur+2)*time.Second)
	defer cancel()

	var initWG sync.WaitGroup
	for i := 0; i < threads; i++ {
		i := i
		initWG.Add(1)
		go func() {
			defer initWG.Done()
			downloadLoop(ctx, v, url, counters[i])
		}()
	}
	// Let goroutines start before sampling (give them 200 ms to connect).
	time.Sleep(200 * time.Millisecond)

	// Sample each second.
	for t := 0; t < dur; t++ {
		time.Sleep(time.Second)
		var b uint64
		for _, c := range counters {
			b += c.take()
		}
		buckets[t] = b
	}
	cancel()
	initWG.Wait()

	// Average over all non-zero seconds.
	var total uint64
	count := 0
	for _, b := range buckets {
		if b > 0 {
			total += b
			count++
		}
	}
	if count == 0 {
		return 0
	}
	avgBytesPerSec := total / uint64(count)
	// Convert bytes/s → kbps (kilobits per second, 1 kbps = 1000 bits/s)
	fmt.Printf("[speed] node=%s avg_speed=%dkbps\n", v.ProxyInfo().Name, int(avgBytesPerSec*8/1000))
	kbps := int(avgBytesPerSec * 8 / 1000)
	return kbps
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

type writeCounter struct {
	mu    sync.Mutex
	total uint64
}

func (wc *writeCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.mu.Lock()
	wc.total += uint64(n)
	wc.mu.Unlock()
	return n, nil
}

func (wc *writeCounter) take() uint64 {
	wc.mu.Lock()
	t := wc.total
	wc.total = 0
	wc.mu.Unlock()
	return t
}

// downloadLoop opens an HTTP GET to url through the vendor, copies the body
// into wc (which just counts bytes), and repeats until ctx is cancelled.
func downloadLoop(ctx context.Context, v interfaces.Vendor, url string, wc *writeCounter) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := downloadOnce(ctx, v, url, wc); err != nil {
			// short pause before retry to avoid tight loop on persistent failure
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}

func downloadOnce(ctx context.Context, v interfaces.Vendor, dlURL string, wc *writeCounter) error {
	transport := &http.Transport{
		DialContext: func(dCtx context.Context, _, _ string) (net.Conn, error) {
			return v.DialTCP(dCtx, dlURL, interfaces.ROptionsTCP)
		},
		ResponseHeaderTimeout: 10 * time.Second,
		DisableKeepAlives:     true,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err := io.Copy(wc, resp.Body); err != nil {
		if ctx.Err() != nil {
			return nil // expected cancellation
		}
		return err
	}
	return nil
}
