package transcode

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/nexryai/placidus/internal/tuner"
)

// LiveView is the user-visible state of one shared live transcoding session.
type LiveView struct {
	ServiceID uint64
	Encoder   string
	Hardware  bool
	Warning   string
	Error     string
	Ready     bool
}

type liveSession struct {
	serviceID uint64
	directory string
	cancel    context.CancelFunc
	encoder   Encoder
	warning   string
	err       string
	lastTouch time.Time
	startedAt time.Time
	ready     bool
}

// LiveManager owns shared FFmpeg processes and their temporary HLS files.
type LiveManager struct {
	mu           sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	binary       string
	mirakurunURL string
	root         string
	capabilities Capabilities
	coordinator  *tuner.Coordinator
	logger       *slog.Logger
	sessions     map[uint64]*liveSession
}

// NewLiveManager creates a live manager and starts its idle-session janitor.
func NewLiveManager(binary, mirakurunURL, root string, capabilities Capabilities, coordinator *tuner.Coordinator, logger *slog.Logger) (*LiveManager, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create HLS directory: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	manager := &LiveManager{
		ctx: ctx, cancel: cancel, binary: binary, mirakurunURL: mirakurunURL, root: root,
		capabilities: capabilities, coordinator: coordinator, logger: logger, sessions: make(map[uint64]*liveSession),
	}
	go manager.janitor()
	return manager, nil
}

// Start returns an existing shared session or starts FFmpeg for the service.
func (m *LiveManager) Start(serviceID uint64) (LiveView, error) {
	allowed, allocationWarning := m.coordinator.CanView(serviceID)
	if !allowed {
		return LiveView{ServiceID: serviceID, Warning: allocationWarning}, fmt.Errorf("live viewing is unavailable while recordings occupy all tuners")
	}
	if !m.capabilities.Available {
		return LiveView{ServiceID: serviceID, Warning: m.capabilities.Warning}, fmt.Errorf("FFmpeg is not ready: %s", m.capabilities.Warning)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if session := m.sessions[serviceID]; session != nil {
		session.lastTouch = time.Now()
		view := m.viewLocked(session)
		if view.Warning == "" {
			view.Warning = allocationWarning
		}
		return view, nil
	}
	directory, err := os.MkdirTemp(m.root, strconv.FormatUint(serviceID, 10)+"-")
	if err != nil {
		return LiveView{}, fmt.Errorf("create live session directory: %w", err)
	}
	ctx, cancel := context.WithCancel(m.ctx)
	session := &liveSession{
		serviceID: serviceID, directory: directory, cancel: cancel,
		encoder: m.capabilities.Selected, warning: m.capabilities.Warning, lastTouch: time.Now(),
	}
	if allocationWarning != "" {
		session.warning = allocationWarning
	}
	m.sessions[serviceID] = session
	if err := m.launchLocked(ctx, session, session.encoder); err != nil {
		delete(m.sessions, serviceID)
		cancel()
		os.RemoveAll(directory)
		return LiveView{}, err
	}
	return m.viewLocked(session), nil
}

// PlaylistPath returns the safe local path for an HLS playlist or segment.
func (m *LiveManager) PlaylistPath(serviceID uint64, name string) (string, LiveView, error) {
	if filepath.Base(name) != name || (filepath.Ext(name) != ".m3u8" && filepath.Ext(name) != ".ts") {
		return "", LiveView{}, fmt.Errorf("invalid HLS file name")
	}
	view, err := m.Start(serviceID)
	if err != nil {
		return "", view, err
	}
	m.mu.Lock()
	session := m.sessions[serviceID]
	if session == nil {
		m.mu.Unlock()
		return "", view, fmt.Errorf("live session stopped")
	}
	session.lastTouch = time.Now()
	path := filepath.Join(session.directory, name)
	view = m.viewLocked(session)
	m.mu.Unlock()
	return path, view, nil
}

// Status returns the current state of a service without creating a session.
func (m *LiveManager) Status(serviceID uint64) LiveView {
	m.mu.Lock()
	defer m.mu.Unlock()
	if session := m.sessions[serviceID]; session != nil {
		return m.viewLocked(session)
	}
	return LiveView{
		ServiceID: serviceID,
		Encoder:   m.capabilities.Selected.Name, Hardware: m.capabilities.Selected.Hardware,
		Warning: m.capabilities.Warning,
	}
}

// Capabilities returns the immutable startup probe result.
func (m *LiveManager) Capabilities() Capabilities { return m.capabilities }

// Close stops all FFmpeg processes and removes their temporary output.
func (m *LiveManager) Close() {
	m.cancel()
	m.mu.Lock()
	for _, session := range m.sessions {
		session.cancel()
		// Removing open HLS files is safe on supported Unix hosts and avoids
		// leaving cleanup work racing with application shutdown.
		_ = os.RemoveAll(session.directory)
	}
	m.sessions = make(map[uint64]*liveSession)
	m.mu.Unlock()
}

func (m *LiveManager) launchLocked(ctx context.Context, session *liveSession, encoder Encoder) error {
	input, err := url.Parse(m.mirakurunURL)
	if err != nil {
		return fmt.Errorf("build Mirakurun stream URL: %w", err)
	}
	input.Path = "/api/services/" + strconv.FormatUint(session.serviceID, 10) + "/stream"
	query := input.Query()
	query.Set("decode", "1")
	query.Set("priority", "0")
	input.RawQuery = query.Encode()
	playlist := filepath.Join(session.directory, "index.m3u8")
	command := exec.CommandContext(ctx, m.binary, LiveArguments(input.String(), playlist, encoder)...)
	stderr := &tailBuffer{limit: 8192}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start FFmpeg with %s: %w", encoder.Name, err)
	}
	session.encoder = encoder
	session.startedAt = time.Now()
	m.logger.Info("live transcoder started", "service_id", session.serviceID, "encoder", encoder.Name, "hardware", encoder.Hardware)
	go m.waitForProcess(ctx, session, command, stderr)
	return nil
}

