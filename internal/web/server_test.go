package web

import (
	"context"
	"encoding/json"
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
	return newTestHandlerWithLogger(t, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func newTestHandlerWithLogger(t *testing.T, logger *slog.Logger) (http.Handler, *recording.Store, mirakurun.Program) {
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
	server, err := NewServer(backend, store, fakeScheduler{store}, fakeLive{}, coordinator, filepath.Join(directory, "recordings"), 0, 1, logger)
	if err != nil {
		t.Fatal(err)
	}
	return server.Handler(), store, program
}

func TestRequestLoggingIncludesOutcomeAndRequestID(t *testing.T) {
	var logs strings.Builder
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler, _, _ := newTestHandlerWithLogger(t, logger)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("User-Agent", "placidus-test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("response is missing X-Request-ID")
	}
	for _, expected := range []string{"http request completed", `"status":200`, `"bytes":`, `"request_id":1`, `"user_agent":"placidus-test"`} {
		if !strings.Contains(logs.String(), expected) {
			t.Errorf("logs do not contain %q: %s", expected, logs.String())
		}
	}
}

func TestRequestLogLevelKeepsHighFrequencyRoutesAtDebug(t *testing.T) {
	tests := []struct {
		method string
		path   string
		status int
		want   slog.Level
	}{
		{http.MethodGet, "/", http.StatusOK, slog.LevelInfo},
		{http.MethodPost, "/reservations", http.StatusSeeOther, slog.LevelInfo},
		{http.MethodGet, "/hls/1/segment-000001.ts", http.StatusOK, slog.LevelDebug},
		{http.MethodGet, "/healthz", http.StatusOK, slog.LevelDebug},
		{http.MethodGet, "/missing", http.StatusNotFound, slog.LevelWarn},
		{http.MethodGet, "/broken", http.StatusInternalServerError, slog.LevelError},
	}
	for _, test := range tests {
		request := httptest.NewRequest(test.method, test.path, nil)
		if got := requestLogLevel(request, test.status); got != test.want {
			t.Errorf("requestLogLevel(%s, %s, %d) = %s, want %s", test.method, test.path, test.status, got, test.want)
		}
	}
}

func TestUnknownRouteReturnsNotFoundAndLogsWarning(t *testing.T) {
	var logs strings.Builder
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler, _, _ := newTestHandlerWithLogger(t, logger)
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	for _, expected := range []string{`"level":"WARN"`, `"path":"/missing"`, `"status":404`} {
		if !strings.Contains(logs.String(), expected) {
			t.Errorf("logs do not contain %q: %s", expected, logs.String())
		}
	}
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

func TestPageAPIsReturnStateWithoutCaching(t *testing.T) {
	handler, _, _ := newTestHandler(t)
	for _, path := range []string{"/api/home", "/api/live/3273601024", "/api/reservations"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			var data map[string]json.RawMessage
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &data) != nil {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
			if data["Page"] == nil || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("missing page state or cache protection: %s", response.Body.String())
			}
		})
	}
}

func TestReservationAPICreateCancelAndErrors(t *testing.T) {
	handler, store, _ := newTestHandler(t)
	post := func(path, form string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	if response := post("/api/reservations", "program_id=invalid"); response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "Invalid program ID") {
		t.Fatalf("invalid program response = %d %s", response.Code, response.Body.String())
	}
	response := post("/api/reservations", "program_id=99&title=Untrusted")
	if response.Code != http.StatusOK || response.Header().Get("Location") != "" {
		t.Fatalf("create response = %d %s", response.Code, response.Body.String())
	}
	items := store.List()
	if len(items) != 1 || items[0].Title != "Morning News" {
		t.Fatalf("reservations = %#v", items)
	}
	response = post("/api/reservations/"+items[0].ID+"/cancel", "")
	item, _ := store.Get(items[0].ID)
	if response.Code != http.StatusOK || item.State != recording.StateCancelled {
		t.Fatalf("cancel response = %d, item = %#v", response.Code, item)
	}
}
