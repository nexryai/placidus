// Package config loads placidus runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config contains the complete runtime configuration for one placidus process.
type Config struct {
	ListenAddress  string
	MirakurunURL   string
	DataDirectory  string
	RecordingDir   string
	FFmpegPath     string
	Encoder        string
	GPUDevice      string
	LogLevel       string
	TunerLimit     int
	ScheduleMargin time.Duration
}

// Load reads configuration from the environment and applies local-first defaults.
func Load() (Config, error) {
	dataDir := envOr("PLACIDUS_DATA_DIR", "./data")
	cfg := Config{
		ListenAddress:  envOr("PLACIDUS_LISTEN", ":40773"),
		MirakurunURL:   strings.TrimRight(envOr("PLACIDUS_MIRAKURUN_URL", "http://127.0.0.1:40772"), "/"),
		DataDirectory:  dataDir,
		RecordingDir:   envOr("PLACIDUS_RECORDING_DIR", filepath.Join(dataDir, "recordings")),
		FFmpegPath:     envOr("PLACIDUS_FFMPEG", "ffmpeg"),
		Encoder:        envOr("PLACIDUS_ENCODER", "auto"),
		GPUDevice:      envOr("PLACIDUS_GPU_DEVICE", "/dev/dri/renderD128"),
		LogLevel:       strings.ToLower(envOr("PLACIDUS_LOG_LEVEL", "info")),
		TunerLimit:     1,
		ScheduleMargin: 0,
	}

	if raw := os.Getenv("PLACIDUS_TUNER_LIMIT"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return Config{}, fmt.Errorf("PLACIDUS_TUNER_LIMIT must be a positive integer")
		}
		cfg.TunerLimit = value
	}
	if raw := os.Getenv("PLACIDUS_RECORDING_MARGIN"); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value < 0 {
			return Config{}, fmt.Errorf("PLACIDUS_RECORDING_MARGIN must be a non-negative duration")
		}
		cfg.ScheduleMargin = value
	}
	if cfg.MirakurunURL == "" {
		return Config{}, fmt.Errorf("PLACIDUS_MIRAKURUN_URL cannot be empty")
	}
	if cfg.GPUDevice == "" {
		return Config{}, fmt.Errorf("PLACIDUS_GPU_DEVICE cannot be empty")
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("PLACIDUS_LOG_LEVEL must be debug, info, warn, or error")
	}
	return cfg, nil
}

func envOr(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}
