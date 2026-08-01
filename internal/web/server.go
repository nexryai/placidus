// Package web serves placidus HTML, HLS, reservation, and recording endpoints.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nexryai/placidus/internal/mirakurun"
	"github.com/nexryai/placidus/internal/recording"
	"github.com/nexryai/placidus/internal/transcode"
	"github.com/nexryai/placidus/internal/tuner"
)

//go:embed templates/*.gotmpl
var templateFiles embed.FS

type liveService interface {
	Start(uint64) (transcode.LiveView, error)
	PlaylistPath(uint64, string) (string, transcode.LiveView, error)
	Status(uint64) transcode.LiveView
	Capabilities() transcode.Capabilities
}

type reservationScheduler interface {
	Cancel(string) error
}

// Server owns the HTTP presentation layer.
type Server struct {
	backend         mirakurun.API
	store           *recording.Store
	scheduler       reservationScheduler
	live            liveService
	coordinator     *tuner.Coordinator
	recordingDir    string
	recordingMargin time.Duration
	tunerLimit      int
	logger          *slog.Logger
	templates       *template.Template
	mux             *http.ServeMux
}

// NewServer parses embedded templates and registers application routes.
func NewServer(backend mirakurun.API, store *recording.Store, scheduler reservationScheduler, live liveService, coordinator *tuner.Coordinator, recordingDir string, recordingMargin time.Duration, tunerLimit int, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}
	functions := template.FuncMap{
		"clock":      func(value time.Time) string { return value.Local().Format("15:04") },
		"dateTime":   func(value time.Time) string { return value.Local().Format("Jan 2, 15:04") },
		"duration":   formatDuration,
		"stateClass": func(state recording.State) string { return "state-" + string(state) },
	}
	views, err := template.New("views").Funcs(functions).ParseFS(templateFiles, "templates/*.gotmpl")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	server := &Server{
		backend: backend, store: store, scheduler: scheduler, live: live, coordinator: coordinator,
		recordingDir: recordingDir, recordingMargin: recordingMargin, tunerLimit: tunerLimit, logger: logger, templates: views, mux: http.NewServeMux(),
	}
	server.routes()
	return server, nil
}

// Handler returns the complete application HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.securityHeaders(s.logRequests(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.home)
	s.mux.HandleFunc("GET /live/{serviceID}", s.livePage)
	s.mux.HandleFunc("GET /hls/{serviceID}/{file}", s.hlsFile)
	s.mux.HandleFunc("GET /api/live/{serviceID}/status", s.liveStatus)
	s.mux.HandleFunc("GET /reservations", s.reservationsPage)
	s.mux.HandleFunc("POST /reservations", s.createReservation)
	s.mux.HandleFunc("POST /reservations/{id}/cancel", s.cancelReservation)
	s.mux.HandleFunc("GET /recordings/{id}/file", s.recordingFile)
	s.mux.HandleFunc("GET /healthz", s.health)
}

type pageMeta struct {
	Title     string
	Active    string
	Message   string
	Error     string
	Recording tuner.Snapshot
}

type programView struct {
	ID          uint64
	Name        string
	Description string
	StartAt     time.Time
	EndAt       time.Time
}

type serviceView struct {
	ID       uint64
	Name     string
	Type     string
	Number   int
	Current  *programView
	Next     *programView
	Programs []programView
}

type homeData struct {
	Page         pageMeta
	Services     []serviceView
	Capabilities transcode.Capabilities
	BackendError string
}

type liveData struct {
	Page         pageMeta
	Service      serviceView
	Services     []serviceView
	Live         transcode.LiveView
	BackendError string
}

type reservationsData struct {
	Page         pageMeta
	Reservations []recording.Reservation
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	services, err := s.guide(r.Context())
	data := homeData{
		Page:     pageMeta{Title: "Home", Active: "home", Message: r.URL.Query().Get("message"), Recording: s.coordinator.Snapshot()},
		Services: services, Capabilities: s.live.Capabilities(),
	}
	if err != nil {
		data.BackendError = err.Error()
	}
	s.render(w, "home", data)
}

