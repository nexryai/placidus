package transcode

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexryai/placidus/internal/tuner"
)

func TestLiveManagerFallsBackAfterHardwareProcessFailure(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "fake-ffmpeg")
	script := `#!/bin/sh
case " $* " in
  *" h264_nvenc "*) echo "NVENC device disappeared" >&2; exit 1 ;;
esac
for last do :; done
mkdir -p "$(dirname "$last")"
printf '#EXTM3U\n' > "$last"
sleep 30
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	capabilities := Capabilities{
		Available: true,
		Selected:  Encoder{Name: "h264_nvenc", Hardware: true},
		Software:  softwareEncoder,
	}
	manager, err := NewLiveManager(binary, "http://mirakurun:40772", t.TempDir(), capabilities, tuner.NewCoordinator(1), slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.Start(100); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		view := manager.Status(100)
		if view.Encoder == "libx264" && strings.Contains(view.Warning, "Falling back") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("fallback did not become visible: %#v", manager.Status(100))
}
