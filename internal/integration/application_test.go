// Package integration exercises the complete placidus request and media lifecycle.
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexryai/placidus/internal/mirakurun"
	"github.com/nexryai/placidus/internal/recording"
	"github.com/nexryai/placidus/internal/transcode"
	"github.com/nexryai/placidus/internal/tuner"
	"github.com/nexryai/placidus/internal/web"
)

func TestProgramToRecordingAndLiveViewingLifecycle(t *testing.T) {
	now := time.Now()
	services := []mirakurun.Service{
		{ID: 100, ServiceID: 10, NetworkID: 1, Name: "One TV", RemoteControlKeyID: 1, Channel: &mirakurun.Channel{Type: "GR"}},
		{ID: 200, ServiceID: 20, NetworkID: 1, Name: "Two TV", RemoteControlKeyID: 2, Channel: &mirakurun.Channel{Type: "GR"}},
	}
	programs := []mirakurun.Program{{
		ID: 900, ServiceID: 10, NetworkID: 1, Name: "Integration News",
		StartAt: now.Add(-time.Second).UnixMilli(), Duration: int64(2200 * time.Millisecond / time.Millisecond),
	}}
	mirakurunServer := newFakeMirakurun(t, services, programs)
	defer mirakurunServer.Close()

	ffmpeg := fakeFFmpeg(t)
	capabilities := transcode.Probe(context.Background(), ffmpeg, "h264_nvenc")
	if !capabilities.Available || !capabilities.Selected.Hardware {
		t.Fatalf("fake hardware encoder was not selected: %#v", capabilities)
	}
	backend, err := mirakurun.NewClient(mirakurunServer.URL, mirakurunServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	store, err := recording.NewStore(filepath.Join(directory, "reservations.json"))
	if err != nil {
		t.Fatal(err)
	}
	coordinator := tuner.NewCoordinator(1)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	live, err := transcode.NewLiveManager(ffmpeg, mirakurunServer.URL, filepath.Join(directory, "hls"), capabilities, coordinator, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	scheduler, err := recording.NewScheduler(store, backend, coordinator, filepath.Join(directory, "recordings"), 0, logger)
	if err != nil {
		t.Fatal(err)
	}
	server, err := web.NewServer(backend, store, scheduler, live, coordinator, filepath.Join(directory, "recordings"), 0, 1, logger)
	if err != nil {
		t.Fatal(err)
	}
	application := httptest.NewServer(server.Handler())
	defer application.Close()

	home := getBody(t, application.URL+"/")
	if !strings.Contains(home, "Integration News") || !strings.Contains(home, "One TV") {
		t.Fatalf("program guide is missing backend data")
	}
	form := url.Values{"program_id": {"900"}}
	response, err := application.Client().PostForm(application.URL+"/reservations", form)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if len(store.List()) != 1 {
		t.Fatalf("reservation was not persisted: %#v", store.List())
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { scheduler.Run(ctx); close(done) }()
	waitForState(t, store, recording.StateRecording, 2*time.Second)

	blockedPage := getBody(t, application.URL+"/live/200")
	if !strings.Contains(blockedPage, "live viewing is unavailable while recordings occupy all tuners") ||
		!strings.Contains(blockedPage, "Tuners are occupied by recording") {
		t.Fatal("unrelated live service was not blocked with a visible recording warning")
	}
	getBody(t, application.URL+"/live/100")
	playlist := getBody(t, application.URL+"/hls/100/index.m3u8")
	if !strings.Contains(playlist, "#EXTM3U") {
		t.Fatalf("live HLS playlist was not served: %q", playlist)
	}

	completed := waitForState(t, store, recording.StateCompleted, 4*time.Second)
	info, err := os.Stat(filepath.Join(directory, "recordings", completed.FileName))
	if err != nil || info.Size() == 0 {
		t.Fatalf("completed recording file is missing or empty: %v", err)
	}
	cancel()
	<-done
}

func newFakeMirakurun(t *testing.T, services []mirakurun.Service, programs []mirakurun.Program) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/services":
			_ = json.NewEncoder(w).Encode(services)
		case r.URL.Path == "/api/programs":
			_ = json.NewEncoder(w).Encode(programs)
		case r.URL.Path == "/api/tuners":
			_ = json.NewEncoder(w).Encode([]mirakurun.Tuner{{Index: 0, Name: "Fake tuner", IsAvailable: true, IsFree: true}})
		case strings.HasSuffix(r.URL.Path, "/stream"):
			w.Header().Set("Content-Type", "video/MP2T")
			flusher, _ := w.(http.Flusher)
			for {
				if _, err := w.Write([]byte("fake-transport-packet")); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
				select {
				case <-r.Context().Done():
					return
				case <-time.After(10 * time.Millisecond):
				}
			}
		default:
			http.NotFound(w, r)
		}
	}))
}

func fakeFFmpeg(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	script := `#!/bin/sh
if [ "$1" = "-version" ]; then
  echo "ffmpeg version integration-test"
  exit 0
fi
case " $* " in
  *"index.m3u8"*)
    for last do :; done
    directory=$(dirname "$last")
    printf 'fake-segment' > "$directory/segment-000000.ts"
    printf '#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\nsegment-000000.ts\n' > "$last"
    exit 0
    ;;
  *) exit 0 ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func getBody(t *testing.T, target string) string {
	t.Helper()
	response, err := http.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned %s: %s", target, response.Status, data)
	}
	return string(data)
}

func waitForState(t *testing.T, store *recording.Store, expected recording.State, timeout time.Duration) recording.Reservation {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		items := store.List()
		if len(items) == 1 && items[0].State == expected {
			return items[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	returnState := store.List()
	t.Fatalf("reservation did not reach %s: %s", expected, fmt.Sprint(returnState))
	return recording.Reservation{}
}
