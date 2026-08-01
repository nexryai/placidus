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
	if cfg.RecordingDir != filepath.Join(cfg.DataDirectory, "recordings") {
		t.Fatalf("RecordingDir = %q", cfg.RecordingDir)
	}
}

func TestLoadRejectsInvalidTunerLimit(t *testing.T) {
	t.Setenv("PLACIDUS_TUNER_LIMIT", "0")
	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded with an invalid tuner limit")
	}
}
