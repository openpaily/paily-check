package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Sub-structures
// ---------------------------------------------------------------------------

type CoreConfig struct {
	URL    string `yaml:"url"`
	Secret string `yaml:"secret"`
}

type ScheduleConfig struct {
	Cron string `yaml:"cron"`
}

// InitialConfig controls the initial latency pass over all nodes.
type InitialConfig struct {
	PingURL          string `yaml:"ping_url"`
	PingAvg          int    `yaml:"ping_avg"`
	PingTimeoutMS    int    `yaml:"ping_timeout_ms"`
	Concurrency      int    `yaml:"concurrency"`
	AliveThresholdMS int    `yaml:"alive_threshold_ms"`
}

// DeepPingConfig controls detailed latency checks.
type DeepPingConfig struct {
	Concurrency    int    `yaml:"concurrency"`
	Avg            int    `yaml:"avg"`
	PingURL        string `yaml:"ping_url"`
	TimeoutMS      int    `yaml:"timeout_ms"`
	FilterDeadDeep bool   `yaml:"filter_dead_deep"` // drop nodes whose deep ping latency is -1
}

// GeoConfig controls geo/region detection.
type GeoConfig struct {
	Concurrency     int                `yaml:"concurrency"`
	Mode            string             `yaml:"mode"`      // "script" | "mmdb" | "native_api"
	MmdbPath        string             `yaml:"mmdb_path"` // required when mode=mmdb
	FallbackLegacy  bool               `yaml:"fallback_legacy"`
	FallbackInbound bool               `yaml:"fallback_inbound"`
	IpScript        string             `yaml:"ip_script"`  // optional custom IP-lookup script
	GeoScript       string             `yaml:"geo_script"` // optional custom GeoIP script
	Retry           int                `yaml:"retry"`
	NativeAPI       GeoNativeAPIConfig `yaml:"native_api"`
}

type GeoNativeAPIConfig struct {
	TimeoutMS int      `yaml:"timeout_ms"`
	Providers []string `yaml:"providers"`
}

// ScriptConfig controls third-party script checks.
type ScriptConfig struct {
	NodeConcurrency   int      `yaml:"node_concurrency"`
	ScriptConcurrency int      `yaml:"script_concurrency"`
	Dir               string   `yaml:"dir"`
	TimeoutMS         int      `yaml:"timeout_ms"`
	Engine            string   `yaml:"engine"`
	Regions           []string `yaml:"regions"`
	Scripts           []string `yaml:"scripts"`
}

// SpeedConfig controls the optional download speed test.
type SpeedConfig struct {
	Concurrency int    `yaml:"concurrency"`
	Enabled     bool   `yaml:"enabled"`
	URL         string `yaml:"url"`
	Threads     int    `yaml:"threads"`
	DurationS   int    `yaml:"duration_s"`
}

// ProgressBarConfig enables the live terminal progress indicator.
type ProgressBarConfig struct {
	Enabled bool `yaml:"enabled"`
}

// DeepConfig groups detailed-check configuration.
type DeepConfig struct {
	Ping   DeepPingConfig `yaml:"ping"`
	Geo    GeoConfig      `yaml:"geo"`
	Script ScriptConfig   `yaml:"script"`
	Speed  SpeedConfig    `yaml:"speed"`
}

// Config is the root configuration object.
type Config struct {
	Core        CoreConfig        `yaml:"core"`
	Schedule    ScheduleConfig    `yaml:"schedule"`
	Initial     InitialConfig     `yaml:"initial"`
	Deep        DeepConfig        `yaml:"deep"`
	ProgressBar ProgressBarConfig `yaml:"progress_bar"`
}

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

