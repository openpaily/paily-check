package script

import (
	"context"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/openpaily/paily-check/internal/interfaces"
)

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// ScriptEntry describes a single JS script to run against a node.
type ScriptEntry struct {
	Name      string
	Content   string
	TimeoutMS int
}

// EngineMode selects how scripts are executed.
type EngineMode string

const (
	// EngineGoja runs every script in goja.
	EngineGoja EngineMode = "goja"
	// EngineNative only uses registered native handlers; unsupported scripts fail closed.
	EngineNative EngineMode = "native"
	// EngineAuto tries a registered native handler first and falls back to goja.
	EngineAuto EngineMode = "auto"
)

// NativeHandler is a targeted Go implementation of one scripts/*.js file.
type NativeHandler func(ctx context.Context, v interfaces.Vendor, timeoutMS int) (bool, error)

var nativeHandlers = map[string]NativeHandler{}

// RegisterNativeHandler adds a script-native implementation. Names are matched
// case-insensitively against ScriptEntry.Name (the JS filename without .js).
func RegisterNativeHandler(name string, handler NativeHandler) {
	name = normalizeName(name)
	if name == "" || handler == nil {
		return
	}
	nativeHandlers[name] = handler

}

func nativeHandler(name string) (NativeHandler, bool) {
	h, ok := nativeHandlers[normalizeName(name)]
	return h, ok
}

// RunAll runs every script in entries concurrently (limited by sem), returning
// a map of scriptName → unlocked bool.
//
// sem limits total concurrent scripts across all callers.
// If sem is nil a default semaphore of size 32 is used.
func RunAll(ctx context.Context, v interfaces.Vendor, entries []ScriptEntry, sem *semaphore.Weighted) map[string]bool {
	return RunAllWithEngine(ctx, v, entries, sem, string(EngineGoja))
}

// RunAllWithEngine runs scripts using engine: goja (default), native, or auto.
// auto/native acquire the same semaphore so script_concurrency remains a global
// cap across both native network checks and goja VMs.
func RunAllWithEngine(ctx context.Context, v interfaces.Vendor, entries []ScriptEntry, sem *semaphore.Weighted, engine string) map[string]bool {
	if sem == nil {
		sem = semaphore.NewWeighted(32)
	}
	mode := parseEngineMode(engine)

	results := make(map[string]bool, len(entries))
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}

	for _, e := range entries {
		e := e
		wg.Add(1)
		go func() {
			defer wg.Done()

			if err := sem.Acquire(ctx, 1); err != nil {
				mu.Lock()
				results[e.Name] = false
				mu.Unlock()
				return
			}
			defer sem.Release(1)

			timeoutMS := uint64(e.TimeoutMS)
			if timeoutMS == 0 {
				timeoutMS = 10000
			}
			script := &interfaces.Script{
				Type:          interfaces.STypeMedia,
				Content:       e.Content,
				TimeoutMillis: timeoutMS,
			}
			unlocked := runEntry(ctx, v, e, script, mode)

			mu.Lock()
			results[e.Name] = unlocked
			mu.Unlock()
		}()
	}

	wg.Wait()
	return results
}

func runEntry(ctx context.Context, v interfaces.Vendor, entry ScriptEntry, jsScript *interfaces.Script, mode EngineMode) bool {
	if mode == EngineNative || mode == EngineAuto {
		if handler, ok := nativeHandler(entry.Name); ok {
			if ok, err := runNativeWithTimeout(ctx, handler, v, entry.TimeoutMS); err == nil {
				return ok
			}
		}
		if mode == EngineNative {
			return false
		}
	}

	return ExecScript(v, jsScript).Unlocked
}

func runNativeWithTimeout(ctx context.Context, handler NativeHandler, v interfaces.Vendor, timeoutMS int) (bool, error) {
	if timeoutMS <= 0 {
		timeoutMS = 10000
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	return handler(runCtx, v, timeoutMS)
}

func parseEngineMode(engine string) EngineMode {
	switch EngineMode(normalizeName(engine)) {
	case EngineNative:
		return EngineNative
	case EngineAuto:
		return EngineAuto
	default:
		return EngineGoja
	}
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
