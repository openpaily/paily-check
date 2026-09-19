// Command checker is the paily-check service entry point.
// It loads a YAML config, schedules a recurring check via cron, and on each
// tick fetches nodes from paily-core, runs the full pipeline, and posts
// results back.
package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/openpaily/paily-check/internal/checker"
	"github.com/openpaily/paily-check/internal/client"
	"github.com/openpaily/paily-check/internal/config"
	geoMacro "github.com/openpaily/paily-check/internal/macros/geo"
	"github.com/openpaily/paily-check/internal/vendor"
	"github.com/sirupsen/logrus"
)

const (
	getNodesRetryCount    = 10
	getNodesRetryDelay    = time.Second
	postResultsRetryCount = 10
	postResultsRetryDelay = time.Second
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("checker: load config: %v", err)
	}

	logrus.SetOutput(io.Discard)

	cl, err := client.New(cfg.Core.URL, cfg.Core.Secret)
	if err != nil {
		log.Fatalf("checker: create client: %v", err)
	}
	v := vendor.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var background sync.WaitGroup
	if strings.EqualFold(cfg.Deep.Geo.Mode, "mmdb") && cfg.Deep.Geo.MmdbPath != "" {
		updater := geoMacro.NewMMDBUpdater(cfg.Deep.Geo.MmdbPath, geoMacro.DefaultMMDBUpdateURL, geoMacro.DefaultMMDBUpdateInterval)
		background.Add(1)
		go func() {
			defer background.Done()
			updater.Start(ctx)
		}()
		log.Printf("checker: geo mmdb updater enabled for %s", cfg.Deep.Geo.MmdbPath)
	}

	// running is 1 while a cycle is in progress; CAS ensures at most one
	// cycle runs at a time — concurrent cron triggers are skipped.
	var running atomic.Int32

	// runOnce encapsulates a single check cycle.
	// Returns false (skipped) when another cycle is already running.
	runOnce := func() bool {
		if !running.CompareAndSwap(0, 1) {
			log.Printf("checker: cycle already running, skipping trigger")
			return false
		}
		defer running.Store(0)

		var nodes []client.Node
		var err error
		for attempt := 0; attempt <= getNodesRetryCount; attempt++ {
			nodes, err = cl.GetNodes()
			if err == nil {
				break
			}
			if attempt == getNodesRetryCount {
				log.Printf("checker: GetNodes failed after %d retries: %v", getNodesRetryCount, err)
				return true
			}
			log.Printf("checker: GetNodes failed (attempt %d/%d): %v", attempt+1, getNodesRetryCount+1, err)
			time.Sleep(getNodesRetryDelay)
		}
		if len(nodes) == 0 {
			log.Printf("checker: no nodes returned, skipping cycle")
			return true
		}
		log.Printf("checker: starting pipeline for %d nodes", len(nodes))

		results := checker.Run(context.Background(), nodes, v, cfg)

		for attempt := 0; attempt <= postResultsRetryCount; attempt++ {
			err = cl.PostResults(results)
			if err == nil {
				break
			}
			if attempt == postResultsRetryCount {
				log.Printf("checker: PostResults failed after %d retries: %v", postResultsRetryCount, err)
				return true
			}
			log.Printf("checker: PostResults failed (attempt %d/%d): %v", attempt+1, postResultsRetryCount+1, err)
			time.Sleep(postResultsRetryDelay)
		}
		log.Printf("checker: cycle complete, posted %d results", len(results))
		return true
	}

	// Validate the cron expression before starting anything.
	c := cron.New(cron.WithSeconds())
	_, err = c.AddFunc(cfg.Schedule.Cron, func() { runOnce() })
	if err != nil {
		log.Fatalf("checker: invalid cron expression %q: %v", cfg.Schedule.Cron, err)
	}
	log.Printf("checker: scheduled with cron %q", cfg.Schedule.Cron)

	// Run immediately on startup, then hand off to the cron scheduler.
	log.Printf("checker: running initial cycle")
	runOnce()

	c.Start()
	defer c.Stop()

	// Block until SIGINT / SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("checker: shutting down")
	cancel()
	background.Wait()
}