func (s *Server) livePage(w http.ResponseWriter, r *http.Request) {
	serviceID, err := parseUintPath(r.PathValue("serviceID"))
	if err != nil {
		http.Error(w, "Invalid service ID", http.StatusBadRequest)
		return
	}
	services, guideErr := s.guide(r.Context())
	var selected serviceView
	for _, service := range services {
		if service.ID == serviceID {
			selected = service
			break
		}
	}
	if selected.ID == 0 && guideErr == nil {
		http.NotFound(w, r)
		return
	}
	liveView, startErr := s.live.Start(serviceID)
	data := liveData{
		Page:    pageMeta{Title: selected.Name, Active: "live", Recording: s.coordinator.Snapshot()},
		Service: selected, Services: services, Live: liveView,
	}
	if guideErr != nil {
		data.BackendError = guideErr.Error()
	}
	if startErr != nil {
		data.Page.Error = startErr.Error()
	}
	s.render(w, "live", data)
}

func (s *Server) hlsFile(w http.ResponseWriter, r *http.Request) {
	serviceID, err := parseUintPath(r.PathValue("serviceID"))
	if err != nil {
		http.Error(w, "Invalid service ID", http.StatusBadRequest)
		return
	}
	name := r.PathValue("file")
	path, _, err := s.live.PlaylistPath(serviceID, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if filepath.Ext(name) == ".m3u8" {
		deadline := time.Now().Add(8 * time.Second)
		for {
			if _, statErr := os.Stat(path); statErr == nil || time.Now().After(deadline) {
				break
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	} else {
		w.Header().Set("Content-Type", "video/mp2t")
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, path)
}

func (s *Server) liveStatus(w http.ResponseWriter, r *http.Request) {
	serviceID, err := parseUintPath(r.PathValue("serviceID"))
	if err != nil {
		http.Error(w, "Invalid service ID", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(s.live.Status(serviceID))
}

func (s *Server) reservationsPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "reservations", reservationsData{
		Page: pageMeta{
			Title: "Reservations", Active: "reservations", Message: r.URL.Query().Get("message"),
			Error: r.URL.Query().Get("error"), Recording: s.coordinator.Snapshot(),
		},
		Reservations: s.store.List(),
	})
}

func (s *Server) createReservation(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectReservations(w, r, "", "Invalid reservation form.")
		return
	}
	programID, err := strconv.ParseUint(r.FormValue("program_id"), 10, 64)
	if err != nil {
		s.redirectReservations(w, r, "", "Invalid program ID.")
		return
	}
	programs, err := s.backend.Programs(r.Context())
	if err != nil {
		s.redirectReservations(w, r, "", "Could not refresh the Mirakurun program guide.")
		return
	}
	services, err := s.backend.Services(r.Context())
	if err != nil {
		s.redirectReservations(w, r, "", "Could not refresh Mirakurun services.")
		return
	}
	var selected mirakurun.Program
	for _, program := range programs {
		if program.ID == programID {
			selected = program
			break
		}
	}
	if selected.ID == 0 || !selected.EndTime().After(time.Now()) {
		s.redirectReservations(w, r, "", "That program is unavailable or has already ended.")
		return
	}
	var serviceID uint64
	for _, service := range services {
		if service.ServiceID == selected.ServiceID && service.NetworkID == selected.NetworkID {
			serviceID = service.ID
			break
		}
	}
	if serviceID == 0 {
		s.redirectReservations(w, r, "", "The program's Mirakurun service could not be found.")
		return
	}
	item := recording.Reservation{
		ProgramID: selected.ID, ServiceID: serviceID, Title: fallbackTitle(selected.Name),
		StartAt: selected.StartTime(), EndAt: selected.EndTime(),
	}
	conflictCandidate := item
	conflictCandidate.StartAt = conflictCandidate.StartAt.Add(-s.recordingMargin)
	conflictCandidate.EndAt = conflictCandidate.EndAt.Add(s.recordingMargin)
	existing := s.store.List()
	for index := range existing {
		existing[index].StartAt = existing[index].StartAt.Add(-s.recordingMargin)
		existing[index].EndAt = existing[index].EndAt.Add(s.recordingMargin)
	}
	if recording.WouldConflict(existing, conflictCandidate, s.tunerLimit) {
		s.redirectReservations(w, r, "", "This reservation conflicts with another service and the configured tuner limit.")
		return
	}
	if _, err := s.store.Create(item); err != nil {
		s.logger.Error("create reservation", "error", err)
		s.redirectReservations(w, r, "", "The reservation could not be saved.")
		return
	}
	s.redirectReservations(w, r, "Recording reserved.", "")
}

func (s *Server) cancelReservation(w http.ResponseWriter, r *http.Request) {
	if err := s.scheduler.Cancel(r.PathValue("id")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		s.redirectReservations(w, r, "", err.Error())
		return
	}
	s.redirectReservations(w, r, "Reservation cancelled.", "")
}

func (s *Server) recordingFile(w http.ResponseWriter, r *http.Request) {
	item, ok := s.store.Get(r.PathValue("id"))
	if !ok || item.FileName == "" || filepath.Base(item.FileName) != item.FileName {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", item.FileName))
	http.ServeFile(w, r, filepath.Join(s.recordingDir, item.FileName))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	_, backendErr := s.backend.Services(r.Context())
	capabilities := s.live.Capabilities()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "running", "mirakurun": backendErr == nil, "ffmpeg": capabilities.Available,
		"encoder": capabilities.Selected.Name,
	})
}

func (s *Server) guide(ctx context.Context) ([]serviceView, error) {
	services, err := s.backend.Services(ctx)
	if err != nil {
		return nil, fmt.Errorf("Mirakurun services are unavailable: %w", err)
	}
	programs, err := s.backend.Programs(ctx)
	if err != nil {
		return nil, fmt.Errorf("Mirakurun program guide is unavailable: %w", err)
	}
	now := time.Now()
	cutoff := now.Add(24 * time.Hour)
	result := make([]serviceView, 0, len(services))
	for _, service := range services {
		view := serviceView{ID: service.ID, Name: fallbackTitle(service.Name), Number: service.RemoteControlKeyID}
		if service.Channel != nil {
			view.Type = service.Channel.Type
		}
		for _, program := range programs {
			if program.ServiceID != service.ServiceID || program.NetworkID != service.NetworkID ||
				!program.EndTime().After(now) || !program.StartTime().Before(cutoff) {
				continue
			}
			view.Programs = append(view.Programs, programView{
				ID: program.ID, Name: fallbackTitle(program.Name), Description: program.Description,
				StartAt: program.StartTime(), EndAt: program.EndTime(),
			})
		}
		sort.Slice(view.Programs, func(i, j int) bool { return view.Programs[i].StartAt.Before(view.Programs[j].StartAt) })
		for index := range view.Programs {
			program := &view.Programs[index]
			if !program.StartAt.After(now) && program.EndAt.After(now) {
				view.Current = program
			} else if program.StartAt.After(now) && view.Next == nil {
				view.Next = program
			}
		}
		result = append(result, view)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Number == 0 || result[j].Number == 0 {
			return result[i].Name < result[j].Name
		}
		return result[i].Number < result[j].Number
	})
	return result, nil
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		s.logger.Error("render template", "template", name, "error", err)
	}
}

func (s *Server) redirectReservations(w http.ResponseWriter, r *http.Request, message, errorMessage string) {
	values := make([]string, 0, 2)
	if message != "" {
		values = append(values, "message="+url.QueryEscape(message))
	}
	if errorMessage != "" {
		values = append(values, "error="+url.QueryEscape(errorMessage))
	}
	target := "/reservations"
	if len(values) > 0 {
		target += "?" + strings.Join(values, "&")
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Debug("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; media-src 'self' blob:; connect-src 'self'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

func parseUintPath(value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("invalid identifier")
	}
	return parsed, nil
}

func fallbackTitle(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Untitled program"
	}
	return value
}

func formatDuration(value time.Duration) string {
	minutes := int(value.Round(time.Minute).Minutes())
	if minutes < 60 {
		return fmt.Sprintf("%d min", minutes)
	}
	return fmt.Sprintf("%dh %02dm", minutes/60, minutes%60)
}
