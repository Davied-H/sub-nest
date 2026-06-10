package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

type principal struct {
	User domain.User
}

type contextKey string

const principalContextKey contextKey = "principal"

func trafficQueryMinutes(settings domain.Settings) int {
	if settings.TrafficQueryMinutes <= 0 {
		return 5
	}
	return settings.TrafficQueryMinutes
}

func refreshMinutes(settings domain.Settings) int {
	if settings.RefreshMinutes <= 0 {
		return 60
	}
	return settings.RefreshMinutes
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
	mux.HandleFunc("POST /api/register", s.handleRegister)
	mux.HandleFunc("GET /s/{slug}", s.handlePublicSubscription)
	mux.HandleFunc("GET /u/{userSlug}/s/{slug}", s.handleUserPublicSubscription)

	mux.Handle("GET /api/me", s.auth(http.HandlerFunc(s.handleMe)))
	mux.Handle("POST /api/admin/invite-codes", s.authAdmin(http.HandlerFunc(s.handleCreateInviteCode)))
	mux.Handle("GET /api/admin/invite-codes", s.authAdmin(http.HandlerFunc(s.handleInviteCodes)))
	mux.Handle("DELETE /api/admin/invite-codes/{id}", s.authAdmin(http.HandlerFunc(s.handleDeleteInviteCode)))
	mux.Handle("GET /api/admin/users", s.authAdmin(http.HandlerFunc(s.handleUsers)))
	mux.Handle("PUT /api/admin/users/{id}", s.authAdmin(http.HandlerFunc(s.handleUpdateUser)))

	mux.Handle("GET /api/dashboard", s.auth(http.HandlerFunc(s.handleDashboard)))
	mux.Handle("GET /api/settings", s.auth(http.HandlerFunc(s.handleSettings)))
	mux.Handle("PUT /api/settings/refresh", s.authAdmin(http.HandlerFunc(s.handleUpdateRefreshSettings)))
	mux.Handle("PUT /api/settings/traffic-query", s.authAdmin(http.HandlerFunc(s.handleUpdateTrafficQuerySettings)))
	mux.Handle("PUT /api/settings/admin-token", s.authAdmin(http.HandlerFunc(s.handleUpdateAdminToken)))
	mux.Handle("PUT /api/settings/user-token", s.authAdmin(http.HandlerFunc(s.handleUpdateUserToken)))
	mux.Handle("GET /api/sources", s.auth(http.HandlerFunc(s.handleSources)))
	mux.Handle("POST /api/sources", s.auth(http.HandlerFunc(s.handleCreateSource)))
	mux.Handle("PUT /api/sources/{id}", s.auth(http.HandlerFunc(s.handleUpdateSource)))
	mux.Handle("DELETE /api/sources/{id}", s.auth(http.HandlerFunc(s.handleDeleteSource)))
	mux.Handle("POST /api/sources/{id}/refresh", s.auth(http.HandlerFunc(s.handleRefreshSource)))
	mux.Handle("POST /api/sources/{id}/traffic-query", s.auth(http.HandlerFunc(s.handleQuerySourceTraffic)))
	mux.Handle("POST /api/traffic-query/test", s.auth(http.HandlerFunc(s.handleTestTrafficQuery)))
	mux.Handle("POST /api/refresh", s.auth(http.HandlerFunc(s.handleRefreshAll)))

	mux.Handle("GET /api/rule-sources", s.auth(http.HandlerFunc(s.handleRuleSources)))
	mux.Handle("PUT /api/rule-sources", s.auth(http.HandlerFunc(s.handleUpdateRuleSources)))
	mux.Handle("GET /api/rule-sets", s.auth(http.HandlerFunc(s.handleRuleSets)))
	mux.Handle("PUT /api/rule-sets", s.auth(http.HandlerFunc(s.handleUpdateRuleSets)))
	mux.Handle("GET /api/rule-sets/{id}/domains", s.auth(http.HandlerFunc(s.handleRuleSetDomains)))
	mux.Handle("POST /api/rule-sets/{id}/sync", s.auth(http.HandlerFunc(s.handleSyncRuleSet)))

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

func (s *Server) StartBackgroundTasks(ctx context.Context) {
	go s.runRefreshLoop(ctx)
	go s.runTrafficQueryLoop(ctx)
}

func (s *Server) runRefreshLoop(ctx context.Context) {
	s.refreshDueSources()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshDueSources()
		}
	}
}

func (s *Server) runTrafficQueryLoop(ctx context.Context) {
	s.queryDueTrafficSources()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.queryDueTrafficSources()
		}
	}
}

func (s *Server) refreshDueSources() {
	cfg := s.store.Snapshot()
	interval := time.Duration(refreshMinutes(cfg.Settings)) * time.Minute
	now := time.Now()
	for _, source := range cfg.Sources {
		if !source.Enabled || source.LastStatus == "refreshing" {
			continue
		}
		if source.LastRefreshedAt != nil && now.Sub(*source.LastRefreshedAt) < interval {
			continue
		}
		if _, err := s.startRefreshSource(source.OwnerUserID, source.ID); err != nil {
			s.logger.Warn("scheduled source refresh failed", "source", source.Name, "error", err)
		}
	}
}

func (s *Server) queryDueTrafficSources() {
	cfg := s.store.Snapshot()
	interval := time.Duration(trafficQueryMinutes(cfg.Settings)) * time.Minute
	now := time.Now()
	for _, source := range cfg.Sources {
		if !source.Enabled || source.TrafficQuery.Mode == "" || source.TrafficQuery.Mode == "disabled" {
			continue
		}
		if source.TrafficInfo.LastStatus == "checking" {
			continue
		}
		if source.TrafficInfo.LastCheckedAt != nil && now.Sub(*source.TrafficInfo.LastCheckedAt) < interval {
			continue
		}
		if _, _, err := s.querySourceTraffic(source.OwnerUserID, source.ID, nil); err != nil {
			s.logger.Warn("scheduled traffic query failed", "source", source.Name, "error", err)
		}
	}
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
		ensureAdminUser(cfg)
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "配置保存失败")
		return
	}
	writeJSON(w, http.StatusOK, authResponse(issueSessionToken("admin", token), adminUser()))
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
	if bcrypt.CompareHashAndPassword([]byte(cfg.Settings.AdminTokenHash), []byte(token)) == nil {
		now := time.Now()
		_ = s.store.Update(func(cfg *domain.Config) error {
			ensureAdminUser(cfg)
			for i := range cfg.Users {
				if cfg.Users[i].ID == "admin" {
					cfg.Users[i].LastLoginAt = &now
					return nil
				}
			}
			return nil
		})
		user := findUser(s.store.Snapshot(), "admin")
		writeJSON(w, http.StatusOK, authResponse(issueSessionToken("admin", token), user))
		return
	}
	for _, user := range cfg.Users {
		if user.ID == "admin" || !user.Enabled || user.TokenHash == "" {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(user.TokenHash), []byte(token)) != nil {
			continue
		}
		now := time.Now()
		_ = s.store.Update(func(cfg *domain.Config) error {
			for i := range cfg.Users {
				if cfg.Users[i].ID == user.ID {
					cfg.Users[i].LastLoginAt = &now
					return nil
				}
			}
			return nil
		})
		user.LastLoginAt = &now
		writeJSON(w, http.StatusOK, authResponse(issueSessionToken(user.ID, token), user))
		return
	}
	writeError(w, http.StatusUnauthorized, "token 不正确")
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, settingsView(s.store.Snapshot()))
}

