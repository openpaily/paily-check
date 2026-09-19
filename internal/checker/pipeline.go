// Package checker provides the top-level node-check pipeline.
package checker

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openpaily/paily-check/internal/checker/deep"
	"github.com/openpaily/paily-check/internal/checker/initial"
	"github.com/openpaily/paily-check/internal/checker/progress"
	"github.com/openpaily/paily-check/internal/client"
	"github.com/openpaily/paily-check/internal/config"
	"github.com/openpaily/paily-check/internal/interfaces"
	scriptMacro "github.com/openpaily/paily-check/internal/macros/script"
)

// Run executes the full check pipeline and returns one CheckResult per node.
//
// Pipeline order:
//
//	initial latency → filter → detailed latency → geo → scripts → optional speed
//
// Each check result is merged into the final []client.CheckResult slice.
func Run(ctx context.Context, nodes []client.Node, v interfaces.Vendor, cfg *config.Config) []client.CheckResult {
	total := len(nodes)

	// ------------------------------------------------------------------
	// Load third-party scripts from disk once before checks start.
	// ------------------------------------------------------------------
	scripts, err := loadScripts(cfg.Deep.Script.Dir, cfg.Deep.Script.TimeoutMS, cfg.Deep.Script.Scripts)
	if err != nil {
		log.Printf("[pipeline] warning: failed to load scripts from %q: %v", cfg.Deep.Script.Dir, err)
	}

	tracker := progress.New(cfg.ProgressBar.Enabled)

	// ------------------------------------------------------------------
	// Initial latency check
	// ------------------------------------------------------------------
	t0 := time.Now()
	tracker.Start("Initial Ping", total)
	initialResults := initial.RunAll(ctx, nodes, v, &cfg.Initial, tracker)
	tracker.Done()
	initialDuration := time.Since(t0)

	// Count alive nodes.
	threshold := cfg.Initial.AliveThresholdMS
	var aliveNodes []client.Node
	resultByID := make(map[string]*client.CheckResult, total)

	for i := range initialResults {
		r := &initialResults[i]
		resultByID[r.NodeID] = r
		lms := r.Initial.LatencyMS
		if lms != -1 && (threshold <= 0 || lms <= threshold) {
			// Find the corresponding Node struct.
			for _, n := range nodes {
				if n.ID == r.NodeID {
					aliveNodes = append(aliveNodes, n)
					break
				}
			}
		}
	}

	alive := len(aliveNodes)
	logMetrics("Initial latency",
		fmt.Sprintf("total=%d alive=%d", total, alive),
		initialDuration, total)

	if alive == 0 {
		log.Printf("[pipeline] no alive nodes after initial latency check")
		return initialResults
	}

	// ------------------------------------------------------------------
	// Detailed latency check
	// ------------------------------------------------------------------
	t0 = time.Now()
	tracker.Start("Deep Ping", alive)
	pingResults := deep.RunPing(ctx, aliveNodes, v, &cfg.Deep.Ping, tracker)
	tracker.Done()
	logMetrics("Detailed latency", fmt.Sprintf("total=%d", alive), time.Since(t0), alive)

	// ------------------------------------------------------------------
	// Optional: drop nodes whose deep ping returned -1 from all
	// subsequent phases AND from aliveNodes itself.
	// ------------------------------------------------------------------
	if cfg.Deep.Ping.FilterDeadDeep {
		filtered := 0
		kept := aliveNodes[:0]
		for _, node := range aliveNodes {
			if pr, ok := pingResults[node.ID]; ok && pr.LatencyMS == -1 {
				// Treat as initial failure: overwrite latency to -1 so the
				// caller sees a uniformly dead node; keep the record in the
				// output and skip all remaining deep phases.
				if r, ok := resultByID[node.ID]; ok {
					r.Initial.LatencyMS = -1
				}
				filtered++
			} else {
				kept = append(kept, node)
			}
		}
		aliveNodes = kept
		if filtered > 0 {
			log.Printf("[pipeline] filter_dead_deep: dropped %d node(s) with deep latency=-1", filtered)
		}
		alive = len(aliveNodes)
	}

	// ------------------------------------------------------------------
	// Geo detection
	// ------------------------------------------------------------------
	t0 = time.Now()
	tracker.Start("Geo Detection", alive)
	geoResults := deep.RunGeo(ctx, aliveNodes, v, &cfg.Deep.Geo, tracker)
	tracker.Done()
	logMetrics("Geo detection", fmt.Sprintf("total=%d", alive), time.Since(t0), alive)

	// ------------------------------------------------------------------
	// Third-party scripts
	// ------------------------------------------------------------------
	scriptNodes := filterScriptNodesByRegion(aliveNodes, geoResults, cfg.Deep.Script.Regions)
	if len(scriptNodes) != len(aliveNodes) {
		log.Printf("[pipeline] script region filter: kept %d/%d node(s) for regions=%v", len(scriptNodes), len(aliveNodes), cfg.Deep.Script.Regions)
	}
	t0 = time.Now()
	tracker.Start("Script Check", len(scriptNodes))
	scriptResults := deep.RunScript(ctx, scriptNodes, v, scripts, &cfg.Deep.Script, tracker)
	tracker.Done()
	logMetrics("Script checks", fmt.Sprintf("total=%d", len(scriptNodes)), time.Since(t0), len(scriptNodes))

	// ------------------------------------------------------------------
	// Optional speed test
	// ------------------------------------------------------------------
	var speedResults map[string]int
	if cfg.Deep.Speed.Enabled {
		t0 = time.Now()
		tracker.Start("Speed Test", alive)
		speedResults = deep.RunSpeed(ctx, aliveNodes, v, &cfg.Deep.Speed, tracker)
		tracker.Done()
		logMetrics("Speed test", fmt.Sprintf("total=%d", alive), time.Since(t0), alive)
	} else {
		log.Printf("[speed test] disabled")
	}

	// ------------------------------------------------------------------
	// Assemble final results
	// ------------------------------------------------------------------
	for _, node := range aliveNodes {
		r, ok := resultByID[node.ID]
		if !ok {
			continue
		}

		// Use a distinct name to avoid shadowing the deep checker package import.
		dr := &client.DeepResult{}

		if pr, ok := pingResults[node.ID]; ok {
			dr.AvgLatencyMS = pr.LatencyMS
			dr.JitterMS = pr.JitterMS
		}

		if region, ok := geoResults[node.ID]; ok {
			dr.Region = region
		}

		if sr, ok := scriptResults[node.ID]; ok {
			dr.Streaming = sr
		}

		if speedResults == nil {
			kv := -1
			dr.SpeedKbps = &kv
		} else {
			if spd, ok := speedResults[node.ID]; ok {
				kv := spd
				dr.SpeedKbps = &kv
			}
		}

		r.Deep = dr
	}

	return initialResults
}

