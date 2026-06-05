package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"sub-nest/internal/domain"
	"sub-nest/internal/store"
)

func TestUpdateAdminTokenInvalidatesOldToken(t *testing.T) {
	handler, _ := newTestServer(t)
	adminToken := setupAdmin(t, handler, "admin-old-token")

	updateBody := map[string]string{
		"currentToken": "admin-old-token",
		"newToken":     "admin-new-token",
	}
	updateResp := doJSON(t, handler, http.MethodPut, "/api/settings/admin-token", updateBody, adminToken)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update admin token status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(updateResp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if payload.Token == "" {
		t.Fatal("expected new session token")
	}

	oldResp := doJSON(t, handler, http.MethodGet, "/api/settings", nil, adminToken)
	if oldResp.Code != http.StatusUnauthorized {
		t.Fatalf("old token status = %d, want 401", oldResp.Code)
	}

	newResp := doJSON(t, handler, http.MethodGet, "/api/settings", nil, payload.Token)
	if newResp.Code != http.StatusOK {
		t.Fatalf("new token status = %d, body = %s", newResp.Code, newResp.Body.String())
	}
}

func TestUserTokenProtectsPublicSubscription(t *testing.T) {
	handler, st := newTestServer(t)
	adminToken := setupAdmin(t, handler, "admin-token")
	seedOutputConfig(t, st)

	noTokenBefore := doJSON(t, handler, http.MethodGet, "/s/main", nil, "")
	if noTokenBefore.Code != http.StatusOK {
		t.Fatalf("public subscription before user token status = %d, body = %s", noTokenBefore.Code, noTokenBefore.Body.String())
	}

	updateResp := doJSON(t, handler, http.MethodPut, "/api/settings/user-token", map[string]string{
		"token": "user-sub-token",
	}, adminToken)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update user token status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	noTokenAfter := doJSON(t, handler, http.MethodGet, "/s/main", nil, "")
	if noTokenAfter.Code != http.StatusUnauthorized {
		t.Fatalf("public subscription without user token status = %d, want 401", noTokenAfter.Code)
	}

	wrongToken := doJSON(t, handler, http.MethodGet, "/s/main?token=wrong-token", nil, "")
	if wrongToken.Code != http.StatusUnauthorized {
		t.Fatalf("public subscription with wrong user token status = %d, want 401", wrongToken.Code)
	}

	rightToken := doJSON(t, handler, http.MethodGet, "/s/main?token=user-sub-token", nil, "")
	if rightToken.Code != http.StatusOK {
		t.Fatalf("public subscription with user token status = %d, body = %s", rightToken.Code, rightToken.Body.String())
	}
}

func newTestServer(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	srv := New(st, "", slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil)))
	return srv.Routes(), st
}

func setupAdmin(t *testing.T, handler http.Handler, token string) string {
	t.Helper()
	resp := doJSON(t, handler, http.MethodPost, "/api/setup", map[string]string{"token": token}, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("setup status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode setup response: %v", err)
	}
	return payload.Token
}

func seedOutputConfig(t *testing.T, st *store.Store) {
	t.Helper()
	err := st.Update(func(cfg *domain.Config) error {
		cfg.Sources = []domain.Source{{
			ID:            "source-1",
			Name:          "Source",
			Enabled:       true,
			LastStatus:    "ok",
			LastNodeCount: 1,
			CachedNodes: []domain.Node{{
				Name:   "Test Node",
				Type:   "ss",
				Server: "127.0.0.1",
				Port:   8388,
				Raw:    "ss://example",
			}},
		}}
		cfg.Outputs = []domain.Output{{
			ID:        "output-1",
			Slug:      "main",
			Name:      "Main",
			Enabled:   true,
			Format:    "base64",
			SourceIDs: []string{"source-1"},
		}}
		return nil
	})
	if err != nil {
		t.Fatalf("seed config: %v", err)
	}
}

func doJSON(t *testing.T, handler http.Handler, method string, path string, body interface{}, token string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}
