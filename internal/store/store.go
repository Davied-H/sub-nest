package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"sub-nest/internal/domain"
)

type Store struct {
	path string
	mu   sync.RWMutex
	cfg  domain.Config
}

func New(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		now := time.Now()
		s.cfg = domain.Config{
			Version: 1,
			Settings: domain.Settings{
				RefreshMinutes: 60,
			},
			Sources: []domain.Source{},
			Outputs: []domain.Output{},
			Updated: now,
		}
		return s.saveLocked()
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &s.cfg); err != nil {
		return err
	}
	if s.cfg.Version == 0 {
		s.cfg.Version = 1
	}
	if s.cfg.Settings.RefreshMinutes == 0 {
		s.cfg.Settings.RefreshMinutes = 60
	}
	if s.cfg.Sources == nil {
		s.cfg.Sources = []domain.Source{}
	}
	if s.cfg.Outputs == nil {
		s.cfg.Outputs = []domain.Output{}
	}
	if resetInterruptedRefreshes(s.cfg.Sources) {
		return s.saveLocked()
	}
	return nil
}

func (s *Store) Snapshot() domain.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneConfig(s.cfg)
}

func (s *Store) Update(fn func(*domain.Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := cloneConfig(s.cfg)
	if err := fn(&next); err != nil {
		return err
	}
	next.Updated = time.Now()
	s.cfg = next
	return s.saveLocked()
}

func (s *Store) Replace(cfg domain.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Settings.RefreshMinutes == 0 {
		cfg.Settings.RefreshMinutes = 60
	}
	cfg.Updated = time.Now()
	s.cfg = cloneConfig(cfg)
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func cloneConfig(cfg domain.Config) domain.Config {
	data, _ := json.Marshal(cfg)
	var out domain.Config
	_ = json.Unmarshal(data, &out)
	return out
}

func resetInterruptedRefreshes(sources []domain.Source) bool {
	changed := false
	for i := range sources {
		if sources[i].LastStatus != "refreshing" {
			continue
		}
		sources[i].LastStatus = "error"
		sources[i].LastError = "刷新任务已中断，请重新刷新"
		sources[i].RefreshProgress = ""
		sources[i].RefreshPercent = 0
		changed = true
	}
	return changed
}
