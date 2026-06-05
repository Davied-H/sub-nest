package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"sub-nest/internal/aggregator"
	"sub-nest/internal/domain"
	"sub-nest/internal/store"
)

type Server struct {
	store      *store.Store
	fetcher    *aggregator.Fetcher
	staticPath string
	logger     *slog.Logger
}

func New(st *store.Store, staticPath string, logger *slog.Logger) *Server {
	return &Server{
		store:      st,
		fetcher:    aggregator.NewFetcher(),
		staticPath: staticPath,
		logger:     logger,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/setup", s.handleSetup)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("GET /s/{slug}", s.handlePublicSubscription)

	mux.Handle("GET /api/dashboard", s.auth(http.HandlerFunc(s.handleDashboard)))
	mux.Handle("GET /api/settings", s.auth(http.HandlerFunc(s.handleSettings)))
	mux.Handle("PUT /api/settings/admin-token", s.auth(http.HandlerFunc(s.handleUpdateAdminToken)))
	mux.Handle("PUT /api/settings/user-token", s.auth(http.HandlerFunc(s.handleUpdateUserToken)))
	mux.Handle("GET /api/sources", s.auth(http.HandlerFunc(s.handleSources)))
	mux.Handle("POST /api/sources", s.auth(http.HandlerFunc(s.handleCreateSource)))
	mux.Handle("PUT /api/sources/{id}", s.auth(http.HandlerFunc(s.handleUpdateSource)))
	mux.Handle("DELETE /api/sources/{id}", s.auth(http.HandlerFunc(s.handleDeleteSource)))
	mux.Handle("POST /api/sources/{id}/refresh", s.auth(http.HandlerFunc(s.handleRefreshSource)))
	mux.Handle("POST /api/refresh", s.auth(http.HandlerFunc(s.handleRefreshAll)))

	mux.Handle("GET /api/outputs", s.auth(http.HandlerFunc(s.handleOutputs)))
	mux.Handle("POST /api/outputs", s.auth(http.HandlerFunc(s.handleCreateOutput)))
	mux.Handle("PUT /api/outputs/{id}", s.auth(http.HandlerFunc(s.handleUpdateOutput)))
	mux.Handle("DELETE /api/outputs/{id}", s.auth(http.HandlerFunc(s.handleDeleteOutput)))
	mux.Handle("GET /api/outputs/{id}/preview", s.auth(http.HandlerFunc(s.handlePreview)))
	mux.Handle("GET /api/backup", s.auth(http.HandlerFunc(s.handleBackup)))
	mux.Handle("POST /api/restore", s.auth(http.HandlerFunc(s.handleRestore)))

	if s.staticPath != "" {
		mux.HandleFunc("/", s.handleStatic)
	}
	return withCORS(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":              true,
		"needsAdminSetup": s.needsSetup(),
	})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Snapshot()
	if cfg.Settings.AdminTokenHash != "" {
		writeError(w, http.StatusConflict, "管理员 token 已设置")
		return
	}
	var req struct {
		Token         string `json:"token"`
		PublicBaseURL string `json:"publicBaseUrl"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	token := strings.TrimSpace(req.Token)
	hash, err := hashToken(token)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	err = s.store.Update(func(cfg *domain.Config) error {
		cfg.Settings.AdminTokenHash = string(hash)
		cfg.Settings.PublicBaseURL = strings.TrimRight(req.PublicBaseURL, "/")
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "配置保存失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": issueSessionToken(token)})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	cfg := s.store.Snapshot()
	if cfg.Settings.AdminTokenHash == "" {
		writeError(w, http.StatusPreconditionRequired, "需要先初始化管理员 token")
		return
	}
	token := strings.TrimSpace(req.Token)
	if bcrypt.CompareHashAndPassword([]byte(cfg.Settings.AdminTokenHash), []byte(token)) != nil {
		writeError(w, http.StatusUnauthorized, "token 不正确")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": issueSessionToken(token)})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Snapshot()
	writeJSON(w, http.StatusOK, domain.SettingsView{
		PublicBaseURL: cfg.Settings.PublicBaseURL,
		HasUserToken:  cfg.Settings.UserTokenHash != "",
	})
}

func (s *Server) handleUpdateAdminToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentToken string `json:"currentToken"`
		NewToken     string `json:"newToken"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	currentToken := strings.TrimSpace(req.CurrentToken)
	newToken := strings.TrimSpace(req.NewToken)
	cfg := s.store.Snapshot()
	if bcrypt.CompareHashAndPassword([]byte(cfg.Settings.AdminTokenHash), []byte(currentToken)) != nil {
		writeError(w, http.StatusUnauthorized, "当前管理员 token 不正确")
		return
	}
	hash, err := hashToken(newToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentToken)) == nil {
		writeError(w, http.StatusBadRequest, "新 token 不能与当前 token 相同")
		return
	}
	err = s.store.Update(func(cfg *domain.Config) error {
		cfg.Settings.AdminTokenHash = string(hash)
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "管理员 token 保存失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": issueSessionToken(newToken)})
}

