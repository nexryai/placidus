package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexryai/placidus/internal/mirakurun"
	"github.com/nexryai/placidus/internal/recording"
	"github.com/nexryai/placidus/internal/transcode"
	"github.com/nexryai/placidus/internal/tuner"
)

type fakeBackend struct {
	services []mirakurun.Service
	programs []mirakurun.Program
}

func (f fakeBackend) Services(context.Context) ([]mirakurun.Service, error) { return f.services, nil }
func (f fakeBackend) Programs(context.Context) ([]mirakurun.Program, error) { return f.programs, nil }
func (f fakeBackend) Tuners(context.Context) ([]mirakurun.Tuner, error)     { return nil, nil }
func (f fakeBackend) OpenServiceStream(context.Context, uint64, int) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("stream")), nil
}

type fakeLive struct{}

func (fakeLive) Start(id uint64) (transcode.LiveView, error) {
	return transcode.LiveView{ServiceID: id, Encoder: "h264_nvenc", Hardware: true, Ready: true}, nil
}
func (fakeLive) PlaylistPath(id uint64, name string) (string, transcode.LiveView, error) {
	return filepath.Join("/tmp", name), transcode.LiveView{ServiceID: id}, nil
}
func (fakeLive) Status(id uint64) transcode.LiveView { return transcode.LiveView{ServiceID: id} }
func (fakeLive) Capabilities() transcode.Capabilities {
	return transcode.Capabilities{Available: true, Selected: transcode.Encoder{Name: "h264_nvenc", Hardware: true}}
}

type fakeScheduler struct{ store *recording.Store }

func (f fakeScheduler) Cancel(id string) error {
	_, err := f.store.Update(id, func(item *recording.Reservation) error {
		item.State = recording.StateCancelled
		return nil
	})
	return err
}

func newTestHandler(t *testing.T) (http.Handler, *recording.Store, mirakurun.Program) {
	t.Helper()
	now := time.Now()
	program := mirakurun.Program{
		ID: 99, ServiceID: 1024, NetworkID: 32736, Name: "Morning News",
		StartAt: now.Add(-time.Minute).UnixMilli(), Duration: int64((time.Hour + time.Minute) / time.Millisecond),
	}
	backend := fakeBackend{
		services: []mirakurun.Service{{ID: 3273601024, ServiceID: 1024, NetworkID: 32736, Name: "Test TV", RemoteControlKeyID: 1, Channel: &mirakurun.Channel{Type: "GR"}}},
		programs: []mirakurun.Program{program},
	}
	directory := t.TempDir()
	store, err := recording.NewStore(filepath.Join(directory, "reservations.json"))
	if err != nil {
		t.Fatal(err)
	}
	coordinator := tuner.NewCoordinator(1)
	server, err := NewServer(backend, store, fakeScheduler{store}, fakeLive{}, coordinator, filepath.Join(directory, "recordings"), 0, 1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return server.Handler(), store, program
}

func TestHomeRendersMirakurunGuide(t *testing.T) {
	handler, _, _ := newTestHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	for _, expected := range []string{"Test TV", "Morning News", "h264_nvenc", "Program guide"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("home page does not contain %q", expected)
		}
	}
	if response.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("security headers are missing")
	}
}

func TestCreateReservationUsesAuthoritativeProgramData(t *testing.T) {
	handler, store, program := newTestHandler(t)
	form := url.Values{"program_id": {"99"}, "title": {"Untrusted title"}}
	request := httptest.NewRequest(http.MethodPost, "/reservations", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	items := store.List()
	if len(items) != 1 || items[0].Title != program.Name || items[0].ServiceID != 3273601024 {
		t.Fatalf("stored reservations = %#v", items)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/reservations", nil)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "Morning News") || !strings.Contains(listResponse.Body.String(), "scheduled") {
		t.Fatalf("reservation page did not render the saved item: %s", listResponse.Body.String())
	}
}

func TestLivePageStartsPinnedHLSPlayer(t *testing.T) {
	handler, _, _ := newTestHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/live/3273601024", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	for _, expected := range []string{"hls.js@1.6.16", "/hls/3273601024/index.m3u8", "h264_nvenc"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("live page does not contain %q", expected)
		}
	}
}

func TestLiveStatusEndpointSupportsVisibleFallbackUpdates(t *testing.T) {
	handler, _, _ := newTestHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/api/live/3273601024/status", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"ServiceID":3273601024`) {
		t.Fatalf("live status response = %d %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("live status response can be cached")
	}
}
