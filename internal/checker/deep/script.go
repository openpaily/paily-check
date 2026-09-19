package deep

import (
	"context"

	"golang.org/x/sync/semaphore"

	"github.com/openpaily/paily-check/internal/checker/progress"
	"github.com/openpaily/paily-check/internal/checker/worker"
	"github.com/openpaily/paily-check/internal/client"
	"github.com/openpaily/paily-check/internal/config"
	"github.com/openpaily/paily-check/internal/interfaces"
	scriptMacro "github.com/openpaily/paily-check/internal/macros/script"
)

// RunScript runs all scripts against every node. It returns a nested
// map of nodeID → scriptName → unlocked bool.
//
// Outer concurrency: cfg.NodeConcurrency  (how many nodes at once).
// Inner concurrency: cfg.ScriptConcurrency (shared across all nodes,
// limiting total in-flight JS VMs simultaneously).
func RunScript(
	ctx context.Context,
	nodes []client.Node,
	vendorBase interfaces.Vendor,
	scripts []scriptMacro.ScriptEntry,
	cfg *config.ScriptConfig,
	tracker *progress.Tracker,
) map[string]map[string]bool {
	scriptConcurrency := cfg.ScriptConcurrency
	if scriptConcurrency <= 0 {
		scriptConcurrency = 32
	}
	// One global JS-VM semaphore shared across all node goroutines.
	globalSem := semaphore.NewWeighted(int64(scriptConcurrency))

	return worker.RunNodes(ctx, nodes, cfg.NodeConcurrency, func(ctx context.Context, node client.Node) map[string]bool {
		defer tracker.Inc()
		v := vendorBase.Build(node.Raw)
		return scriptMacro.RunAllWithEngine(ctx, v, scripts, globalSem, cfg.Engine)
	})
}
