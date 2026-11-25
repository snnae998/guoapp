// Package history remembers, per series, the last episode played and whether
// it was watched to the end, so playback can resume. It is a local JSON file
// with mode 0600 and holds no account or device data.
package history

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Entry is the watch position of one series.
type Entry struct {
	SeriesID string `json:"series_id"`
	Title    string `json:"title,omitempty"`
	// Episode is the last episode started; Finished is set once it played
	// to the end.
	Episode  int  `json:"episode"`
	Episodes int  `json:"episodes,omitempty"`
	Finished bool `json:"finished"`
	// Position is where an unfinished episode was left, in seconds.
	Position  float64   `json:"position,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store keeps entries in one file.
type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store { return &Store{path: path} }

// List returns every entry, most recently updated first.
func (s *Store) List(ctx context.Context) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(all))
	for _, e := range all {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Get returns the entry of seriesID, if any.
func (s *Store) Get(ctx context.Context, seriesID string) (Entry, bool, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.load()
	if err != nil {
		return Entry{}, false, err
	}
	e, ok := all[seriesID]
	return e, ok, nil
}

// Record replaces the entry of e.SeriesID. A zero UpdatedAt is set to now.
func (s *Store) Record(ctx context.Context, e Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if e.SeriesID == "" || e.Episode < 1 {
		return errors.New("history entry needs a series ID and an episode")
	}
	if e.UpdatedAt.IsZero() {
		e.UpdatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.load()
	if err != nil {
		return err
	}
	all[e.SeriesID] = e
	return s.save(all)
}

// Clear removes the history file.
func (s *Store) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := rejectSymlink(s.path); err != nil {
		return err
	}
	err := os.Remove(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) load() (map[string]Entry, error) {
	if err := rejectSymlink(s.path); err != nil {
		return nil, err
	}
	all := map[string]Entry{}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return all, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, fmt.Errorf("history file is corrupt: %w", err)
	}
	return all, nil
}

// save writes through a temp file and a rename so a crash never leaves a
// half-written history.
func (s *Store) save(all map[string]Entry) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".ohg-history-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(all); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.path)
}

func rejectSymlink(path string) error {
	if path == "" {
		return errors.New("history path is empty")
	}
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("history path is a symlink")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
