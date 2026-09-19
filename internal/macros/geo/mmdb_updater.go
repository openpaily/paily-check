package geo

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultMMDBUpdateURL      = "https://cdn.jsdelivr.net/gh/Loyalsoldier/geoip@release/Country-without-asn.mmdb"
	DefaultMMDBUpdateInterval = 12 * time.Hour
)

// MMDBUpdater refreshes a local MMDB file on a fixed interval.
type MMDBUpdater struct {
	path     string
	url      string
	interval time.Duration
	client   *http.Client
}

// NewMMDBUpdater creates an updater for path. Empty url/interval values use
// the service defaults.
func NewMMDBUpdater(path, url string, interval time.Duration) *MMDBUpdater {
	if url == "" {
		url = DefaultMMDBUpdateURL
	}
	if interval <= 0 {
		interval = DefaultMMDBUpdateInterval
	}
	return &MMDBUpdater{
		path:     path,
		url:      url,
		interval: interval,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Start runs one refresh immediately and then repeats until ctx is canceled.
// Download errors are logged and do not stop the updater.
func (u *MMDBUpdater) Start(ctx context.Context) {
	u.refresh(ctx)

	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.refresh(ctx)
		}
	}
}

func (u *MMDBUpdater) refresh(ctx context.Context) {
	if err := u.Update(ctx); err != nil {
		log.Printf("geo mmdb: update failed: %v", err)
		return
	}
	log.Printf("geo mmdb: updated %s", u.path)
}

// Update downloads the MMDB and atomically replaces the configured path.
func (u *MMDBUpdater) Update(ctx context.Context) error {
	if u.path == "" {
		return fmt.Errorf("empty mmdb path")
	}

	dir := filepath.Dir(u.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create mmdb directory: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("download: unexpected status %s", resp.Status)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(u.path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := replaceCachedMMDBFile(tmpPath, u.path); err != nil {
		return fmt.Errorf("replace mmdb file: %w", err)
	}
	cleanup = false
	return nil
}
