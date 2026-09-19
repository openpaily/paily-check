// Package deep implements detailed-check batch runners for the checker pipeline.
// Each file handles one phase (ping / geo / script / speed).
package deep

import (
	"context"

	"github.com/openpaily/paily-check/internal/checker/progress"
	"github.com/openpaily/paily-check/internal/checker/worker"
	"github.com/openpaily/paily-check/internal/client"
	"github.com/openpaily/paily-check/internal/config"
	"github.com/openpaily/paily-check/internal/interfaces"
	pingMacro "github.com/openpaily/paily-check/internal/macros/ping"
)

// RunPing runs a deep ping (avg=cfg.Avg) for each node concurrently, returning
// a map of nodeID → PingResult.
func RunPing(ctx context.Context, nodes []client.Node, vendorBase interfaces.Vendor, cfg *config.DeepPingConfig, tracker *progress.Tracker) map[string]pingMacro.PingResult {
	return worker.RunNodes(ctx, nodes, cfg.Concurrency, func(ctx context.Context, node client.Node) pingMacro.PingResult {
		defer tracker.Inc()
		v := vendorBase.Build(node.Raw)
		return pingMacro.Ping(ctx, v, pingMacro.PingConfig{
			URL:       cfg.PingURL,
			Avg:       cfg.Avg,
			TimeoutMS: cfg.TimeoutMS,
		})
	})
}