func (s *Server) handleUpdateUserToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	token := strings.TrimSpace(req.Token)
	var hash []byte
	var err error
	if token != "" {
		hash, err = hashToken(token)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	err = s.store.Update(func(cfg *domain.Config) error {
		cfg.Settings.UserTokenHash = string(hash)
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "用户 token 保存失败")
		return
	}
	writeJSON(w, http.StatusOK, domain.SettingsView{
		PublicBaseURL: s.store.Snapshot().Settings.PublicBaseURL,
		HasUserToken:  token != "",
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Snapshot()
	dashboard := domain.Dashboard{
		SourceCount:     len(cfg.Sources),
		OutputCount:     len(cfg.Outputs),
		NeedsAdminSetup: cfg.Settings.AdminTokenHash == "",
	}
	for _, source := range cfg.Sources {
		if source.Enabled {
			dashboard.EnabledSources++
		}
		if source.LastStatus == "ok" {
			dashboard.HealthySources++
		}
		if source.LastStatus == "error" {
			dashboard.UnhealthySources++
		}
		dashboard.TotalCachedNodes += len(source.CachedNodes)
		if source.LastRefreshedAt != nil && (dashboard.LastRefreshAt == nil || source.LastRefreshedAt.After(*dashboard.LastRefreshAt)) {
			t := *source.LastRefreshedAt
			dashboard.LastRefreshAt = &t
		}
	}
	for _, output := range cfg.Outputs {
		if output.Enabled {
			dashboard.EnabledOutputs++
		}
	}
	base := originFromRequest(r)
	dashboard.PublicExampleURL = strings.TrimRight(base, "/") + "/s/main"
	writeJSON(w, http.StatusOK, dashboard)
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Snapshot()
	includeURL := r.URL.Query().Get("includeUrl") == "1"
	views := make([]domain.SourceView, 0, len(cfg.Sources))
	for _, source := range cfg.Sources {
		views = append(views, domain.SourceToView(source, includeURL))
	}
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	var req domain.Source
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.ID = uuid.NewString()
	normalizeSourceInput(&req)
	if !sourceInputReady(req) {
		writeError(w, http.StatusBadRequest, "名称和订阅来源不能为空")
		return
	}
	if req.LastStatus == "" {
		req.LastStatus = "pending"
	}
	err := s.store.Update(func(cfg *domain.Config) error {
		cfg.Sources = append(cfg.Sources, req)
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存失败")
		return
	}
	writeJSON(w, http.StatusCreated, domain.SourceToView(req, true))
}

func (s *Server) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req domain.Source
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	normalizeSourceInput(&req)
	if !sourceInputReady(req) {
		writeError(w, http.StatusBadRequest, "名称和订阅来源不能为空")
		return
	}
	var updated domain.Source
	err := s.store.Update(func(cfg *domain.Config) error {
		for i := range cfg.Sources {
			if cfg.Sources[i].ID == id {
				cfg.Sources[i].Name = req.Name
				cfg.Sources[i].URL = req.URL
				cfg.Sources[i].SourceType = req.SourceType
				cfg.Sources[i].FileName = req.FileName
				cfg.Sources[i].FileContent = req.FileContent
				cfg.Sources[i].Enabled = req.Enabled
				cfg.Sources[i].Remark = req.Remark
				cfg.Sources[i].Tags = req.Tags
				updated = cfg.Sources[i]
				return nil
			}
		}
		return errors.New("not found")
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "订阅源不存在")
		return
	}
	writeJSON(w, http.StatusOK, domain.SourceToView(updated, true))
}

func normalizeSourceInput(source *domain.Source) {
	source.Name = strings.TrimSpace(source.Name)
	source.URL = strings.TrimSpace(source.URL)
	source.FileName = strings.TrimSpace(source.FileName)
	if source.SourceType != "file" {
		source.SourceType = "url"
		source.FileName = ""
		source.FileContent = ""
		return
	}
	source.URL = ""
}

func sourceInputReady(source domain.Source) bool {
	if source.Name == "" {
		return false
	}
	if source.SourceType == "file" {
		return strings.TrimSpace(source.FileContent) != ""
	}
	return strings.TrimSpace(source.URL) != ""
}

func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.Update(func(cfg *domain.Config) error {
		next := cfg.Sources[:0]
		found := false
		for _, source := range cfg.Sources {
			if source.ID == id {
				found = true
				continue
			}
			next = append(next, source)
		}
		if !found {
			return errors.New("not found")
		}
		cfg.Sources = next
		for i := range cfg.Outputs {
			cfg.Outputs[i].SourceIDs = removeString(cfg.Outputs[i].SourceIDs, id)
		}
		return nil
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "订阅源不存在")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRefreshSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	source, err := s.startRefreshSource(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "订阅源不存在")
		return
	}
	writeJSON(w, http.StatusAccepted, domain.SourceToView(source, false))
}

func (s *Server) handleRefreshAll(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Snapshot()
	views := []domain.SourceView{}
	for _, source := range cfg.Sources {
		if !source.Enabled {
			continue
		}
		refreshed, err := s.startRefreshSource(source.ID)
		if err != nil {
			continue
		}
		views = append(views, domain.SourceToView(refreshed, false))
	}
	writeJSON(w, http.StatusAccepted, views)
}

func (s *Server) handleOutputs(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Snapshot()
	writeJSON(w, http.StatusOK, cfg.Outputs)
}

func (s *Server) handleCreateOutput(w http.ResponseWriter, r *http.Request) {
	var req domain.Output
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "名称和 slug 不能为空")
		return
	}
	req.ID = uuid.NewString()
	req.Slug = normalizeSlug(req.Slug)
	if req.Format == "" {
		req.Format = "clash"
	}
	if req.GroupMode == "" {
		req.GroupMode = "region"
	}
	err := s.store.Update(func(cfg *domain.Config) error {
		for _, output := range cfg.Outputs {
			if output.Slug == req.Slug {
				return errors.New("slug exists")
			}
		}
		cfg.Outputs = append(cfg.Outputs, req)
		return nil
	})
	if err != nil {
		writeError(w, http.StatusConflict, "slug 已存在")
		return
	}
	writeJSON(w, http.StatusCreated, req)
}

