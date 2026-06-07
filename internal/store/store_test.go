package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sub-nest/internal/domain"
)

func TestStoreSplitsPrivateConfigByUserFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	st, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	now := time.Now()
	err = st.Update(func(cfg *domain.Config) error {
		cfg.Users = []domain.User{
			{ID: "admin", Slug: "admin", Name: "Admin", Role: "admin", Enabled: true, CreatedAt: now},
			{ID: "alice", Slug: "alice", Name: "Alice", Role: "user", Enabled: true, CreatedAt: now},
		}
		cfg.Sources = []domain.Source{
			{ID: "admin-source", OwnerUserID: "admin", Name: "Admin Source"},
			{ID: "alice-source", OwnerUserID: "alice", Name: "Alice Source"},
		}
		cfg.RuleSources = []domain.RuleSource{
			{ID: "admin-rule", OwnerUserID: "admin", Name: "Admin Rule", Format: "domain-list"},
			{ID: "alice-rule", OwnerUserID: "alice", Name: "Alice Rule", Format: "domain-list"},
		}
		cfg.RuleSets = []domain.RuleSet{
			{ID: "admin-set", OwnerUserID: "admin", Name: "Admin Set", SourceIDs: []string{"admin-rule"}},
			{ID: "alice-set", OwnerUserID: "alice", Name: "Alice Set", SourceIDs: []string{"alice-rule"}},
		}
		cfg.Outputs = []domain.Output{
			{ID: "admin-output", OwnerUserID: "admin", Slug: "main", Name: "Admin Output"},
			{ID: "alice-output", OwnerUserID: "alice", Slug: "main", Name: "Alice Output"},
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	var global domain.Config
	readJSON(t, path, &global)
	if len(global.Sources) != 0 || len(global.RuleSources) != 0 || len(global.RuleSets) != 0 || len(global.Outputs) != 0 {
		t.Fatalf("global config contains private data: %#v", global)
	}
	if len(global.Users) != 2 {
		t.Fatalf("global users = %d, want 2", len(global.Users))
	}

	var adminFile userConfig
	readJSON(t, filepath.Join(dir, "users", "admin.json"), &adminFile)
	if got := adminFile.Sources[0].ID; got != "admin-source" {
		t.Fatalf("admin source = %q, want admin-source", got)
	}
	if got := adminFile.Outputs[0].ID; got != "admin-output" {
		t.Fatalf("admin output = %q, want admin-output", got)
	}

	var aliceFile userConfig
	readJSON(t, filepath.Join(dir, "users", "alice.json"), &aliceFile)
	if got := aliceFile.Sources[0].ID; got != "alice-source" {
		t.Fatalf("alice source = %q, want alice-source", got)
	}
	if got := aliceFile.Outputs[0].ID; got != "alice-output" {
		t.Fatalf("alice output = %q, want alice-output", got)
	}

	reloaded, err := New(path)
	if err != nil {
		t.Fatalf("reload New: %v", err)
	}
	cfg := reloaded.Snapshot()
	if len(cfg.Sources) != 2 || len(cfg.RuleSources) < 2 || len(cfg.RuleSets) < 2 || len(cfg.Outputs) != 2 {
		t.Fatalf("reloaded private data missing: sources=%d ruleSources=%d ruleSets=%d outputs=%d", len(cfg.Sources), len(cfg.RuleSources), len(cfg.RuleSets), len(cfg.Outputs))
	}
}

func TestStoreMigratesLegacySingleFileToUserFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	now := time.Now()
	legacy := domain.Config{
		Version: 1,
		Settings: domain.Settings{
			RefreshMinutes:      60,
			TrafficQueryMinutes: 5,
		},
		Users: []domain.User{
			{ID: "admin", Slug: "admin", Name: "Admin", Role: "admin", Enabled: true, CreatedAt: now},
			{ID: "alice", Slug: "alice", Name: "Alice", Role: "user", Enabled: true, CreatedAt: now},
		},
		Sources: []domain.Source{
			{ID: "admin-source", OwnerUserID: "admin", Name: "Admin Source"},
			{ID: "alice-source", OwnerUserID: "alice", Name: "Alice Source"},
		},
		Outputs: []domain.Output{
			{ID: "admin-output", OwnerUserID: "admin", Slug: "main", Name: "Admin Output"},
			{ID: "alice-output", OwnerUserID: "alice", Slug: "main", Name: "Alice Output"},
		},
		Updated: now,
	}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	st, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cfg := st.Snapshot()
	if len(cfg.Sources) != 2 || len(cfg.Outputs) != 2 {
		t.Fatalf("snapshot lost private data: sources=%d outputs=%d", len(cfg.Sources), len(cfg.Outputs))
	}
	var global domain.Config
	readJSON(t, path, &global)
	if len(global.Sources) != 0 || len(global.Outputs) != 0 {
		t.Fatalf("legacy private data not removed from global config: %#v", global)
	}
	var aliceFile userConfig
	readJSON(t, filepath.Join(dir, "users", "alice.json"), &aliceFile)
	if got := aliceFile.Sources[0].ID; got != "alice-source" {
		t.Fatalf("migrated alice source = %q, want alice-source", got)
	}
}

func readJSON(t *testing.T, path string, out interface{}) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
