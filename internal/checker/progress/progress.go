// Package progress provides a single-line terminal progress indicator for the
// checker pipeline. All exported methods are nil-safe and concurrency-safe.
// When enabled is false every method is a no-op.
package progress

import (
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	barWidth   = 16
	renderTick = 100 * time.Millisecond
	logStepPct = 20
	// clearWidth is the minimum width padded when finishing a line to
	// overwrite any previously longer line.
	clearWidth = 100
)

// Tracker renders a live single-line progress indicator to stderr.
// Create one with New and reuse across pipeline phases via Start.
type Tracker struct {
	enabled bool

	mu         sync.Mutex
	phase      string
	total      int
	start      time.Time
	nextLogPct int

	done   atomic.Int32
	stopCh chan struct{}
	doneCh chan struct{}
}

// New returns a Tracker. Pass enabled=false for a no-op instance.
func New(enabled bool) *Tracker {
	return &Tracker{enabled: enabled}
}

// Start begins tracking a new phase. Any previous render loop is stopped first.
// total is the expected number of Inc() calls for this phase.
func (t *Tracker) Start(phase string, total int) {
	if t == nil {
		return
	}
	if !t.enabled {
		t.mu.Lock()
		t.phase = phase
		t.total = total
		t.start = time.Now()
		t.nextLogPct = logStepPct
		t.done.Store(0)
		t.mu.Unlock()
		return
	}
	t.internalStop()

	t.mu.Lock()
	t.phase = phase
	t.total = total
	t.start = time.Now()
	t.nextLogPct = logStepPct
	t.done.Store(0)
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	t.stopCh = stopCh
	t.doneCh = doneCh
	t.mu.Unlock()

	go t.loop(stopCh, doneCh)
}

// Inc marks one node as processed. Safe to call from any goroutine.
func (t *Tracker) Inc() {
	if t == nil {
		return
	}
	done := int(t.done.Add(1))
	if !t.enabled {
		t.maybeLogProgress(done)
	}
}

// Done stops the render loop and prints the final state with a trailing newline.
func (t *Tracker) Done() {
	if t == nil {
		return
	}
	if !t.enabled {
		t.logFinalProgress()
		return
	}
	t.internalStop()
}

func (t *Tracker) maybeLogProgress(done int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.total <= 0 {
		return
	}
	pct := done * 100 / t.total
	if pct < t.nextLogPct {
		return
	}
	for pct >= t.nextLogPct {
		t.nextLogPct += logStepPct
	}
	if t.nextLogPct > 100 {
		t.nextLogPct = 100
	}
	elapsed := time.Since(t.start)
	rate := 0.0
	if s := elapsed.Seconds(); s > 0 {
		rate = float64(done) / s
	}
	etaStr := "--"
	if rate > 0 && done < t.total {
		remaining := time.Duration(float64(time.Second) * float64(t.total-done) / rate)
		etaStr = fmtDuration(remaining)
	} else if done >= t.total {
		etaStr = "done"
	}
	fmt.Fprintf(os.Stderr, "[%-20s] progress %d/%d (%d%%) elapsed=%s eta=%s %.1fn/s\n",
		t.phase, min(done, t.total), t.total, min(pct, 100), fmtDuration(elapsed), etaStr, rate,
	)
}

func (t *Tracker) logFinalProgress() {
	t.mu.Lock()
	phase := t.phase
	total := t.total
	start := t.start
	t.mu.Unlock()

	done := int(t.done.Load())
	elapsed := time.Since(start)
	rate := 0.0
	if s := elapsed.Seconds(); s > 0 {
		rate = float64(done) / s
	}
	pct := 100
	if total > 0 {
		pct = min(done*100/total, 100)
	}
	fmt.Fprintf(os.Stderr, "[%-20s] progress %d/%d (%d%%) elapsed=%s eta=done %.1fn/s\n",
		phase, max(min(done, total), 0), total, pct, fmtDuration(elapsed), rate,
	)
}

// internalStop signals the render goroutine to finish and waits for it.
func (t *Tracker) internalStop() {
	t.mu.Lock()
	s := t.stopCh
	d := t.doneCh
	t.stopCh = nil
	t.doneCh = nil
	t.mu.Unlock()

	if s != nil {
		close(s)
		<-d
	}
}

func (t *Tracker) loop(stopCh, doneCh chan struct{}) {
	defer close(doneCh)

	ticker := time.NewTicker(renderTick)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			t.render(true)
			return
		case <-ticker.C:
			t.render(false)
		}
	}
}

func (t *Tracker) render(final bool) {
	t.mu.Lock()
	phase := t.phase
	total := t.total
	start := t.start
	t.mu.Unlock()

	done := int(t.done.Load())
	elapsed := time.Since(start)

	var pct float64
	if total > 0 {
		pct = float64(done) / float64(total)
		if pct > 1 {
			pct = 1
		}
	}

	filled := int(math.Round(float64(barWidth) * pct))
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	rate := 0.0
	if s := elapsed.Seconds(); s > 0 {
		rate = float64(done) / s
	}

	var etaStr string
	switch {
	case rate > 0 && done < total:
		remaining := time.Duration(float64(time.Second) * float64(total-done) / rate)
		etaStr = fmtDuration(remaining)
	case total > 0 && done >= total:
		etaStr = "done"
	default:
		etaStr = "--"
	}

	line := fmt.Sprintf("[%-20s] %s %d/%d (%3.0f%%) elapsed=%-6s eta=%-6s %.1fn/s",
		phase, bar, done, total, pct*100,
		fmtDuration(elapsed), etaStr, rate,
	)

	if final {
		// Erase the progress line so the caller's log output takes its place.
		fmt.Fprintf(os.Stderr, "\r%-*s\r", clearWidth, "")
	} else {
		fmt.Fprintf(os.Stderr, "\r%s", line)
	}
}

func fmtDuration(d time.Duration) string {
	s := d.Seconds()
	if s < 60 {
		return fmt.Sprintf("%.1fs", s)
	}
	m := int(s) / 60
	return fmt.Sprintf("%dm%02.0fs", m, math.Mod(s, 60))
}
