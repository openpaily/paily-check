package deep

import (
	"context"

	"github.com/openpaily/paily-check/internal/checker/progress"
	"github.com/openpaily/paily-check/internal/checker/worker"
	"github.com/openpaily/paily-check/internal/client"
	"github.com/openpaily/paily-check/internal/config"
	"github.com/openpaily/paily-check/internal/interfaces"
	geoMacro "github.com/openpaily/paily-check/internal/macros/geo"
)

// RunGeo runs geo detection for each node concurrently, returning a map of
// nodeID → ISO 3166-1 alpha-2 country code (or "" when detection failed).
func RunGeo(ctx context.Context, nodes []client.Node, vendorBase interfaces.Vendor, cfg *config.GeoConfig, tracker *progress.Tracker) map[string]string {
	return worker.RunNodes(ctx, nodes, cfg.Concurrency, func(ctx context.Context, node client.Node) string {
		defer tracker.Inc()
		v := vendorBase.Build(node.Raw)
		return geoMacro.Detect(ctx, v, geoMacro.GeoConfig{
			Mode:            cfg.Mode,
			MmdbPath:        cfg.MmdbPath,
			FallbackLegacy:  cfg.FallbackLegacy,
			FallbackInbound: cfg.FallbackInbound,
			IpScript:        cfg.IpScript,
			GeoScript:       cfg.GeoScript,
			Retry:           cfg.Retry,
			NativeAPI: geoMacro.GeoNativeAPIConfig{
				TimeoutMS: cfg.NativeAPI.TimeoutMS,
				Providers: append([]string(nil), cfg.NativeAPI.Providers...),
			},
		})
	})
}
