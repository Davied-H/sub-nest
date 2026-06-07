package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"sub-nest/internal/domain"
)

type Store struct {
	path string
	mu   sync.RWMutex
	cfg  domain.Config
}

type userConfig struct {
	Version     int                 `json:"version"`
	OwnerUserID string              `json:"ownerUserId"`
	Sources     []domain.Source     `json:"sources"`
	RuleSources []domain.RuleSource `json:"ruleSources"`
	RuleSets    []domain.RuleSet    `json:"ruleSets"`
	Outputs     []domain.Output     `json:"outputs"`
	Updated     time.Time           `json:"updated"`
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
				RefreshMinutes:      60,
				TrafficQueryMinutes: 5,
			},
			Users:       defaultUsers(now),
			InviteCodes: []domain.InviteCode{},
			Sources:     []domain.Source{},
			RuleSources: domain.DefaultRuleSources("admin"),
			RuleSets:    []domain.RuleSet{domain.DefaultRuleSet("admin")},
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
	hadInlinePrivateConfig := hasPrivateConfig(s.cfg)
	if _, err := s.loadUserConfigsLocked(); err != nil {
		return err
	}
	if s.cfg.Version == 0 {
		s.cfg.Version = 1
	}
	if s.cfg.Settings.RefreshMinutes == 0 {
		s.cfg.Settings.RefreshMinutes = 60
	}
	if s.cfg.Settings.TrafficQueryMinutes == 0 {
		s.cfg.Settings.TrafficQueryMinutes = 5
	}
	normalized := normalizeMultiUserConfig(&s.cfg)
	changed := hadInlinePrivateConfig || normalized
	if s.cfg.Sources == nil {
		s.cfg.Sources = []domain.Source{}
	}
	if s.cfg.RuleSources == nil {
		s.cfg.RuleSources = []domain.RuleSource{}
		changed = true
	}
	if s.cfg.RuleSets == nil {
		s.cfg.RuleSets = []domain.RuleSet{}
		changed = true
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
	if cfg.Settings.TrafficQueryMinutes == 0 {
		cfg.Settings.TrafficQueryMinutes = 5
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
	if err := os.MkdirAll(s.usersDir(), 0o755); err != nil {
		return err
	}
	ownerIDs := privateConfigOwnerIDs(s.cfg)
	if err := s.removeStaleUserConfigsLocked(ownerIDs); err != nil {
		return err
	}
	for _, ownerID := range ownerIDs {
		if err := s.saveUserConfigLocked(ownerID); err != nil {
			return err
		}
	}
	cfg := globalConfig(s.cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) loadUserConfigsLocked() (bool, error) {
	files, err := os.ReadDir(s.usersDir())
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	privateConfigs := map[string]userConfig{}
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}
		path := filepath.Join(s.usersDir(), file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return false, err
		}
		var userCfg userConfig
		if err := json.Unmarshal(data, &userCfg); err != nil {
			return false, err
		}
		ownerID := userCfg.OwnerUserID
		if ownerID == "" {
			ownerID = strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		}
		if ownerID == "" || ownerID != safeUserConfigName(ownerID) {
			continue
		}
		privateConfigs[ownerID] = userCfg
	}
	if len(privateConfigs) == 0 {
		return false, nil
	}
	s.cfg.Sources = []domain.Source{}
	s.cfg.RuleSources = []domain.RuleSource{}
	s.cfg.RuleSets = []domain.RuleSet{}
	s.cfg.Outputs = []domain.Output{}
	for _, ownerID := range sortedMapKeys(privateConfigs) {
		appendPrivateConfig(&s.cfg, ownerID, privateConfigs[ownerID])
	}
	return true, nil
}

func (s *Store) saveUserConfigLocked(ownerID string) error {
	userCfg := privateConfigForOwner(s.cfg, ownerID)
	data, err := json.MarshalIndent(userCfg, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.usersDir(), safeUserConfigName(ownerID)+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) usersDir() string {
	return filepath.Join(filepath.Dir(s.path), "users")
}

func (s *Store) removeStaleUserConfigsLocked(ownerIDs []string) error {
	allowed := map[string]bool{}
	for _, ownerID := range ownerIDs {
		allowed[safeUserConfigName(ownerID)+".json"] = true
	}
	files, err := os.ReadDir(s.usersDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" || allowed[file.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(s.usersDir(), file.Name())); err != nil {
			return err
		}
	}
	return nil
}

func cloneConfig(cfg domain.Config) domain.Config {
	data, _ := json.Marshal(cfg)
	var out domain.Config
	_ = json.Unmarshal(data, &out)
	return out
}