func (s *Server) handleUpdateRefreshSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Minutes int `json:"minutes"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Minutes < 1 || req.Minutes > 1440 {
		writeError(w, http.StatusBadRequest, "自动刷新间隔需要在 1 到 1440 分钟之间")
		return
	}
	err := s.store.Update(func(cfg *domain.Config) error {
		cfg.Settings.RefreshMinutes = req.Minutes
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "自动刷新设置保存失败")
		return
	}
	writeJSON(w, http.StatusOK, settingsView(s.store.Snapshot()))
}

func (s *Server) handleUpdateTrafficQuerySettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Minutes int `json:"minutes"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Minutes < 1 || req.Minutes > 1440 {
		writeError(w, http.StatusBadRequest, "流量查询间隔需要在 1 到 1440 分钟之间")
		return
	}
	err := s.store.Update(func(cfg *domain.Config) error {
		cfg.Settings.TrafficQueryMinutes = req.Minutes
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "流量查询设置保存失败")
		return
	}
	cfg := s.store.Snapshot()
	writeJSON(w, http.StatusOK, settingsView(cfg))
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
	writeJSON(w, http.StatusOK, authResponse(issueSessionToken("admin", newToken), findUser(s.store.Snapshot(), "admin")))
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
	writeJSON(w, http.StatusOK, settingsView(s.store.Snapshot()))
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InviteCode string `json:"inviteCode"`
		UserSlug   string `json:"userSlug"`
		Token      string `json:"token"`
		Name       string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.UserSlug = normalizeSlug(req.UserSlug)
	req.Name = strings.TrimSpace(req.Name)
	req.InviteCode = strings.TrimSpace(req.InviteCode)
	if req.UserSlug == "" || req.UserSlug == "admin" {
		writeError(w, http.StatusBadRequest, "用户标识格式不正确")
		return
	}
	if len(req.Token) < 8 {
		writeError(w, http.StatusBadRequest, "token 至少需要 8 位")
		return
	}
	tokenHash, err := bcrypt.GenerateFromPassword([]byte(req.Token), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token 保存失败")
		return
	}
	var created domain.User
	err = s.store.Update(func(cfg *domain.Config) error {
		if bcrypt.CompareHashAndPassword([]byte(cfg.Settings.AdminTokenHash), []byte(req.Token)) == nil {
			return errors.New("token exists")
		}
		for _, user := range cfg.Users {
			if user.Slug == req.UserSlug {
				return errors.New("slug exists")
			}
			if user.TokenHash != "" && bcrypt.CompareHashAndPassword([]byte(user.TokenHash), []byte(req.Token)) == nil {
				return errors.New("token exists")
			}
		}
		inviteIndex := -1
		for i := range cfg.InviteCodes {
			if cfg.InviteCodes[i].UsedAt != nil {
				continue
			}
			if bcrypt.CompareHashAndPassword([]byte(cfg.InviteCodes[i].CodeHash), []byte(req.InviteCode)) == nil {
				inviteIndex = i
				break
			}
		}
		if inviteIndex < 0 {
			return errors.New("invalid invite")
		}
		now := time.Now()
		name := req.Name
		if name == "" {
			name = req.UserSlug
		}
		created = domain.User{
			ID:          uuid.NewString(),
			Slug:        req.UserSlug,
			Name:        name,
			TokenHash:   string(tokenHash),
			Role:        "user",
			Enabled:     true,
			CreatedAt:   now,
			LastLoginAt: &now,
		}
		cfg.Users = append(cfg.Users, created)
		cfg.InviteCodes[inviteIndex].UsedAt = &now
		cfg.InviteCodes[inviteIndex].UsedByUserID = created.ID
		return nil
	})
	if err != nil {
		switch err.Error() {
		case "slug exists":
			writeError(w, http.StatusConflict, "用户标识已存在")
		case "invalid invite":
			writeError(w, http.StatusUnauthorized, "授权码无效或已使用")
		case "token exists":
			writeError(w, http.StatusConflict, "token 已被使用，请换一个")
		default:
			writeError(w, http.StatusInternalServerError, "注册失败")
		}
		return
	}
	writeJSON(w, http.StatusCreated, authResponse(issueSessionToken(created.ID, req.Token), created))
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"user": publicUser(currentPrincipal(r).User)})
}