func (m *LiveManager) waitForProcess(ctx context.Context, session *liveSession, command *exec.Cmd, stderr *tailBuffer) {
	err := command.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if ctx.Err() != nil || m.sessions[session.serviceID] != session {
		return
	}
	reason := compactError(err, stderr.String())
	_, playlistErr := os.Stat(filepath.Join(session.directory, "index.m3u8"))
	if session.encoder.Hardware && playlistErr != nil {
		session.warning = fmt.Sprintf("Hardware encoder %s failed (%s). Falling back to software encoding; CPU usage may be high.", session.encoder.Name, reason)
		m.logger.Warn("hardware encoding failed; falling back to software", "service_id", session.serviceID, "encoder", session.encoder.Name, "reason", reason)
		if launchErr := m.launchLocked(ctx, session, m.capabilities.Software); launchErr == nil {
			return
		} else {
			session.err = launchErr.Error()
		}
	} else {
		session.err = "FFmpeg stopped: " + reason
		m.logger.Error("transcoder stopped", "service_id", session.serviceID, "encoder", session.encoder.Name, "reason", reason)
	}
}

func (m *LiveManager) viewLocked(session *liveSession) LiveView {
	_, err := os.Stat(filepath.Join(session.directory, "index.m3u8"))
	session.ready = err == nil
	return LiveView{
		ServiceID: session.serviceID, Encoder: session.encoder.Name, Hardware: session.encoder.Hardware,
		Warning: session.warning, Error: session.err, Ready: session.ready,
	}
}

func (m *LiveManager) janitor() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case now := <-ticker.C:
			m.mu.Lock()
			for serviceID, session := range m.sessions {
				if now.Sub(session.lastTouch) > 45*time.Second {
					session.cancel()
					delete(m.sessions, serviceID)
					go os.RemoveAll(session.directory)
				}
			}
			m.mu.Unlock()
		}
	}
}

func compactError(err error, stderr string) string {
	value := stderr
	if value == "" && err != nil {
		value = err.Error()
	}
	if len(value) > 400 {
		value = value[len(value)-400:]
	}
	return value
}

// tailBuffer bounds long-running FFmpeg diagnostics while retaining the most
// useful final lines for an actionable user-facing error.
type tailBuffer struct {
	data  []byte
	limit int
}

func (b *tailBuffer) Write(value []byte) (int, error) {
	b.data = append(b.data, value...)
	if len(b.data) > b.limit {
		b.data = append(b.data[:0], b.data[len(b.data)-b.limit:]...)
	}
	return len(value), nil
}

func (b *tailBuffer) String() string { return string(b.data) }