func globalConfig(cfg domain.Config) domain.Config {
	return domain.Config{
		Version:     cfg.Version,
		Settings:    cfg.Settings,
		Users:       cfg.Users,
		InviteCodes: cfg.InviteCodes,
		Sources:     []domain.Source{},
		RuleSources: []domain.RuleSource{},
		RuleSets:    []domain.RuleSet{},
		Outputs:     []domain.Output{},
		Updated:     cfg.Updated,
	}
}

func hasPrivateConfig(cfg domain.Config) bool {
	return len(cfg.Sources) > 0 || len(cfg.RuleSources) > 0 || len(cfg.RuleSets) > 0 || len(cfg.Outputs) > 0
}

func privateConfigOwnerIDs(cfg domain.Config) []string {
	seen := map[string]bool{}
	for _, user := range cfg.Users {
		if user.ID != "" {
			seen[user.ID] = true
		}
	}
	for _, source := range cfg.Sources {
		if source.OwnerUserID != "" {
			seen[source.OwnerUserID] = true
		}
	}
	for _, source := range cfg.RuleSources {
		if source.OwnerUserID != "" {
			seen[source.OwnerUserID] = true
		}
	}
	for _, set := range cfg.RuleSets {
		if set.OwnerUserID != "" {
			seen[set.OwnerUserID] = true
		}
	}
	for _, output := range cfg.Outputs {
		if output.OwnerUserID != "" {
			seen[output.OwnerUserID] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		if id == safeUserConfigName(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func sortedMapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func privateConfigForOwner(cfg domain.Config, ownerID string) userConfig {
	out := userConfig{
		Version:     cfg.Version,
		OwnerUserID: ownerID,
		Sources:     []domain.Source{},
		RuleSources: []domain.RuleSource{},
		RuleSets:    []domain.RuleSet{},
		Outputs:     []domain.Output{},
		Updated:     cfg.Updated,
	}
	for _, source := range cfg.Sources {
		if source.OwnerUserID == ownerID {
			out.Sources = append(out.Sources, source)
		}
	}
	for _, source := range cfg.RuleSources {
		if source.OwnerUserID == ownerID {
			out.RuleSources = append(out.RuleSources, source)
		}
	}
	for _, set := range cfg.RuleSets {
		if set.OwnerUserID == ownerID {
			out.RuleSets = append(out.RuleSets, set)
		}
	}
	for _, output := range cfg.Outputs {
		if output.OwnerUserID == ownerID {
			out.Outputs = append(out.Outputs, output)
		}
	}
	return out
}

func appendPrivateConfig(cfg *domain.Config, ownerID string, userCfg userConfig) {
	for _, source := range userCfg.Sources {
		source.OwnerUserID = ownerID
		cfg.Sources = append(cfg.Sources, source)
	}
	for _, source := range userCfg.RuleSources {
		source.OwnerUserID = ownerID
		cfg.RuleSources = append(cfg.RuleSources, source)
	}
	for _, set := range userCfg.RuleSets {
		set.OwnerUserID = ownerID
		cfg.RuleSets = append(cfg.RuleSets, set)
	}
	for _, output := range userCfg.Outputs {
		output.OwnerUserID = ownerID
		cfg.Outputs = append(cfg.Outputs, output)
	}
}

func safeUserConfigName(ownerID string) string {
	ownerID = strings.TrimSpace(ownerID)
	var b strings.Builder
	for _, r := range ownerID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
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
	for i := range cfg.RuleSources {
		if cfg.RuleSources[i].OwnerUserID == "" {
			cfg.RuleSources[i].OwnerUserID = "admin"
			changed = true
		}
		normalized := domain.NormalizeRuleSource(cfg.RuleSources[i])
		if !reflect.DeepEqual(cfg.RuleSources[i], normalized) {
			changed = true
		}
		cfg.RuleSources[i] = normalized
	}
	for i := range cfg.RuleSets {
		if cfg.RuleSets[i].OwnerUserID == "" {
			cfg.RuleSets[i].OwnerUserID = "admin"
			changed = true
		}
		normalized := domain.NormalizeRuleSet(cfg.RuleSets[i])
		if !reflect.DeepEqual(cfg.RuleSets[i], normalized) {
			changed = true
		}
		cfg.RuleSets[i] = normalized
	}
	owners := map[string]bool{}
	for _, user := range cfg.Users {
		if user.ID != "" {
			owners[user.ID] = true
		}
	}
	for i := range cfg.Outputs {
		if cfg.Outputs[i].OwnerUserID == "" {
			cfg.Outputs[i].OwnerUserID = "admin"
			changed = true
		}
		owners[cfg.Outputs[i].OwnerUserID] = true
		normalized := domain.NormalizePACConfig(cfg.Outputs[i].PAC)
		if !reflect.DeepEqual(cfg.Outputs[i].PAC, normalized) {
			changed = true
		}
		cfg.Outputs[i].PAC = normalized
		if migrateLegacyPACToRuleSet(cfg, cfg.Outputs[i]) {
			changed = true
		}
	}
	for ownerID := range owners {
		if ensureDefaultRules(cfg, ownerID) {
			changed = true
		}
	}
	return changed
}

func ensureDefaultRules(cfg *domain.Config, ownerID string) bool {
	changed := false
	for _, source := range domain.DefaultRuleSources(ownerID) {
		if findRuleSourceIndex(*cfg, ownerID, source.ID) < 0 {
			cfg.RuleSources = append(cfg.RuleSources, source)
			changed = true
		}
	}
	if findRuleSetIndex(*cfg, ownerID, domain.DefaultRuleSetID) < 0 {
		cfg.RuleSets = append(cfg.RuleSets, domain.DefaultRuleSet(ownerID))
		changed = true
	} else {
		setIndex := findRuleSetIndex(*cfg, ownerID, domain.DefaultRuleSetID)
		set := domain.NormalizeRuleSet(cfg.RuleSets[setIndex])
		nextSourceIDs := domain.NormalizeStringList(append(set.SourceIDs, domain.DefaultRuleSourceIDs...))
		if !reflect.DeepEqual(set.SourceIDs, nextSourceIDs) {
			set.SourceIDs = nextSourceIDs
			cfg.RuleSets[setIndex] = set
			changed = true
		}
	}
	return changed
}

func migrateLegacyPACToRuleSet(cfg *domain.Config, output domain.Output) bool {
	pac := domain.NormalizePACConfig(output.PAC)
	if pac.RuleSetID != domain.DefaultRuleSetID {
		return false
	}
	changed := ensureDefaultRules(cfg, output.OwnerUserID)
	sourceIndex := findRuleSourceIndex(*cfg, output.OwnerUserID, domain.DefaultRuleSourceID)
	setIndex := findRuleSetIndex(*cfg, output.OwnerUserID, domain.DefaultRuleSetID)
	if sourceIndex >= 0 {
		source := cfg.RuleSources[sourceIndex]
		if pac.RuleSourceURL != "" && source.URL != pac.RuleSourceURL {
			source.URL = pac.RuleSourceURL
			changed = true
		}
		if pac.RuleSourceFormat != "" && source.Format != pac.RuleSourceFormat {
			source.Format = pac.RuleSourceFormat
			changed = true
		}
		if pac.RuleRefreshHours > 0 && source.RefreshHours != pac.RuleRefreshHours {
			source.RefreshHours = pac.RuleRefreshHours
			changed = true
		}
		if pac.RuleSourceURL == domain.DefaultPACRuleSourceURL && source.LocalPath == "" {
			source.LocalPath = domain.DefaultPACRuleSourceLocalPath
			changed = true
		}
		source.CachedDomainSuffixes = domain.NormalizeStringList(append(source.CachedDomainSuffixes, pac.CachedDomainSuffixes...))
		source.LastSyncedAt = newestTime(source.LastSyncedAt, pac.LastSyncedAt)
		if source.LastSyncStatus == "" && pac.LastSyncStatus != "" {
			source.LastSyncStatus = pac.LastSyncStatus
			source.LastSyncError = pac.LastSyncError
		}
		cfg.RuleSources[sourceIndex] = domain.NormalizeRuleSource(source)
	}
	if setIndex >= 0 {
		set := cfg.RuleSets[setIndex]
		set.DirectDomainSuffixes = domain.NormalizeStringList(append(set.DirectDomainSuffixes, pac.DirectDomainSuffixes...))
		set.DirectCIDRs = domain.NormalizeStringList(append(set.DirectCIDRs, pac.DirectCIDRs...))
		cfg.RuleSets[setIndex] = domain.NormalizeRuleSet(set)
	}
	return changed
}

func newestTime(a *time.Time, b *time.Time) *time.Time {
	if a == nil {
		return b
	}
	if b == nil || a.After(*b) {
		return a
	}
	return b
}

func findRuleSourceIndex(cfg domain.Config, ownerID string, id string) int {
	for i, source := range cfg.RuleSources {
		if source.OwnerUserID == ownerID && source.ID == id {
			return i
		}
	}
	return -1
}

func findRuleSetIndex(cfg domain.Config, ownerID string, id string) int {
	for i, set := range cfg.RuleSets {
		if set.OwnerUserID == ownerID && set.ID == id {
			return i
		}
	}
	return -1
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
