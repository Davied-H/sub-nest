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
			Users:       defaultUsers(now),
			InviteCodes: []domain.InviteCode{},
			Sources:     []domain.Source{},
			Outputs:     []domain.Output{},
			Updated:     now,
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
	changed := normalizeMultiUserConfig(&s.cfg)
	if s.cfg.Sources == nil {
		s.cfg.Sources = []domain.Source{}
	}
	if s.cfg.Outputs == nil {
		s.cfg.Outputs = []domain.Output{}
	}
	if resetInterruptedRefreshes(s.cfg.Sources) {
		changed = true
	}
	if changed {
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
	normalizeMultiUserConfig(&cfg)
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

func defaultUsers(now time.Time) []domain.User {
	return []domain.User{{
		ID:        "admin",
		Slug:      "admin",
		Name:      "Admin",
		Role:      "admin",
		Enabled:   true,
		CreatedAt: now,
	}}
}

func normalizeMultiUserConfig(cfg *domain.Config) bool {
	changed := false
	now := time.Now()
	if cfg.Users == nil {
		cfg.Users = []domain.User{}
		changed = true
	}
	if cfg.InviteCodes == nil {
		cfg.InviteCodes = []domain.InviteCode{}
		changed = true
	}
	adminFound := false
	for i := range cfg.Users {
		if cfg.Users[i].ID == "admin" {
			adminFound = true
			if cfg.Users[i].Slug == "" {
				cfg.Users[i].Slug = "admin"
				changed = true
			}
			if cfg.Users[i].Name == "" {
				cfg.Users[i].Name = "Admin"
				changed = true
			}
			if cfg.Users[i].Role == "" {
				cfg.Users[i].Role = "admin"
				changed = true
			}
			if !cfg.Users[i].Enabled {
				cfg.Users[i].Enabled = true
				changed = true
			}
			if cfg.Users[i].CreatedAt.IsZero() {
				cfg.Users[i].CreatedAt = now
				changed = true
			}
			continue
		}
		if cfg.Users[i].Role == "" {
			cfg.Users[i].Role = "user"
			changed = true
		}
		if cfg.Users[i].CreatedAt.IsZero() {
			cfg.Users[i].CreatedAt = now
			changed = true
		}
	}
	if !adminFound {
		cfg.Users = append(defaultUsers(now), cfg.Users...)
		changed = true
	}
	for i := range cfg.Sources {
		if cfg.Sources[i].OwnerUserID == "" {
			cfg.Sources[i].OwnerUserID = "admin"
			changed = true
		}
	}
	for i := range cfg.Outputs {
		if cfg.Outputs[i].OwnerUserID == "" {
			cfg.Outputs[i].OwnerUserID = "admin"
			changed = true
		}
	}
	return changed
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