func defaults() *Config {
	return &Config{
		Core: CoreConfig{
			URL:    "http://localhost:8080",
			Secret: "",
		},
		Schedule: ScheduleConfig{
			Cron: "0 */30 * * * *",
		},
		Initial: InitialConfig{
			PingURL:          "https://www.gstatic.com/generate_204",
			PingAvg:          3,
			PingTimeoutMS:    3000,
			Concurrency:      50,
			AliveThresholdMS: 3000,
		},
		Deep: DeepConfig{
			Ping: DeepPingConfig{
				Concurrency: 20,
				Avg:         7,
				PingURL:     "https://www.gstatic.com/generate_204",
				TimeoutMS:   3000,
			},
			Geo: GeoConfig{
				Concurrency:     20,
				Mode:            "script",
				FallbackLegacy:  true,
				FallbackInbound: true,
				Retry:           2,
				NativeAPI: GeoNativeAPIConfig{
					TimeoutMS: 5000,
					Providers: []string{
						"cloudflare-trace",
						"ipwhois",
						"myip",
						"ipapi-co",
						"ident-me",
						"ip-api",
						"ip-sb",
						"ipinfo",
					},
				},
			},
			Script: ScriptConfig{
				NodeConcurrency:   5,
				ScriptConcurrency: 32,
				Dir:               "./scripts",
				TimeoutMS:         10000,
				Engine:            "goja",
				Scripts:           nil,
			},
			Speed: SpeedConfig{
				Concurrency: 2,
				Enabled:     false,
				Threads:     4,
				DurationS:   8,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

// Load reads YAML from path (if non-empty), then overlays environment
// variables. Environment variables always take precedence over file values.
// If path is empty the defaults are returned with env overrides applied.
func Load(path string) (*Config, error) {
	cfg := defaults()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("config: read file: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config: parse yaml: %w", err)
		}
	}

	applyEnv(cfg)
	return cfg, nil
}

// ---------------------------------------------------------------------------
// Environment variable overrides
// ---------------------------------------------------------------------------

func applyEnv(cfg *Config) {
	// Core
	envStr("PAILY_CORE_URL", &cfg.Core.URL)
	envStr("PAILY_SERVICE_SECRET", &cfg.Core.Secret)

	// Schedule
	envStr("CHECK_CRON", &cfg.Schedule.Cron)

	// Initial
	envStr("INITIAL_PING_URL", &cfg.Initial.PingURL)
	envInt("INITIAL_PING_AVG", &cfg.Initial.PingAvg)
	envInt("INITIAL_PING_TIMEOUT_MS", &cfg.Initial.PingTimeoutMS)
	envInt("INITIAL_CONCURRENCY", &cfg.Initial.Concurrency)
	envInt("INITIAL_ALIVE_THRESHOLD_MS", &cfg.Initial.AliveThresholdMS)

	// Deep ping
	envInt("DEEP_PING_CONCURRENCY", &cfg.Deep.Ping.Concurrency)
	envInt("DEEP_PING_AVG", &cfg.Deep.Ping.Avg)
	envStr("DEEP_PING_URL", &cfg.Deep.Ping.PingURL)
	envInt("DEEP_PING_TIMEOUT_MS", &cfg.Deep.Ping.TimeoutMS)
	envBool("DEEP_PING_FILTER_DEAD", &cfg.Deep.Ping.FilterDeadDeep)

	// Deep geo
	envInt("DEEP_GEO_CONCURRENCY", &cfg.Deep.Geo.Concurrency)
	envStr("GEO_MODE", &cfg.Deep.Geo.Mode)
	envStr("GEO_MMDB_PATH", &cfg.Deep.Geo.MmdbPath)
	envBool("GEO_FALLBACK_LEGACY", &cfg.Deep.Geo.FallbackLegacy)
	envBool("GEO_FALLBACK_INBOUND", &cfg.Deep.Geo.FallbackInbound)
	envStr("GEO_IP_SCRIPT", &cfg.Deep.Geo.IpScript)
	envStr("GEO_GEO_SCRIPT", &cfg.Deep.Geo.GeoScript)
	envInt("GEO_RETRY", &cfg.Deep.Geo.Retry)
	envInt("GEO_NATIVE_API_TIMEOUT_MS", &cfg.Deep.Geo.NativeAPI.TimeoutMS)
	envCSV("GEO_NATIVE_API_PROVIDERS", &cfg.Deep.Geo.NativeAPI.Providers)

	// Deep script
	envInt("DEEP_SCRIPT_NODE_CONCURRENCY", &cfg.Deep.Script.NodeConcurrency)
	envInt("DEEP_SCRIPT_CONCURRENCY", &cfg.Deep.Script.ScriptConcurrency)
	envStr("SCRIPT_DIR", &cfg.Deep.Script.Dir)
	envInt("SCRIPT_TIMEOUT_MS", &cfg.Deep.Script.TimeoutMS)
	envStr("SCRIPT_ENGINE", &cfg.Deep.Script.Engine)
	envCSV("SCRIPT_REGIONS", &cfg.Deep.Script.Regions)
	envCSV("SCRIPT_LIST", &cfg.Deep.Script.Scripts)

	// Deep speed
	envInt("DEEP_SPEED_CONCURRENCY", &cfg.Deep.Speed.Concurrency)
	envBool("SPEED_ENABLED", &cfg.Deep.Speed.Enabled)
	envStr("SPEED_URL", &cfg.Deep.Speed.URL)
	envInt("SPEED_THREADS", &cfg.Deep.Speed.Threads)
	envInt("SPEED_DURATION_S", &cfg.Deep.Speed.DurationS)

	// Progress bar
	envBool("PROGRESS_BAR", &cfg.ProgressBar.Enabled)
}

// helpers

func envStr(key string, dst *string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

func envInt(key string, dst *int) {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			*dst = n
		}
	}
}

func envBool(key string, dst *bool) {
	if v := os.Getenv(key); v != "" {
		switch v {
		case "true", "1", "yes":
			*dst = true
		case "false", "0", "no":
			*dst = false
		}
	}
}

func envCSV(key string, dst *[]string) {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			out = append(out, part)
		}
		*dst = out
	}
}