// filterScriptNodesByRegion keeps script checks focused after geo detection.
// Empty regions means no filtering. Region matching is case-insensitive and
// accepts the special value "unknown" for nodes with an empty geo result.
func filterScriptNodesByRegion(nodes []client.Node, geoResults map[string]string, regions []string) []client.Node {
	if len(regions) == 0 {
		return nodes
	}

	allowed := make(map[string]struct{}, len(regions))
	for _, region := range regions {
		region = normalizeRegion(region)
		if region == "" {
			continue
		}
		allowed[region] = struct{}{}
	}
	if len(allowed) == 0 {
		return nodes
	}

	filtered := make([]client.Node, 0, len(nodes))
	for _, node := range nodes {
		region := normalizeRegion(geoResults[node.ID])
		if region == "" {
			region = "UNKNOWN"
		}
		if _, ok := allowed[region]; ok {
			filtered = append(filtered, node)
		}
	}
	return filtered

}

func normalizeRegion(region string) string {
	return strings.ToUpper(strings.TrimSpace(region))
}

// loadScripts reads all *.js files from dir and returns ScriptEntry slice.
// timeoutMS is applied uniformly across all scripts.
func loadScripts(dir string, timeoutMS int, selected []string) ([]scriptMacro.ScriptEntry, error) {
	if dir == "" {
		return nil, nil
	}
	if timeoutMS <= 0 {
		timeoutMS = 10000
	}

	selectedSet := make(map[string]struct{}, len(selected))
	for _, name := range selected {
		name = normalizeScriptName(name)
		if name == "" {
			continue
		}
		selectedSet[name] = struct{}{}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", dir, err)
	}

	var scripts []scriptMacro.ScriptEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".js" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[pipeline] skipping %s: %v", path, err)
			continue
		}
		// Script name is the filename without extension.
		name := e.Name()[:len(e.Name())-len(".js")]
		if len(selectedSet) > 0 {
			if _, ok := selectedSet[normalizeScriptName(name)]; !ok {
				continue
			}
		}
		scripts = append(scripts, scriptMacro.ScriptEntry{
			Name:      name,
			Content:   string(data),
			TimeoutMS: timeoutMS,
		})
	}
	return scripts, nil
}

func normalizeScriptName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// logMetrics prints a single-line metric summary for a pipeline phase.
func logMetrics(phase string, extra string, dur time.Duration, nodeCount int) {
	rate := 0.0
	if dur.Seconds() > 0 {
		rate = float64(nodeCount) / dur.Seconds()
	}
	log.Printf("[%-22s] %s duration=%.1fs rate=%.1f nodes/s",
		phase, extra, dur.Seconds(), rate)
}
