package config

import (
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PLACIDUS_DATA_DIR", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddress != ":40773" {
		t.Fatalf("ListenAddress = %q", cfg.ListenAddress)
	}
	if cfg.TunerLimit != 1 {
		t.Fatalf("TunerLimit = %d", cfg.TunerLimit)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q", cfg.LogLevel)
	}
	if cfg.GPUDevice != "/dev/dri/renderD128" {
		t.Fatalf("GPUDevice = %q", cfg.GPUDevice)
	}
	if cfg.RecordingDir != filepath.Join(cfg.DataDirectory, "recordings") {
		t.Fatalf("RecordingDir = %q", cfg.RecordingDir)
	}
}

func TestLoadRejectsEmptyGPUDevice(t *testing.T) {
	t.Setenv("PLACIDUS_GPU_DEVICE", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded with an empty GPU device")
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	t.Setenv("PLACIDUS_LOG_LEVEL", "verbose")
	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded with an invalid log level")
	}
}

func TestLoadRejectsInvalidTunerLimit(t *testing.T) {
	t.Setenv("PLACIDUS_TUNER_LIMIT", "0")
	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded with an invalid tuner limit")
	}
}
