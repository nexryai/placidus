// Package transcode discovers FFmpeg encoders and supervises live HLS sessions.
package transcode

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Encoder describes the FFmpeg arguments required by one H.264 encoder.
type Encoder struct {
	Name       string
	Hardware   bool
	InputArgs  []string
	FilterArgs []string
	VideoArgs  []string
}

// Capabilities is the result of real FFmpeg encoder initialization probes.
type Capabilities struct {
	Available bool
	Version   string
	Selected  Encoder
	Software  Encoder
	Warning   string
	Failures  map[string]string
}

var softwareEncoder = Encoder{
	Name:      "libx264",
	VideoArgs: []string{"-preset", "veryfast", "-tune", "zerolatency", "-crf", "23"},
}

var hardwareEncoders = []Encoder{
	{Name: "h264_nvenc", Hardware: true, VideoArgs: []string{"-preset", "p4", "-tune", "ll", "-cq", "24"}},
	{Name: "h264_qsv", Hardware: true, VideoArgs: []string{"-preset", "veryfast", "-global_quality", "25"}},
	{Name: "h264_vaapi", Hardware: true, InputArgs: []string{"-vaapi_device", "/dev/dri/renderD128"}, FilterArgs: []string{"-vf", "format=nv12,hwupload"}, VideoArgs: []string{"-qp", "24"}},
	{Name: "h264_videotoolbox", Hardware: true, VideoArgs: []string{"-realtime", "1", "-q:v", "65"}},
	{Name: "h264_v4l2m2m", Hardware: true, VideoArgs: []string{"-b:v", "5M"}},
}

// Probe verifies FFmpeg and attempts to initialize each candidate encoder.
func Probe(ctx context.Context, binary, preferred string) Capabilities {
	result := Capabilities{Software: softwareEncoder, Failures: make(map[string]string)}
	versionCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(versionCtx, binary, "-version").CombinedOutput()
	if err != nil {
		result.Warning = fmt.Sprintf("FFmpeg is unavailable: %v", err)
		return result
	}
	result.Version = firstLine(string(output))

	candidates := orderedHardwareEncoders(preferred)
	for _, candidate := range candidates {
		if err := probeEncoder(ctx, binary, candidate); err != nil {
			result.Failures[candidate.Name] = err.Error()
			continue
		}
		result.Available = true
		result.Selected = candidate
		break
	}

	softwareErr := probeEncoder(ctx, binary, softwareEncoder)
	if softwareErr != nil {
		result.Failures[softwareEncoder.Name] = softwareErr.Error()
		if !result.Available {
			result.Warning = "FFmpeg is installed, but no supported H.264 encoder could be initialized."
		}
		return result
	}
	if !result.Available {
		result.Available = true
		result.Selected = softwareEncoder
		result.Warning = "No hardware encoder could be initialized. FFmpeg will use software encoding, which may cause high CPU usage."
	}
	return result
}

func orderedHardwareEncoders(preferred string) []Encoder {
	ordered := make([]Encoder, 0, len(hardwareEncoders))
	if preferred != "" && preferred != "auto" {
		for _, candidate := range hardwareEncoders {
			if candidate.Name == preferred {
				ordered = append(ordered, candidate)
				break
			}
		}
	}
	for _, candidate := range hardwareEncoders {
		if len(ordered) == 0 || ordered[0].Name != candidate.Name {
			ordered = append(ordered, candidate)
		}
	}
	return ordered
}

func probeEncoder(parent context.Context, binary string, encoder Encoder) error {
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	args = append(args, encoder.InputArgs...)
	args = append(args, "-f", "lavfi", "-i", "color=c=black:s=128x72:d=0.1")
	args = append(args, encoder.FilterArgs...)
	args = append(args, "-frames:v", "1", "-an", "-c:v", encoder.Name)
	args = append(args, encoder.VideoArgs...)
	args = append(args, "-f", "null", "-")
	output, err := exec.CommandContext(ctx, binary, args...).CombinedOutput()
	if err != nil {
		reason := strings.TrimSpace(string(output))
		if len(reason) > 300 {
			reason = reason[len(reason)-300:]
		}
		if reason == "" {
			reason = err.Error()
		}
		return fmt.Errorf("initialization failed: %s", reason)
	}
	return nil
}

func firstLine(value string) string {
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return strings.TrimSpace(value)
}
