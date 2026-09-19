// Package initial implements the fast initial latency pass over all nodes.
// Only nodes within the alive threshold continue to detailed checks.
package initial

import (
	"context"
	"sync"

	"golang.org/x/sync/semaphore"

	"github.com/openpaily/paily-check/internal/checker/progress"
	"github.com/openpaily/paily-check/internal/client"
	"github.com/openpaily/paily-check/internal/config"
	"github.com/openpaily/paily-check/internal/interfaces"
	pingMacro "github.com/openpaily/paily-check/internal/macros/ping"
)

// RunAll performs an initial ping on every node concurrently (up to
// cfg.Concurrency at a time) using vendorBase.Build(node.Raw) to create
// per-node proxy instances.
//
// Returns one client.CheckResult per node. Deep is always nil (filled in
// detailed checks). The caller uses InitialResult.LatencyMS to decide whether a
// node advances.
func RunAll(ctx context.Context, nodes []client.Node, vendorBase interfaces.Vendor, cfg *config.InitialConfig, tracker *progress.Tracker) []client.CheckResult {
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 50
	}

	sem := semaphore.NewWeighted(int64(concurrency))
	results := make([]client.CheckResult, len(nodes))
	wg := sync.WaitGroup{}

	for i, node := range nodes {
		i, node := i, node
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer tracker.Inc()

			if err := sem.Acquire(ctx, 1); err != nil {
				results[i] = client.CheckResult{
					NodeID:  node.ID,
					Initial: client.InitialResult{LatencyMS: -1},
				}
				return
			}
			defer sem.Release(1)

			v := vendorBase.Build(node.Raw)
			pr := pingMacro.Ping(ctx, v, pingMacro.PingConfig{
				URL:       cfg.PingURL,
				Avg:       cfg.PingAvg,
				TimeoutMS: cfg.PingTimeoutMS,
			})

			results[i] = client.CheckResult{
				NodeID:  node.ID,
				Initial: client.InitialResult{LatencyMS: pr.LatencyMS},
			}
		}()
	}

	wg.Wait()
	return results
}
