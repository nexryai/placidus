// Package recording provides durable reservations and supervised MPEG-TS recording.
package recording

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// State is the durable lifecycle state of a reservation.
type State string

const (
	StateScheduled State = "scheduled"
	StateRecording State = "recording"
	StateCompleted State = "completed"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
)

// Reservation describes one program recording request and its outcome.
type Reservation struct {
	ID        string    `json:"id"`
	ProgramID uint64    `json:"programId"`
	ServiceID uint64    `json:"serviceId"`
	Title     string    `json:"title"`
	StartAt   time.Time `json:"startAt"`
	EndAt     time.Time `json:"endAt"`
	State     State     `json:"state"`
	FileName  string    `json:"fileName,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store persists reservations as an atomically replaced JSON document.
type Store struct {
	mu    sync.RWMutex
	path  string
	items map[string]Reservation
}

// NewStore opens an existing reservation file or creates an empty in-memory store.
func NewStore(path string) (*Store, error) {
	store := &Store{path: path, items: make(map[string]Reservation)}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read reservation store: %w", err)
	}
	if len(data) == 0 {
		return store, nil
	}
	var items []Reservation
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("decode reservation store: %w", err)
	}
	for _, item := range items {
		store.items[item.ID] = item
	}
	return store, nil
}

// Create validates and persists a new reservation.
func (s *Store) Create(item Reservation) (Reservation, error) {
	if item.ServiceID == 0 || item.Title == "" || item.StartAt.IsZero() || !item.EndAt.After(item.StartAt) {
		return Reservation{}, fmt.Errorf("invalid reservation")
	}
	id, err := newID()
	if err != nil {
		return Reservation{}, err
	}
	now := time.Now().UTC()
	item.ID = id
	item.State = StateScheduled
	item.CreatedAt = now
	item.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	if err := s.writeLocked(); err != nil {
		delete(s.items, item.ID)
		return Reservation{}, err
	}
	return item, nil
}

// Get returns one reservation by its opaque identifier.
func (s *Store) Get(id string) (Reservation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	return item, ok
}

// List returns reservations ordered by start time, newest first.
func (s *Store) List() []Reservation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Reservation, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartAt.After(items[j].StartAt) })
	return items
}

// Update applies a state transition and persists it atomically.
func (s *Store) Update(id string, mutate func(*Reservation) error) (Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return Reservation{}, os.ErrNotExist
	}
	before := item
	if err := mutate(&item); err != nil {
		return Reservation{}, err
	}
	item.UpdatedAt = time.Now().UTC()
	s.items[id] = item
	if err := s.writeLocked(); err != nil {
		s.items[id] = before
		return Reservation{}, err
	}
	return item, nil
}

func (s *Store) writeLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create reservation directory: %w", err)
	}
	items := make([]Reservation, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".reservations-*.json")
	if err != nil {
		return fmt.Errorf("create temporary reservation store: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(items); err != nil {
		temporary.Close()
		return fmt.Errorf("encode reservation store: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync reservation store: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close reservation store: %w", err)
	}
	if err := os.Rename(temporaryName, s.path); err != nil {
		return fmt.Errorf("replace reservation store: %w", err)
	}
	return nil
}

func newID() (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create reservation ID: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}
