package transcode

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	result := Probe(context.Background(), binary, "auto", "/dev/dri/renderD128")
	if !result.Available || result.Selected.Name != "libx264" || result.Warning == "" {
		t.Fatalf("Probe() = %#v", result)
	}
}

func TestLiveArgumentsUseSelectedEncoderAndHLS(t *testing.T) {
	args := LiveArguments("http://mirakurun/api/services/1/stream", "/tmp/live/index.m3u8", hardwareEncoders("/dev/dri/renderD128")[0])
	for _, expected := range []string{"h264_nvenc", "aac", "hls", "/tmp/live/segment-%06d.ts", "/tmp/live/index.m3u8"} {
		if !slices.Contains(args, expected) {
			t.Errorf("arguments do not contain %q: %#v", expected, args)
		}
	}
}

func TestHardwareEncodersUseConfiguredRenderDevice(t *testing.T) {
	device := "/dev/dri/renderD129"
	encoders := hardwareEncoders(device)
	for _, name := range []string{"h264_qsv", "h264_vaapi"} {
		var inputArgs []string
		for _, encoder := range encoders {
			if encoder.Name == name {
				inputArgs = encoder.InputArgs
				break
			}
		}
		if !strings.Contains(strings.Join(inputArgs, " "), device) {
			t.Errorf("%s arguments do not contain %q: %#v", name, device, inputArgs)
		}
	}
}