func (s *Server) handleCreateInviteCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label string `json:"label"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	code := RandomToken()
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "授权码生成失败")
		return
	}
	invite := domain.InviteCode{
		ID:               uuid.NewString(),
		CodeHash:         string(hash),
		Label:            strings.TrimSpace(req.Label),
		CreatedAt:        time.Now(),
		CreatedByAdminID: currentPrincipal(r).User.ID,
	}
	err = s.store.Update(func(cfg *domain.Config) error {
		cfg.InviteCodes = append(cfg.InviteCodes, invite)
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "授权码保存失败")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":        invite.ID,
		"code":      code,
		"label":     invite.Label,
		"createdAt": invite.CreatedAt,
	})
}

func (s *Server) handleInviteCodes(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Snapshot()
	items := make([]map[string]interface{}, 0, len(cfg.InviteCodes))
	for _, invite := range cfg.InviteCodes {
		items = append(items, map[string]interface{}{
			"id":           invite.ID,
			"label":        invite.Label,
			"createdAt":    invite.CreatedAt,
			"usedAt":       invite.UsedAt,
			"usedByUserId": invite.UsedByUserID,
		})
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDeleteInviteCode(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "授权码 ID 不能为空")
		return
	}
	deleted := false
	err := s.store.Update(func(cfg *domain.Config) error {
		next := cfg.InviteCodes[:0]
		for _, invite := range cfg.InviteCodes {
			if invite.ID == id {
				deleted = true
				continue
			}
			next = append(next, invite)
		}
		cfg.InviteCodes = next
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "授权码删除失败")
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "授权码不存在")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Snapshot()
	users := make([]map[string]interface{}, 0, len(cfg.Users))
	for _, user := range cfg.Users {
		users = append(users, publicUser(user))
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "admin" {
		writeError(w, http.StatusBadRequest, "不能禁用 admin")
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	var updated domain.User
	err := s.store.Update(func(cfg *domain.Config) error {
		for i := range cfg.Users {
			if cfg.Users[i].ID == id {
				cfg.Users[i].Enabled = req.Enabled
				updated = cfg.Users[i]
				return nil
			}
		}
		return errors.New("not found")
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "用户不存在")
		return
	}
	writeJSON(w, http.StatusOK, publicUser(updated))
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ownerID, ownerSlug, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "token 不正确")
		return
	}
	cfg := s.store.Snapshot()
	dashboard := domain.Dashboard{
		NeedsAdminSetup: cfg.Settings.AdminTokenHash == "",
	}
	for _, source := range cfg.Sources {
		if source.OwnerUserID != ownerID {
			continue
		}
		dashboard.SourceCount++
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
		if output.OwnerUserID != ownerID {
			continue
		}
		dashboard.OutputCount++
		if output.Enabled {
			dashboard.EnabledOutputs++
		}
	}
	base := originFromRequest(r)
	if ownerSlug == "admin" {
		dashboard.PublicExampleURL = strings.TrimRight(base, "/") + "/s/main"
	} else {
		dashboard.PublicExampleURL = strings.TrimRight(base, "/") + "/u/" + ownerSlug + "/s/main"
	}
	writeJSON(w, http.StatusOK, dashboard)
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	cfg := s.store.Snapshot()
	includeURL := r.URL.Query().Get("includeUrl") == "1"
	views := make([]domain.SourceView, 0, len(cfg.Sources))
	for _, source := range cfg.Sources {
		if source.OwnerUserID != ownerID {
			continue
		}
		views = append(views, domain.SourceToView(source, includeURL))
	}
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	var req domain.Source
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.ID = uuid.NewString()
	req.OwnerUserID = ownerID
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
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
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
			if cfg.Sources[i].ID == id && cfg.Sources[i].OwnerUserID == ownerID {
				cfg.Sources[i].Name = req.Name
				cfg.Sources[i].URL = req.URL
				cfg.Sources[i].SourceType = req.SourceType
				cfg.Sources[i].FileName = req.FileName
				cfg.Sources[i].FileContent = req.FileContent
				cfg.Sources[i].Enabled = req.Enabled
				cfg.Sources[i].Remark = req.Remark
				cfg.Sources[i].Tags = req.Tags
				cfg.Sources[i].TrafficQuery = req.TrafficQuery
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
	source.TrafficQuery.Mode = strings.TrimSpace(source.TrafficQuery.Mode)
	source.TrafficQuery.URL = strings.TrimSpace(source.TrafficQuery.URL)
	source.TrafficQuery.Method = strings.ToUpper(strings.TrimSpace(source.TrafficQuery.Method))
	if source.TrafficQuery.Method == "" {
		source.TrafficQuery.Method = http.MethodGet
	}
	source.TrafficQuery.Parser.Type = strings.TrimSpace(source.TrafficQuery.Parser.Type)
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
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	id := r.PathValue("id")
	err := s.store.Update(func(cfg *domain.Config) error {
		next := cfg.Sources[:0]
		found := false
		for _, source := range cfg.Sources {
			if source.ID == id && source.OwnerUserID == ownerID {
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
			if cfg.Outputs[i].OwnerUserID == ownerID {
				cfg.Outputs[i].SourceIDs = removeString(cfg.Outputs[i].SourceIDs, id)
			}
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
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	id := r.PathValue("id")
	source, err := s.startRefreshSource(ownerID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "订阅源不存在")
		return
	}
	writeJSON(w, http.StatusAccepted, domain.SourceToView(source, false))
}

func (s *Server) handleQuerySourceTraffic(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	var req struct {
		Source *domain.Source `json:"source"`
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4*1024*1024))
	_ = r.Body.Close()
	if err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "请求格式错误")
			return
		}
	}
	source, info, err := s.querySourceTraffic(ownerID, r.PathValue("id"), req.Source)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	source.TrafficInfo = info
	writeJSON(w, http.StatusOK, domain.SourceToView(source, false))
}

func (s *Server) handleTestTrafficQuery(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	var req struct {
		Source domain.Source `json:"source"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	source := req.Source
	source.OwnerUserID = ownerID
	normalizeSourceInput(&source)
	info, _ := s.fetcher.QueryTraffic(source)
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleRefreshAll(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	cfg := s.store.Snapshot()
	views := []domain.SourceView{}
	for _, source := range cfg.Sources {
		if source.OwnerUserID != ownerID || !source.Enabled {
			continue
		}
		refreshed, err := s.startRefreshSource(ownerID, source.ID)
		if err != nil {
			continue
		}
		views = append(views, domain.SourceToView(refreshed, false))
	}
	writeJSON(w, http.StatusAccepted, views)
}

