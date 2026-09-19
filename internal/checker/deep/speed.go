package deep

import (
	"context"

	"github.com/openpaily/paily-check/internal/checker/progress"
	"github.com/openpaily/paily-check/internal/checker/worker"
	"github.com/openpaily/paily-check/internal/client"
	"github.com/openpaily/paily-check/internal/config"
	"github.com/openpaily/paily-check/internal/interfaces"
	speedMacro "github.com/openpaily/paily-check/internal/macros/speed"
)

// RunSpeed runs a download speed test on each node concurrently, returning a
// map of nodeID → kbps. Only called when cfg.Enabled == true.
func RunSpeed(ctx context.Context, nodes []client.Node, vendorBase interfaces.Vendor, cfg *config.SpeedConfig, tracker *progress.Tracker) map[string]int {
	return worker.RunNodes(ctx, nodes, cfg.Concurrency, func(ctx context.Context, node client.Node) int {
		defer tracker.Inc()
		v := vendorBase.Build(node.Raw)
		return speedMacro.Test(ctx, v, speedMacro.SpeedConfig{
			URL:       cfg.URL,
			Threads:   cfg.Threads,
			DurationS: cfg.DurationS,
		})
	})
}
