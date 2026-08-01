package transcode

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestProbeFallsBackToSoftwareWithWarning(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "fake-ffmpeg")
	script := `#!/bin/sh
if [ "$1" = "-version" ]; then
  echo "ffmpeg version test"
  exit 0
fi
case " $* " in
  *" -c:v libx264 "*) exit 0 ;;
  *) echo "hardware unavailable" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	result := Probe(context.Background(), binary, "auto")
	if !result.Available || result.Selected.Name != "libx264" || result.Warning == "" {
		t.Fatalf("Probe() = %#v", result)
	}
}

func TestLiveArgumentsUseSelectedEncoderAndHLS(t *testing.T) {
	args := LiveArguments("http://mirakurun/api/services/1/stream", "/tmp/live/index.m3u8", hardwareEncoders[0])
	for _, expected := range []string{"h264_nvenc", "aac", "hls", "/tmp/live/segment-%06d.ts", "/tmp/live/index.m3u8"} {
		if !slices.Contains(args, expected) {
			t.Errorf("arguments do not contain %q: %#v", expected, args)
		}
	}
}