func (s *Server) handleRuleSources(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	cfg := s.store.Snapshot()
	out := []domain.RuleSource{}
	for _, source := range cfg.RuleSources {
		if source.OwnerUserID == ownerID {
			out = append(out, publicRuleSource(source))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleUpdateRuleSources(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	var req []domain.RuleSource
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	nextSources := make([]domain.RuleSource, 0, len(req))
	seen := map[string]bool{}
	cfg := s.store.Snapshot()
	existingSources := map[string]domain.RuleSource{}
	for _, source := range cfg.RuleSources {
		if source.OwnerUserID == ownerID {
			existingSources[source.ID] = source
		}
	}
	for _, source := range req {
		source.OwnerUserID = ownerID
		source = domain.NormalizeRuleSource(source)
		if source.ID == "" || source.Name == "" {
			writeError(w, http.StatusBadRequest, "规则源 ID 和名称不能为空")
			return
		}
		if seen[source.ID] {
			writeError(w, http.StatusBadRequest, "规则源 ID 不能重复")
			return
		}
		if existing, ok := existingSources[source.ID]; ok && len(source.CachedDomainSuffixes) == 0 && sameRuleSourceCacheKey(existing, source) {
			source.CachedDomainSuffixes = existing.CachedDomainSuffixes
			source.LastSyncedAt = existing.LastSyncedAt
			source.LastSyncStatus = existing.LastSyncStatus
			source.LastSyncError = existing.LastSyncError
		}
		seen[source.ID] = true
		nextSources = append(nextSources, source)
	}
	err := s.store.Update(func(cfg *domain.Config) error {
		next := cfg.RuleSources[:0]
		for _, source := range cfg.RuleSources {
			if source.OwnerUserID != ownerID {
				next = append(next, source)
			}
		}
		cfg.RuleSources = append(next, nextSources...)
		for _, set := range cfg.RuleSets {
			if set.OwnerUserID == ownerID {
				updateRuleSetCache(cfg, ownerID, set.ID)
			}
		}
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "规则源保存失败")
		return
	}
	s.handleRuleSources(w, r)
}

func (s *Server) handleRuleSets(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	cfg := s.store.Snapshot()
	out := []domain.RuleSet{}
	for _, set := range cfg.RuleSets {
		if set.OwnerUserID == ownerID {
			out = append(out, publicRuleSet(set))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRuleSetDomains(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	setID := strings.TrimSpace(r.PathValue("id"))
	if setID == "" {
		writeError(w, http.StatusBadRequest, "规则集 ID 不能为空")
		return
	}
	cfg := s.store.Snapshot()
	var ruleSet domain.RuleSet
	found := false
	for _, set := range cfg.RuleSets {
		if set.OwnerUserID == ownerID && set.ID == setID {
			ruleSet = domain.NormalizeRuleSet(set)
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "规则集不存在")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := ruleDomainsLimit(r.URL.Query().Get("limit"))
	rows := ruleDomainsForSet(cfg, ownerID, ruleSet)
	matchedRows := make([]domain.RuleDomain, 0, len(rows))
	for _, row := range rows {
		if matchesRuleDomainQuery(row, query) {
			matchedRows = append(matchedRows, row)
		}
	}
	matched := len(matchedRows)
	truncated := false
	if len(matchedRows) > limit {
		matchedRows = matchedRows[:limit]
		truncated = true
	}
	writeJSON(w, http.StatusOK, domain.RuleDomainsView{
		RuleSetID:   ruleSet.ID,
		RuleSetName: ruleSet.Name,
		Query:       query,
		Total:       len(rows),
		Matched:     matched,
		Limit:       limit,
		Truncated:   truncated,
		Domains:     matchedRows,
	})
}

func (s *Server) handleUpdateRuleSets(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	var req []domain.RuleSet
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	cfg := s.store.Snapshot()
	allowedSources := map[string]bool{}
	for _, source := range cfg.RuleSources {
		if source.OwnerUserID == ownerID {
			allowedSources[source.ID] = true
		}
	}
	nextSets := make([]domain.RuleSet, 0, len(req))
	seen := map[string]bool{}
	existingSets := map[string]domain.RuleSet{}
	for _, set := range cfg.RuleSets {
		if set.OwnerUserID == ownerID {
			existingSets[set.ID] = set
		}
	}
	for _, set := range req {
		set.OwnerUserID = ownerID
		set = domain.NormalizeRuleSet(set)
		if set.ID == "" || set.Name == "" {
			writeError(w, http.StatusBadRequest, "规则集 ID 和名称不能为空")
			return
		}
		if seen[set.ID] {
			writeError(w, http.StatusBadRequest, "规则集 ID 不能重复")
			return
		}
		set.SourceIDs = existingRuleSourceIDs(allowedSources, set.SourceIDs)
		if existing, ok := existingSets[set.ID]; ok && len(set.CachedDomainSuffixes) == 0 {
			set.CachedDomainSuffixes = existing.CachedDomainSuffixes
			set.LastSyncedAt = existing.LastSyncedAt
			set.LastSyncStatus = existing.LastSyncStatus
			set.LastSyncError = existing.LastSyncError
		}
		seen[set.ID] = true
		nextSets = append(nextSets, set)
	}
	err := s.store.Update(func(cfg *domain.Config) error {
		next := cfg.RuleSets[:0]
		for _, set := range cfg.RuleSets {
			if set.OwnerUserID != ownerID {
				next = append(next, set)
			}
		}
		cfg.RuleSets = append(next, nextSets...)
		for _, set := range nextSets {
			updateRuleSetCache(cfg, ownerID, set.ID)
		}
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "规则集保存失败")
		return
	}
	s.handleRuleSets(w, r)
}

func (s *Server) handleSyncRuleSet(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	setID := strings.TrimSpace(r.PathValue("id"))
	if setID == "" {
		writeError(w, http.StatusBadRequest, "规则集 ID 不能为空")
		return
	}
	cfg := s.store.Snapshot()
	found := false
	for _, set := range cfg.RuleSets {
		if set.OwnerUserID == ownerID && set.ID == setID {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "规则集不存在")
		return
	}
	s.syncRuleSet(r.Context(), ownerID, setID, true)
	s.handleRuleSets(w, r)
}

func (s *Server) handleOutputs(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	cfg := s.store.Snapshot()
	ownerConfig := configForOwner(cfg, ownerID)
	outputs := make([]domain.OutputView, 0, len(cfg.Outputs))
	for _, output := range cfg.Outputs {
		if output.OwnerUserID == ownerID {
			result := aggregator.Build(ownerConfig, output)
			output.LastNodeCount = len(result.Nodes)
			output.LastDroppedCount = result.DuplicateCount + result.FilteredCount + result.UnavailableCount
			outputs = append(outputs, domain.OutputView{
				Output:    output,
				NodeNames: outputNodeNames(result.Nodes),
			})
		}
	}
	writeJSON(w, http.StatusOK, outputs)
}

func (s *Server) handleCreateOutput(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
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
	req.OwnerUserID = ownerID
	req.Slug = normalizeSlug(req.Slug)
	if req.Format == "" {
		req.Format = "clash"
	}
	if req.GroupMode == "" {
		req.GroupMode = "region"
	}
	req.PAC = domain.NormalizePACConfig(req.PAC)
	err := s.store.Update(func(cfg *domain.Config) error {
		for _, output := range cfg.Outputs {
			if output.OwnerUserID == ownerID && output.Slug == req.Slug {
				return errors.New("slug exists")
			}
		}
		req.SourceIDs = existingSourceIDsForOwner(*cfg, ownerID, req.SourceIDs)
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
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	id := r.PathValue("id")
	var req domain.Output
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	var updated domain.Output
	err := s.store.Update(func(cfg *domain.Config) error {
		for i := range cfg.Outputs {
			if cfg.Outputs[i].ID == id && cfg.Outputs[i].OwnerUserID == ownerID {
				req.ID = id
				req.OwnerUserID = ownerID
				req.Slug = normalizeSlug(req.Slug)
				if req.Slug == "" {
					return errors.New("empty slug")
				}
				for _, output := range cfg.Outputs {
					if output.OwnerUserID == ownerID && output.ID != id && output.Slug == req.Slug {
						return errors.New("slug exists")
					}
				}
				req.SourceIDs = existingSourceIDsForOwner(*cfg, ownerID, req.SourceIDs)
				req.PAC = domain.NormalizePACConfig(req.PAC)
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
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	id := r.PathValue("id")
	err := s.store.Update(func(cfg *domain.Config) error {
		next := cfg.Outputs[:0]
		found := false
		for _, output := range cfg.Outputs {
			if output.ID == id && output.OwnerUserID == ownerID {
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
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	id := r.PathValue("id")
	cfg := s.store.Snapshot()
	for _, output := range cfg.Outputs {
		if output.ID == id && output.OwnerUserID == ownerID {
			writeJSON(w, http.StatusOK, aggregator.Preview(configForOwner(cfg, ownerID), output))
			return
		}
	}
	writeError(w, http.StatusNotFound, "公开订阅不存在")
}

func (s *Server) handlePublicSubscription(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if pacSlug, ok := slugWithoutPACSuffix(slug); ok {
		s.servePublicPAC(w, r, "admin", pacSlug)
		return
	}
	s.servePublicSubscription(w, r, "admin", slug)
}

func (s *Server) handleUserPublicSubscription(w http.ResponseWriter, r *http.Request) {
	userSlug := normalizeSlug(r.PathValue("userSlug"))
	slug := r.PathValue("slug")
	cfg := s.store.Snapshot()
	for _, user := range cfg.Users {
		if user.Slug == userSlug && user.Enabled {
			if pacSlug, ok := slugWithoutPACSuffix(slug); ok {
				s.servePublicPAC(w, r, user.ID, pacSlug)
				return
			}
			s.servePublicSubscription(w, r, user.ID, slug)
			return
		}
	}
	writeError(w, http.StatusNotFound, "用户不存在")
}

func (s *Server) servePublicSubscription(w http.ResponseWriter, r *http.Request, ownerID string, slug string) {
	slug = normalizeSlug(slug)
	cfg := s.store.Snapshot()
	if cfg.Settings.UserTokenHash != "" {
		token := subscriptionTokenFromRequest(r)
		if bcrypt.CompareHashAndPassword([]byte(cfg.Settings.UserTokenHash), []byte(token)) != nil {
			writeError(w, http.StatusUnauthorized, "订阅 token 不正确")
			return
		}
	}
	for _, output := range cfg.Outputs {
		if output.OwnerUserID != ownerID || output.Slug != slug {
			continue
		}
		if !output.Enabled {
			writeError(w, http.StatusForbidden, "订阅地址已暂停")
			return
		}
		if format := normalizeOutputFormat(r.URL.Query().Get("format")); format != "" {
			output.Format = format
		}
		result := aggregator.Build(configForOwner(cfg, ownerID), output)
		pacConfig := resolvePACConfig(configForOwner(cfg, ownerID), output)
		data, contentType, err := aggregator.Render(output, result, pacConfig)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "订阅生成失败")
			return
		}
		now := time.Now()
		_ = s.store.Update(func(cfg *domain.Config) error {
			for i := range cfg.Outputs {
				if cfg.Outputs[i].ID == output.ID && cfg.Outputs[i].OwnerUserID == ownerID {
					cfg.Outputs[i].LastGeneratedAt = &now
					cfg.Outputs[i].LastNodeCount = len(result.Nodes)
					cfg.Outputs[i].LastDroppedCount = result.DuplicateCount + result.FilteredCount + result.UnavailableCount
				}
			}
			return nil
		})
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store")
		if userInfo := outputSubscriptionUserInfo(configForOwner(cfg, ownerID), output); userInfo != "" {
			w.Header().Set("Subscription-Userinfo", userInfo)
		}
		if r.URL.Query().Get("download") == "1" {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", subscriptionFilename(output.Slug, output.Format)))
		}
		_, _ = w.Write(data)
		return
	}
	writeError(w, http.StatusNotFound, "订阅地址不存在")
}

func (s *Server) servePublicPAC(w http.ResponseWriter, r *http.Request, ownerID string, slug string) {
	slug = normalizeSlug(slug)
	s.syncPACRulesIfDue(r.Context(), ownerID, slug)
	cfg := s.store.Snapshot()
	if cfg.Settings.UserTokenHash != "" {
		token := subscriptionTokenFromRequest(r)
		if bcrypt.CompareHashAndPassword([]byte(cfg.Settings.UserTokenHash), []byte(token)) != nil {
			writeError(w, http.StatusUnauthorized, "订阅 token 不正确")
			return
		}
	}
	for _, output := range cfg.Outputs {
		if output.OwnerUserID != ownerID || output.Slug != slug {
			continue
		}
		if !output.Enabled {
			writeError(w, http.StatusForbidden, "订阅地址已暂停")
			return
		}
		result := aggregator.Build(configForOwner(cfg, ownerID), output)
		pacConfig := resolvePACConfig(configForOwner(cfg, ownerID), output)
		data, contentType, err := aggregator.RenderPAC(output, result, pacConfig)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "PAC 生成失败")
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Query().Get("download") == "1" {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", subscriptionFilename(output.Slug, "pac")))
		}
		_, _ = w.Write(data)
		return
	}
	writeError(w, http.StatusNotFound, "订阅地址不存在")
}

func (s *Server) syncPACRulesIfDue(ctx context.Context, ownerID string, slug string) {
	cfg := s.store.Snapshot()
	var output domain.Output
	var found bool
	for _, item := range cfg.Outputs {
		if item.OwnerUserID == ownerID && item.Slug == slug {
			output = item
			found = true
			break
		}
	}
	if !found {
		return
	}
	pac := domain.NormalizePACConfig(output.PAC)
	if !pac.Enabled {
		return
	}
	if strings.TrimSpace(output.PAC.RuleSourceURL) != "" && output.PAC.RuleSourceURL != domain.DefaultPACRuleSourceURL {
		s.syncLegacyOutputPACRules(ctx, ownerID, slug, pac)
		return
	}
	ruleSet := findRuleSetForOutput(cfg, ownerID, output)
	s.syncRuleSet(ctx, ownerID, ruleSet.ID, false)
}

func (s *Server) syncRuleSet(ctx context.Context, ownerID string, setID string, force bool) {
	cfg := s.store.Snapshot()
	var ruleSet domain.RuleSet
	var found bool
	for _, set := range cfg.RuleSets {
		if set.OwnerUserID == ownerID && set.ID == setID {
			ruleSet = domain.NormalizeRuleSet(set)
			found = true
			break
		}
	}
	if !found {
		return
	}
	for _, source := range ruleSourcesForSet(cfg, ownerID, ruleSet) {
		source = domain.NormalizeRuleSource(source)
		if !force && !aggregator.RuleSourceSyncDue(source, time.Now()) {
			continue
		}
		syncCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		domains, err := aggregator.LoadRuleSource(source)
		if err != nil || len(domains) == 0 || domain.RuleSourceCacheExpired(source, time.Now()) {
			fetched, fetchErr := aggregator.FetchRuleSource(syncCtx, source)
			if fetchErr == nil {
				domains = fetched
				err = nil
			} else if err == nil {
				err = fetchErr
			}
		}
		cancel()
		now := time.Now()
		_ = s.store.Update(func(cfg *domain.Config) error {
			for i := range cfg.RuleSources {
				if cfg.RuleSources[i].OwnerUserID != ownerID || cfg.RuleSources[i].ID != source.ID {
					continue
				}
				next := domain.NormalizeRuleSource(cfg.RuleSources[i])
				next.LastSyncedAt = &now
				if err != nil {
					next.LastSyncStatus = "error"
					next.LastSyncError = err.Error()
				} else {
					next.CachedDomainSuffixes = domains
					next.LastSyncStatus = "ok"
					next.LastSyncError = ""
				}
				cfg.RuleSources[i] = next
				break
			}
			updateRuleSetCache(cfg, ownerID, ruleSet.ID)
			return nil
		})
		if err != nil {
			s.logger.Warn("sync pac rule source failed", "ruleSet", ruleSet.Name, "source", source.Name, "error", err)
			continue
		}
		s.logger.Info("sync pac rule source ok", "ruleSet", ruleSet.Name, "source", source.Name, "domains", len(domains))
	}
	_ = s.store.Update(func(cfg *domain.Config) error {
		updateRuleSetCache(cfg, ownerID, ruleSet.ID)
		return nil
	})
}

func (s *Server) syncLegacyOutputPACRules(ctx context.Context, ownerID string, slug string, pac domain.PACConfig) {
	if !aggregator.PACSyncDue(pac, time.Now()) {
		return
	}
	syncCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	domains, err := aggregator.FetchPACRuleSource(syncCtx, pac)
	now := time.Now()
	_ = s.store.Update(func(cfg *domain.Config) error {
		for i := range cfg.Outputs {
			if cfg.Outputs[i].OwnerUserID != ownerID || cfg.Outputs[i].Slug != slug {
				continue
			}
			next := domain.NormalizePACConfig(cfg.Outputs[i].PAC)
			next.LastSyncedAt = &now
			if err != nil {
				next.LastSyncStatus = "error"
				next.LastSyncError = err.Error()
			} else {
				next.CachedDomainSuffixes = domains
				next.LastSyncStatus = "ok"
				next.LastSyncError = ""
			}
			cfg.Outputs[i].PAC = next
			return nil
		}
		return nil
	})
	if err != nil {
		s.logger.Warn("sync legacy pac rules failed", "slug", slug, "error", err)
		return
	}
	s.logger.Info("sync legacy pac rules ok", "slug", slug, "domains", len(domains))
}

func outputSubscriptionUserInfo(cfg domain.Config, output domain.Output) string {
	sourceIDs := map[string]bool{}
	for _, id := range output.SourceIDs {
		sourceIDs[id] = true
	}
	var upload int64
	var download int64
	var total int64
	var expire *time.Time
	var found bool
	for _, source := range cfg.Sources {
		if !sourceIDs[source.ID] || !source.Enabled || source.TrafficInfo.LastStatus != "ok" {
			continue
		}
		info := source.TrafficInfo
		if info.UploadBytes > 0 || info.DownloadBytes > 0 || info.TotalBytes > 0 || info.ExpireAt != nil {
			found = true
		}
		upload += info.UploadBytes
		download += info.DownloadBytes
		total += info.TotalBytes
		if info.ExpireAt != nil && (expire == nil || info.ExpireAt.Before(*expire)) {
			t := *info.ExpireAt
			expire = &t
		}
	}
	if !found {
		return ""
	}
	parts := []string{
		fmt.Sprintf("upload=%d", upload),
		fmt.Sprintf("download=%d", download),
		fmt.Sprintf("total=%d", total),
	}
	if expire != nil {
		parts = append(parts, fmt.Sprintf("expire=%d", expire.Unix()))
	}
	return strings.Join(parts, "; ")
}

func normalizeOutputFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "clash", "mihomo":
		return "clash"
	case "base64":
		return "base64"
	case "surge":
		return "surge"
	default:
		return ""
	}
}

func subscriptionFilename(slug string, format string) string {
	name := normalizeSlug(slug)
	if name == "" {
		name = "subscription"
	}
	if strings.EqualFold(format, "pac") {
		return name + ".pac"
	}
	if strings.EqualFold(format, "base64") {
		return name + ".txt"
	}
	if strings.EqualFold(format, "surge") {
		return name + ".conf"
	}
	return name + ".yaml"
}

func slugWithoutPACSuffix(slug string) (string, bool) {
	trimmed := strings.TrimSpace(slug)
	if strings.HasSuffix(strings.ToLower(trimmed), ".pac") {
		return trimmed[:len(trimmed)-4], true
	}
	return trimmed, false
}

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	cfg := s.store.Snapshot()
	if p.User.Role != "admin" {
		cfg = configForOwner(cfg, p.User.ID)
		cfg.Users = []domain.User{p.User}
		cfg.InviteCodes = []domain.InviteCode{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"sub-nest-backup.json\"")
	_ = json.NewEncoder(w).Encode(cfg)
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	var incoming domain.Config
	if err := decodeJSON(r, &incoming); err != nil {
		writeError(w, http.StatusBadRequest, "备份文件格式错误")
		return
	}
	var err error
	if p.User.Role == "admin" {
		err = s.store.Replace(incoming)
	} else {
		err = s.store.Update(func(cfg *domain.Config) error {
			nextSources := cfg.Sources[:0]
			for _, source := range cfg.Sources {
				if source.OwnerUserID != p.User.ID {
					nextSources = append(nextSources, source)
				}
			}
			for _, source := range incoming.Sources {
				source.OwnerUserID = p.User.ID
				nextSources = append(nextSources, source)
			}
			nextRuleSources := cfg.RuleSources[:0]
			for _, source := range cfg.RuleSources {
				if source.OwnerUserID != p.User.ID {
					nextRuleSources = append(nextRuleSources, source)
				}
			}
			for _, source := range incoming.RuleSources {
				source.OwnerUserID = p.User.ID
				nextRuleSources = append(nextRuleSources, source)
			}
			nextRuleSets := cfg.RuleSets[:0]
			for _, set := range cfg.RuleSets {
				if set.OwnerUserID != p.User.ID {
					nextRuleSets = append(nextRuleSets, set)
				}
			}
			allowedRuleSources := map[string]bool{}
			for _, source := range nextRuleSources {
				if source.OwnerUserID == p.User.ID {
					allowedRuleSources[source.ID] = true
				}
			}
			for _, set := range incoming.RuleSets {
				set.OwnerUserID = p.User.ID
				set.SourceIDs = existingRuleSourceIDs(allowedRuleSources, set.SourceIDs)
				nextRuleSets = append(nextRuleSets, set)
			}
			nextOutputs := cfg.Outputs[:0]
			for _, output := range cfg.Outputs {
				if output.OwnerUserID != p.User.ID {
					nextOutputs = append(nextOutputs, output)
				}
			}
			for _, output := range incoming.Outputs {
				output.OwnerUserID = p.User.ID
				output.SourceIDs = existingSourceIDsForOwner(domain.Config{Sources: nextSources}, p.User.ID, output.SourceIDs)
				nextOutputs = append(nextOutputs, output)
			}
			cfg.Sources = nextSources
			cfg.RuleSources = nextRuleSources
			cfg.RuleSets = nextRuleSets
			cfg.Outputs = nextOutputs
			return nil
		})
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "恢复失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) refreshSource(ownerID string, id string) (domain.Source, error) {
	cfg := s.store.Snapshot()
	var source domain.Source
	found := false
	for _, item := range cfg.Sources {
		if item.ID == id && item.OwnerUserID == ownerID {
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
				if cfg.Sources[i].ID == id && cfg.Sources[i].OwnerUserID == ownerID {
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
	if refreshErr == nil && refreshed.TrafficQuery.Mode != "" && refreshed.TrafficQuery.Mode != "disabled" {
		info, err := s.fetcher.QueryTraffic(refreshed)
		refreshed.TrafficInfo = info
		if err != nil {
			s.logger.Warn("query source traffic failed", "source", source.Name, "error", err)
		}
	}
	err := s.store.Update(func(cfg *domain.Config) error {
		for i := range cfg.Sources {
			if cfg.Sources[i].ID == id && cfg.Sources[i].OwnerUserID == ownerID {
				if refreshed.CachedNodes == nil && len(cfg.Sources[i].CachedNodes) > 0 {
					refreshed.CachedNodes = cfg.Sources[i].CachedNodes
				}
				refreshed.OwnerUserID = ownerID
				cfg.Sources[i] = refreshed
				return nil
			}
		}
		return errors.New("not found")
	})
	return refreshed, err
}

func (s *Server) querySourceTraffic(ownerID string, id string, draft *domain.Source) (domain.Source, domain.TrafficInfo, error) {
	cfg := s.store.Snapshot()
	var source domain.Source
	found := false
	for _, item := range cfg.Sources {
		if item.ID == id && item.OwnerUserID == ownerID {
			source = item
			found = true
			break
		}
	}
	if !found {
		return domain.Source{}, domain.TrafficInfo{}, errors.New("订阅源不存在")
	}
	querySource := source
	if draft != nil {
		querySource.TrafficQuery = draft.TrafficQuery
		querySource.SourceType = draft.SourceType
		querySource.URL = draft.URL
		querySource.FileName = draft.FileName
		querySource.FileContent = draft.FileContent
		normalizeSourceInput(&querySource)
		querySource.ID = source.ID
		querySource.OwnerUserID = ownerID
		querySource.Name = source.Name
	}
	info, queryErr := s.fetcher.QueryTraffic(querySource)
	err := s.store.Update(func(cfg *domain.Config) error {
		for i := range cfg.Sources {
			if cfg.Sources[i].ID == id && cfg.Sources[i].OwnerUserID == ownerID {
				cfg.Sources[i].TrafficInfo = info
				source = cfg.Sources[i]
				return nil
			}
		}
		return errors.New("not found")
	})
	if err != nil {
		return domain.Source{}, info, err
	}
	if queryErr != nil {
		return source, info, nil
	}
	return source, info, nil
}

func (s *Server) startRefreshSource(ownerID string, id string) (domain.Source, error) {
	cfg := s.store.Snapshot()
	for _, source := range cfg.Sources {
		if source.ID == id && source.OwnerUserID == ownerID {
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
					if cfg.Sources[i].ID == id && cfg.Sources[i].OwnerUserID == ownerID {
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
				if _, err := s.refreshSource(ownerID, id); err != nil {
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
		p, err := s.authenticate(r)
		if err != nil {
			if errors.Is(err, errNeedsSetup) {
				writeError(w, http.StatusPreconditionRequired, "需要先初始化管理员 token")
				return
			}
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey, p)))
	})
}

func (s *Server) authAdmin(next http.Handler) http.Handler {
	return s.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentPrincipal(r).User.Role != "admin" {
			writeError(w, http.StatusForbidden, "需要管理员权限")
			return
		}
		next.ServeHTTP(w, r)
	}))
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

var errNeedsSetup = errors.New("needs setup")

func (s *Server) authenticate(r *http.Request) (principal, error) {
	cfg := s.store.Snapshot()
	if cfg.Settings.AdminTokenHash == "" {
		return principal{}, errNeedsSetup
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	raw, ok := strings.CutPrefix(token, "local:")
	if !ok {
		return principal{}, errors.New("invalid token")
	}
	userID, secret := parseSessionToken(raw)
	if userID == "" && bcrypt.CompareHashAndPassword([]byte(cfg.Settings.AdminTokenHash), []byte(secret)) == nil {
		return principal{User: findUser(cfg, "admin")}, nil
	}
	if userID == "admin" && bcrypt.CompareHashAndPassword([]byte(cfg.Settings.AdminTokenHash), []byte(secret)) == nil {
		return principal{User: findUser(cfg, "admin")}, nil
	}
	for _, user := range cfg.Users {
		if user.ID != userID || !user.Enabled || user.TokenHash == "" {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(user.TokenHash), []byte(secret)) == nil {
			return principal{User: user}, nil
		}
	}
	return principal{}, errors.New("invalid token")
}

func issueSessionToken(userID string, raw string) string {
	payload := userID + ":" + raw
	return "local:v2:" + base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func parseSessionToken(raw string) (string, string) {
	encoded, ok := strings.CutPrefix(raw, "v2:")
	if !ok {
		return "", raw
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", ""
	}
	userID, secret, ok := strings.Cut(string(data), ":")
	if !ok {
		return "", ""
	}
	return userID, secret
}

func currentPrincipal(r *http.Request) principal {
	if p, ok := r.Context().Value(principalContextKey).(principal); ok {
		return p
	}
	return principal{}
}

func (s *Server) resolveOwner(r *http.Request) (string, string, bool) {
	p := currentPrincipal(r)
	if p.User.ID == "" {
		return "", "", false
	}
	if p.User.Role != "admin" {
		return p.User.ID, p.User.Slug, p.User.Enabled
	}
	targetID := strings.TrimSpace(r.URL.Query().Get("userId"))
	if targetID == "" {
		targetID = p.User.ID
	}
	cfg := s.store.Snapshot()
	for _, user := range cfg.Users {
		if user.ID == targetID {
			return user.ID, user.Slug, true
		}
	}
	return "", "", false
}

func configForOwner(cfg domain.Config, ownerID string) domain.Config {
	out := cfg
	out.Sources = []domain.Source{}
	for _, source := range cfg.Sources {
		if source.OwnerUserID == ownerID {
			out.Sources = append(out.Sources, source)
		}
	}
	out.RuleSources = []domain.RuleSource{}
	for _, source := range cfg.RuleSources {
		if source.OwnerUserID == ownerID {
			out.RuleSources = append(out.RuleSources, source)
		}
	}
	out.RuleSets = []domain.RuleSet{}
	for _, set := range cfg.RuleSets {
		if set.OwnerUserID == ownerID {
			out.RuleSets = append(out.RuleSets, set)
		}
	}
	out.Outputs = []domain.Output{}
	for _, output := range cfg.Outputs {
		if output.OwnerUserID == ownerID {
			out.Outputs = append(out.Outputs, output)
		}
	}
	return out
}

func resolvePACConfig(cfg domain.Config, output domain.Output) domain.PACConfig {
	pac := domain.NormalizePACConfig(output.PAC)
	set := findRuleSetForOutput(cfg, output.OwnerUserID, output)
	pac.DomainKeywords = domain.NormalizeStringList(append(pac.DomainKeywords, set.DomainKeywords...))
	pac.DirectDomainSuffixes = domain.NormalizeStringList(append(pac.DirectDomainSuffixes, set.DirectDomainSuffixes...))
	pac.DirectCIDRs = domain.NormalizeStringList(append(pac.DirectCIDRs, set.DirectCIDRs...))
	pac.CachedDomainSuffixes = domain.NormalizeStringList(append(pac.CachedDomainSuffixes, cachedDomainsForRuleSet(cfg, output.OwnerUserID, set)...))
	pac.LastSyncedAt = set.LastSyncedAt
	pac.LastSyncStatus = set.LastSyncStatus
	pac.LastSyncError = set.LastSyncError
	return pac
}

func findRuleSetForOutput(cfg domain.Config, ownerID string, output domain.Output) domain.RuleSet {
	pac := domain.NormalizePACConfig(output.PAC)
	for _, set := range cfg.RuleSets {
		if set.OwnerUserID == ownerID && set.ID == pac.RuleSetID {
			return domain.NormalizeRuleSet(set)
		}
	}
	return domain.DefaultRuleSet(ownerID)
}

func ruleSourcesForSet(cfg domain.Config, ownerID string, set domain.RuleSet) []domain.RuleSource {
	ids := map[string]bool{}
	for _, id := range set.SourceIDs {
		ids[id] = true
	}
	out := []domain.RuleSource{}
	for _, source := range cfg.RuleSources {
		if source.OwnerUserID == ownerID && ids[source.ID] {
			out = append(out, source)
		}
	}
	return out
}

func cachedDomainsForRuleSet(cfg domain.Config, ownerID string, set domain.RuleSet) []string {
	domains := append([]string{}, set.CachedDomainSuffixes...)
	for _, source := range ruleSourcesForSet(cfg, ownerID, set) {
		domains = append(domains, source.CachedDomainSuffixes...)
	}
	excluded := map[string]bool{}
	for _, suffix := range domain.NormalizeStringList(set.ExcludedDomainSuffixes) {
		excluded[suffix] = true
	}
	out := []string{}
	for _, suffix := range domain.NormalizeStringList(domains) {
		if excluded[suffix] {
			continue
		}
		out = append(out, suffix)
	}
	return out
}

func ruleDomainsForSet(cfg domain.Config, ownerID string, set domain.RuleSet) []domain.RuleDomain {
	set = domain.NormalizeRuleSet(set)
	excluded := map[string]bool{}
	for _, suffix := range domain.NormalizeStringList(set.ExcludedDomainSuffixes) {
		excluded[strings.ToLower(suffix)] = true
	}
	seen := map[string]bool{}
	rows := []domain.RuleDomain{}
	appendRow := func(suffix string, source string, ruleType string) {
		suffix = strings.TrimSpace(suffix)
		if suffix == "" {
			return
		}
		key := strings.ToLower(suffix)
		if seen[key] {
			return
		}
		seen[key] = true
		rows = append(rows, domain.RuleDomain{
			Domain: suffix,
			Source: source,
			Type:   ruleType,
		})
	}
	for _, suffix := range domain.NormalizeStringList(set.DirectDomainSuffixes) {
		appendRow(suffix, "当前方案", "manual")
	}
	for _, keyword := range domain.NormalizeStringList(set.DomainKeywords) {
		appendRow(keyword, "当前方案", "keyword")
	}
	for _, source := range ruleSourcesForSet(cfg, ownerID, set) {
		source = domain.NormalizeRuleSource(source)
		for _, suffix := range domain.NormalizeStringList(source.CachedDomainSuffixes) {
			if excluded[strings.ToLower(suffix)] {
				continue
			}
			appendRow(suffix, source.Name, "cache")
		}
	}
	for _, suffix := range domain.NormalizeStringList(set.CachedDomainSuffixes) {
		if excluded[strings.ToLower(suffix)] {
			continue
		}
		appendRow(suffix, "方案缓存", "cache")
	}
	for _, suffix := range domain.NormalizeStringList(set.ExcludedDomainSuffixes) {
		appendRow(suffix, "当前方案", "excluded")
	}
	return rows
}

func matchesRuleDomainQuery(row domain.RuleDomain, query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return true
	}
	normalized := strings.NewReplacer(",", " ", "，", " ", "\n", " ").Replace(strings.ToLower(query))
	terms := strings.Fields(normalized)
	if len(terms) == 0 {
		return true
	}
	haystack := strings.ToLower(row.Domain + " " + row.Source + " " + row.Type)
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}

func ruleDomainsLimit(value string) int {
	const defaultLimit = 500
	const maxLimit = 2000
	limit, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func updateRuleSetCache(cfg *domain.Config, ownerID string, setID string) {
	var set domain.RuleSet
	var index = -1
	for i, item := range cfg.RuleSets {
		if item.OwnerUserID == ownerID && item.ID == setID {
			set = domain.NormalizeRuleSet(item)
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	var lastSynced *time.Time
	status := "ok"
	errors := []string{}
	domains := []string{}
	for _, source := range ruleSourcesForSet(*cfg, ownerID, set) {
		domains = append(domains, source.CachedDomainSuffixes...)
		if source.LastSyncedAt != nil && (lastSynced == nil || source.LastSyncedAt.After(*lastSynced)) {
			t := *source.LastSyncedAt
			lastSynced = &t
		}
		if source.LastSyncStatus == "error" {
			status = "error"
			if source.LastSyncError != "" {
				errors = append(errors, source.Name+": "+source.LastSyncError)
			}
		}
	}
	nextDomains := []string{}
	excluded := map[string]bool{}
	for _, suffix := range set.ExcludedDomainSuffixes {
		excluded[suffix] = true
	}
	for _, suffix := range domain.NormalizeStringList(domains) {
		if excluded[suffix] {
			continue
		}
		nextDomains = append(nextDomains, suffix)
	}
	set.CachedDomainSuffixes = nextDomains
	set.LastSyncedAt = lastSynced
	set.LastSyncStatus = status
	set.LastSyncError = strings.Join(errors, "；")
	cfg.RuleSets[index] = set
}

func outputNodeNames(nodes []domain.Node) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	return names
}

func existingSourceIDsForOwner(cfg domain.Config, ownerID string, ids []string) []string {
	allowed := map[string]bool{}
	for _, source := range cfg.Sources {
		if source.OwnerUserID == ownerID {
			allowed[source.ID] = true
		}
	}
	out := []string{}
	for _, id := range ids {
		if allowed[id] {
			out = append(out, id)
		}
	}
	return out
}

func existingRuleSourceIDs(allowed map[string]bool, ids []string) []string {
	out := []string{}
	for _, id := range ids {
		if allowed[id] {
			out = append(out, id)
		}
	}
	return out
}

func publicRuleSource(source domain.RuleSource) domain.RuleSource {
	source.CachedDomainCount = len(source.CachedDomainSuffixes)
	source.CachedDomainSuffixes = nil
	return source
}

func publicRuleSet(set domain.RuleSet) domain.RuleSet {
	set.CachedDomainCount = len(set.CachedDomainSuffixes)
	set.CachedDomainSuffixes = nil
	return set
}

func sameRuleSourceCacheKey(a domain.RuleSource, b domain.RuleSource) bool {
	a = domain.NormalizeRuleSource(a)
	b = domain.NormalizeRuleSource(b)
	return a.URL == b.URL && a.Format == b.Format && a.LocalPath == b.LocalPath
}

func authResponse(token string, user domain.User) map[string]interface{} {
	return map[string]interface{}{"token": token, "user": publicUser(user)}
}

func publicUser(user domain.User) map[string]interface{} {
	return map[string]interface{}{
		"id":          user.ID,
		"slug":        user.Slug,
		"name":        user.Name,
		"role":        user.Role,
		"enabled":     user.Enabled,
		"createdAt":   user.CreatedAt,
		"lastLoginAt": user.LastLoginAt,
	}
}

func settingsView(cfg domain.Config) domain.SettingsView {
	return domain.SettingsView{
		PublicBaseURL:       cfg.Settings.PublicBaseURL,
		HasUserToken:        cfg.Settings.UserTokenHash != "",
		RefreshMinutes:      refreshMinutes(cfg.Settings),
		TrafficQueryMinutes: trafficQueryMinutes(cfg.Settings),
	}
}

func adminUser() domain.User {
	return domain.User{
		ID:      "admin",
		Slug:    "admin",
		Name:    "Admin",
		Role:    "admin",
		Enabled: true,
	}
}

func ensureAdminUser(cfg *domain.Config) {
	for i := range cfg.Users {
		if cfg.Users[i].ID == "admin" {
			cfg.Users[i].Slug = "admin"
			cfg.Users[i].Name = "Admin"
			cfg.Users[i].Role = "admin"
			cfg.Users[i].Enabled = true
			if cfg.Users[i].CreatedAt.IsZero() {
				cfg.Users[i].CreatedAt = time.Now()
			}
			return
		}
	}
	user := adminUser()
	user.CreatedAt = time.Now()
	cfg.Users = append([]domain.User{user}, cfg.Users...)
}

func findUser(cfg domain.Config, id string) domain.User {
	for _, user := range cfg.Users {
		if user.ID == id {
			return user
		}
	}
	if id == "admin" {
		return adminUser()
	}
	return domain.User{}
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