func (s *Server) handleUpdateOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req domain.Output
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	var updated domain.Output
	err := s.store.Update(func(cfg *domain.Config) error {
		for i := range cfg.Outputs {
			if cfg.Outputs[i].ID == id {
				req.ID = id
				req.Slug = normalizeSlug(req.Slug)
				if req.Slug == "" {
					return errors.New("empty slug")
				}
				for _, output := range cfg.Outputs {
					if output.ID != id && output.Slug == req.Slug {
						return errors.New("slug exists")
					}
				}
				cfg.Outputs[i] = req
				updated = req
				return nil
			}
		}
		return errors.New("not found")
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "公开订阅保存失败")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.Update(func(cfg *domain.Config) error {
		next := cfg.Outputs[:0]
		found := false
		for _, output := range cfg.Outputs {
			if output.ID == id {
				found = true
				continue
			}
			next = append(next, output)
		}
		if !found {
			return errors.New("not found")
		}
		cfg.Outputs = next
		return nil
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "公开订阅不存在")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cfg := s.store.Snapshot()
	for _, output := range cfg.Outputs {
		if output.ID == id {
			writeJSON(w, http.StatusOK, aggregator.Preview(cfg, output))
			return
		}
	}
	writeError(w, http.StatusNotFound, "公开订阅不存在")
}

func (s *Server) handlePublicSubscription(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	cfg := s.store.Snapshot()
	if cfg.Settings.UserTokenHash != "" {
		token := subscriptionTokenFromRequest(r)
		if bcrypt.CompareHashAndPassword([]byte(cfg.Settings.UserTokenHash), []byte(token)) != nil {
			writeError(w, http.StatusUnauthorized, "订阅 token 不正确")
			return
		}
	}
	for _, output := range cfg.Outputs {
		if output.Slug != slug {
			continue
		}
		if !output.Enabled {
			writeError(w, http.StatusForbidden, "订阅地址已暂停")
			return
		}
		if format := normalizeOutputFormat(r.URL.Query().Get("format")); format != "" {
			output.Format = format
		}
		result := aggregator.Build(cfg, output)
		data, contentType, err := aggregator.Render(output, result)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "订阅生成失败")
			return
		}
		now := time.Now()
		_ = s.store.Update(func(cfg *domain.Config) error {
			for i := range cfg.Outputs {
				if cfg.Outputs[i].ID == output.ID {
					cfg.Outputs[i].LastGeneratedAt = &now
					cfg.Outputs[i].LastNodeCount = len(result.Nodes)
					cfg.Outputs[i].LastDroppedCount = result.DuplicateCount + result.FilteredCount + result.UnavailableCount
				}
			}
			return nil
		})
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Query().Get("download") == "1" {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", subscriptionFilename(output.Slug, output.Format)))
		}
		_, _ = w.Write(data)
		return
	}
	writeError(w, http.StatusNotFound, "订阅地址不存在")
}

func normalizeOutputFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "clash", "mihomo":
		return "clash"
	case "base64":
		return "base64"
	default:
		return ""
	}
}

