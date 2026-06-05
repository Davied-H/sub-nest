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
	mux.Handle("GET /api/admin/users", s.authAdmin(http.HandlerFunc(s.handleUsers)))
	mux.Handle("PUT /api/admin/users/{id}", s.authAdmin(http.HandlerFunc(s.handleUpdateUser)))

	mux.Handle("GET /api/dashboard", s.auth(http.HandlerFunc(s.handleDashboard)))
	mux.Handle("GET /api/settings", s.auth(http.HandlerFunc(s.handleSettings)))
	mux.Handle("PUT /api/settings/admin-token", s.authAdmin(http.HandlerFunc(s.handleUpdateAdminToken)))
	mux.Handle("PUT /api/settings/user-token", s.authAdmin(http.HandlerFunc(s.handleUpdateUserToken)))
	mux.Handle("GET /api/sources", s.auth(http.HandlerFunc(s.handleSources)))
	mux.Handle("POST /api/sources", s.auth(http.HandlerFunc(s.handleCreateSource)))
	mux.Handle("PUT /api/sources/{id}", s.auth(http.HandlerFunc(s.handleUpdateSource)))
	mux.Handle("DELETE /api/sources/{id}", s.auth(http.HandlerFunc(s.handleDeleteSource)))
	mux.Handle("POST /api/sources/{id}/refresh", s.auth(http.HandlerFunc(s.handleRefreshSource)))
	mux.Handle("POST /api/sources/{id}/traffic-query", s.auth(http.HandlerFunc(s.handleQuerySourceTraffic)))
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
	writeJSON(w, http.StatusOK, domain.SettingsView{
		PublicBaseURL: s.store.Snapshot().Settings.PublicBaseURL,
		HasUserToken:  token != "",
	})
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
	source, info, err := s.querySourceTraffic(ownerID, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	source.TrafficInfo = info
	writeJSON(w, http.StatusOK, domain.SourceToView(source, false))
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

func (s *Server) handleOutputs(w http.ResponseWriter, r *http.Request) {
	ownerID, _, ok := s.resolveOwner(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在或已禁用")
		return
	}
	cfg := s.store.Snapshot()
	outputs := make([]domain.Output, 0, len(cfg.Outputs))
	for _, output := range cfg.Outputs {
		if output.OwnerUserID == ownerID {
			outputs = append(outputs, output)
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
	s.servePublicSubscription(w, r, "admin", r.PathValue("slug"))
}

func (s *Server) handleUserPublicSubscription(w http.ResponseWriter, r *http.Request) {
	userSlug := normalizeSlug(r.PathValue("userSlug"))
	cfg := s.store.Snapshot()
	for _, user := range cfg.Users {
		if user.Slug == userSlug && user.Enabled {
			s.servePublicSubscription(w, r, user.ID, r.PathValue("slug"))
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
		data, contentType, err := aggregator.Render(output, result)
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

func (s *Server) querySourceTraffic(ownerID string, id string) (domain.Source, domain.TrafficInfo, error) {
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
	info, queryErr := s.fetcher.QueryTraffic(source)
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
	out.Outputs = []domain.Output{}
	for _, output := range cfg.Outputs {
		if output.OwnerUserID == ownerID {
			out.Outputs = append(out.Outputs, output)
		}
	}
	return out
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