func subscriptionFilename(slug string, format string) string {
	name := normalizeSlug(slug)
	if name == "" {
		name = "subscription"
	}
	if strings.EqualFold(format, "base64") {
		return name + ".txt"
	}
	return name + ".yaml"
}

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"sub-nest-backup.json\"")
	_ = json.NewEncoder(w).Encode(cfg)
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	var cfg domain.Config
	if err := decodeJSON(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "备份文件格式错误")
		return
	}
	if err := s.store.Replace(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "恢复失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) refreshSource(id string) (domain.Source, error) {
	cfg := s.store.Snapshot()
	var source domain.Source
	found := false
	for _, item := range cfg.Sources {
		if item.ID == id {
			source = item
			found = true
			break
		}
	}
	if !found {
		return domain.Source{}, errors.New("not found")
	}
	refreshed, refreshErr := s.fetcher.RefreshWithProgress(source, func(status string, message string, percent int, nodes []domain.Node) {
		_ = s.store.Update(func(cfg *domain.Config) error {
			for i := range cfg.Sources {
				if cfg.Sources[i].ID == id {
					cfg.Sources[i].LastStatus = status
					cfg.Sources[i].RefreshProgress = message
					cfg.Sources[i].RefreshPercent = percent
					if nodes != nil {
						cfg.Sources[i].CachedNodes = nodes
						cfg.Sources[i].LastNodeCount = len(nodes)
					}
					now := time.Now()
					cfg.Sources[i].LastRefreshedAt = &now
					return nil
				}
			}
			return errors.New("not found")
		})
	})
	if refreshErr != nil {
		s.logger.Warn("refresh source failed", "source", source.Name, "error", refreshErr)
	}
	err := s.store.Update(func(cfg *domain.Config) error {
		for i := range cfg.Sources {
			if cfg.Sources[i].ID == id {
				if refreshed.CachedNodes == nil && len(cfg.Sources[i].CachedNodes) > 0 {
					refreshed.CachedNodes = cfg.Sources[i].CachedNodes
				}
				cfg.Sources[i] = refreshed
				return nil
			}
		}
		return errors.New("not found")
	})
	return refreshed, err
}

func (s *Server) startRefreshSource(id string) (domain.Source, error) {
	cfg := s.store.Snapshot()
	for _, source := range cfg.Sources {
		if source.ID == id {
			if source.LastStatus == "refreshing" {
				return source, nil
			}
			now := time.Now()
			source.LastStatus = "refreshing"
			source.RefreshProgress = "等待刷新任务启动"
			source.RefreshPercent = 1
			source.LastRefreshedAt = &now
			err := s.store.Update(func(cfg *domain.Config) error {
				for i := range cfg.Sources {
					if cfg.Sources[i].ID == id {
						cfg.Sources[i].LastStatus = source.LastStatus
						cfg.Sources[i].RefreshProgress = source.RefreshProgress
						cfg.Sources[i].RefreshPercent = source.RefreshPercent
						cfg.Sources[i].LastRefreshedAt = source.LastRefreshedAt
						return nil
					}
				}
				return errors.New("not found")
			})
			if err != nil {
				return domain.Source{}, err
			}
			go func() {
				if _, err := s.refreshSource(id); err != nil {
					s.logger.Warn("refresh source task failed", "source_id", id, "error", err)
				}
			}()
			return source, nil
		}
	}
	return domain.Source{}, errors.New("not found")
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := s.store.Snapshot()
		if cfg.Settings.AdminTokenHash == "" {
			writeError(w, http.StatusPreconditionRequired, "需要先初始化管理员 token")
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		raw, ok := strings.CutPrefix(token, "local:")
		if !ok || bcrypt.CompareHashAndPassword([]byte(cfg.Settings.AdminTokenHash), []byte(raw)) != nil {
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) needsSetup() bool {
	return s.store.Snapshot().Settings.AdminTokenHash == ""
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.ServeFile(w, r, filepath.Join(s.staticPath, "index.html"))
		return
	}
	path := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	full := filepath.Join(s.staticPath, path)
	if !strings.HasPrefix(full, filepath.Clean(s.staticPath)) {
		http.NotFound(w, r)
		return
	}
	if _, err := os.Stat(full); err == nil {
		http.ServeFile(w, r, full)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.staticPath, "index.html"))
}

func issueSessionToken(raw string) string {
	return "local:" + raw
}

func hashToken(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 8 {
		return nil, errors.New("token 至少需要 8 位")
	}
	return bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
}

func subscriptionTokenFromRequest(r *http.Request) string {
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		return token
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return strings.TrimSpace(token)
}

func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 4*1024*1024))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func normalizeSlug(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	replacer := strings.NewReplacer(" ", "-", "_", "-", "/", "-", "\\", "-")
	slug = replacer.Replace(slug)
	var b strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func originFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
		host = forwarded
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func RandomToken() string {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return uuid.NewString()
	}
	return hex.EncodeToString(buf)
}
