package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"sub-nest/internal/domain"
	"sub-nest/internal/store"
)

func TestUpdateAdminTokenInvalidatesOldToken(t *testing.T) {
	app, _ := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-old-token")

	var payload struct {
		Token string `json:"token"`
	}
	requestJSON(t, app, http.MethodPut, "/api/settings/admin-token", adminToken, map[string]string{
		"currentToken": "admin-old-token",
		"newToken":     "admin-new-token",
	}, http.StatusOK, &payload)
	if payload.Token == "" {
		t.Fatal("expected new session token")
	}

	requestJSON(t, app, http.MethodGet, "/api/settings", adminToken, nil, http.StatusUnauthorized, nil)
	requestJSON(t, app, http.MethodGet, "/api/settings", payload.Token, nil, http.StatusOK, nil)
}

func TestUserTokenProtectsPublicSubscription(t *testing.T) {
	app, st := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token")
	seedOutputConfig(t, st)

	requestJSON(t, app, http.MethodGet, "/s/main", "", nil, http.StatusOK, nil)

	requestJSON(t, app, http.MethodPut, "/api/settings/user-token", adminToken, map[string]string{
		"token": "user-sub-token",
	}, http.StatusOK, nil)

	requestJSON(t, app, http.MethodGet, "/s/main", "", nil, http.StatusUnauthorized, nil)
	requestJSON(t, app, http.MethodGet, "/s/main?token=wrong-token", "", nil, http.StatusUnauthorized, nil)
	requestJSON(t, app, http.MethodGet, "/s/main?token=user-sub-token", "", nil, http.StatusOK, nil)
}

func TestMultiUserInviteIsolationAndPublicRoutes(t *testing.T) {
	app, _ := newTestServer(t)

	adminToken := setupAdmin(t, app, "admin-token-1")
	invite := createInvite(t, app, adminToken)
	userToken := registerUser(t, app, invite, "alice", "alice-token-1")

	adminSourceID := createSource(t, app, adminToken, "", "admin-source")
	userSourceID := createSource(t, app, userToken, "", "alice-source")
	adminOutputID := createOutput(t, app, adminToken, "", "main", adminSourceID)
	userOutputID := createOutput(t, app, userToken, "", "main", userSourceID)

	if adminOutputID == userOutputID {
		t.Fatalf("expected distinct output ids")
	}
	if got := len(listOutputs(t, app, adminToken, "")); got != 1 {
		t.Fatalf("admin sees %d outputs, want 1", got)
	}
	if got := len(listOutputs(t, app, userToken, "")); got != 1 {
		t.Fatalf("user sees %d outputs, want 1", got)
	}

	requestJSON(t, app, http.MethodPut, "/api/outputs/"+adminOutputID, userToken, map[string]interface{}{
		"name":      "steal",
		"slug":      "stolen",
		"enabled":   true,
		"format":    "clash",
		"sourceIds": []string{userSourceID},
		"filter":    map[string]interface{}{},
	}, http.StatusBadRequest, nil)

	requestJSON(t, app, http.MethodGet, "/s/main", "", nil, http.StatusOK, nil)
	requestJSON(t, app, http.MethodGet, "/u/alice/s/main", "", nil, http.StatusOK, nil)
}

func TestInviteCodeIsOneTime(t *testing.T) {
	app, _ := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token-2")
	invite := createInvite(t, app, adminToken)
	_ = registerUser(t, app, invite, "alice", "alice-token-2")

	requestJSON(t, app, http.MethodPost, "/api/register", "", map[string]string{
		"inviteCode": invite,
		"userSlug":   "bob",
		"token":      "bob-token-2",
	}, http.StatusUnauthorized, nil)
}

func TestRegisterRejectsDuplicateToken(t *testing.T) {
	app, _ := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token-duplicate")
	invite := createInvite(t, app, adminToken)

	requestJSON(t, app, http.MethodPost, "/api/register", "", map[string]string{
		"inviteCode": invite,
		"userSlug":   "alice",
		"token":      "admin-token-duplicate",
	}, http.StatusConflict, nil)
}

func TestAdminCanManageTargetUser(t *testing.T) {
	app, _ := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token-3")
	invite := createInvite(t, app, adminToken)
	_ = registerUser(t, app, invite, "alice", "alice-token-3")
	users := listUsers(t, app, adminToken)
	aliceID := ""
	for _, user := range users {
		if user["slug"] == "alice" {
			aliceID = stringValue(user["id"])
		}
	}
	if aliceID == "" {
		t.Fatalf("alice user not found")
	}

	sourceID := createSource(t, app, adminToken, aliceID, "alice-admin-created")
	_ = createOutput(t, app, adminToken, aliceID, "main", sourceID)
	if got := len(listOutputs(t, app, adminToken, aliceID)); got != 1 {
		t.Fatalf("admin target user sees %d outputs, want 1", got)
	}

	requestJSON(t, app, http.MethodPut, "/api/admin/users/"+aliceID, adminToken, map[string]bool{"enabled": false}, http.StatusOK, nil)
	requestJSON(t, app, http.MethodGet, "/u/alice/s/main", "", nil, http.StatusNotFound, nil)
}

func TestQuerySourceTraffic(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1893456000")
		_, _ = w.Write([]byte("ss://example"))
	}))
	defer upstream.Close()

	app, _ := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token-traffic")
	var created map[string]interface{}
	requestJSON(t, app, http.MethodPost, "/api/sources", adminToken, map[string]interface{}{
		"name":       "traffic-source",
		"url":        upstream.URL,
		"sourceType": "url",
		"enabled":    true,
		"tags":       []string{},
		"trafficQuery": map[string]interface{}{
			"mode": "subscription-header",
		},
	}, http.StatusCreated, &created)
	sourceID := stringValue(created["id"])

	var queried map[string]interface{}
	requestJSON(t, app, http.MethodPost, "/api/sources/"+sourceID+"/traffic-query", adminToken, nil, http.StatusOK, &queried)
	info, ok := queried["trafficInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("trafficInfo missing: %#v", queried)
	}
	if info["lastStatus"] != "ok" || info["remainingBytes"].(float64) != 70 {
		t.Fatalf("unexpected traffic info: %#v", info)
	}
}

func newTestServer(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return New(st, "", slog.New(slog.NewTextHandler(os.Stdout, nil))).Routes(), st
}

func setupAdmin(t *testing.T, app http.Handler, token string) string {
	t.Helper()
	var out map[string]interface{}
	requestJSON(t, app, http.MethodPost, "/api/setup", "", map[string]string{"token": token}, http.StatusOK, &out)
	return stringValue(out["token"])
}

func createInvite(t *testing.T, app http.Handler, token string) string {
	t.Helper()
	var out map[string]interface{}
	requestJSON(t, app, http.MethodPost, "/api/admin/invite-codes", token, map[string]string{"label": "test"}, http.StatusCreated, &out)
	return stringValue(out["code"])
}

func registerUser(t *testing.T, app http.Handler, invite string, slug string, token string) string {
	t.Helper()
	var out map[string]interface{}
	requestJSON(t, app, http.MethodPost, "/api/register", "", map[string]string{
		"inviteCode": invite,
		"userSlug":   slug,
		"token":      token,
	}, http.StatusCreated, &out)
	return stringValue(out["token"])
}

func seedOutputConfig(t *testing.T, st *store.Store) {
	t.Helper()
	err := st.Update(func(cfg *domain.Config) error {
		cfg.Sources = []domain.Source{{
			ID:            "source-1",
			OwnerUserID:   "admin",
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
			ID:          "output-1",
			OwnerUserID: "admin",
			Slug:        "main",
			Name:        "Main",
			Enabled:     true,
			Format:      "base64",
			SourceIDs:   []string{"source-1"},
		}}
		return nil
	})
	if err != nil {
		t.Fatalf("seed config: %v", err)
	}
}

func createSource(t *testing.T, app http.Handler, token string, userID string, name string) string {
	t.Helper()
	path := "/api/sources"
	if userID != "" {
		path += "?userId=" + userID
	}
	var out map[string]interface{}
	requestJSON(t, app, http.MethodPost, path, token, map[string]interface{}{
		"name":        name,
		"sourceType":  "file",
		"fileName":    name + ".txt",
		"fileContent": "ss://YWVzLTEyOC1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjgzODg=#node",
		"enabled":     true,
		"tags":        []string{},
	}, http.StatusCreated, &out)
	return stringValue(out["id"])
}

func createOutput(t *testing.T, app http.Handler, token string, userID string, slug string, sourceID string) string {
	t.Helper()
	path := "/api/outputs"
	if userID != "" {
		path += "?userId=" + userID
	}
	var out map[string]interface{}
	requestJSON(t, app, http.MethodPost, path, token, map[string]interface{}{
		"name":      slug,
		"slug":      slug,
		"enabled":   true,
		"format":    "clash",
		"sourceIds": []string{sourceID},
		"filter": map[string]interface{}{
			"includeKeywords": []string{},
			"excludeKeywords": []string{},
			"regex":           "",
		},
		"renameRules": []interface{}{},
		"groupMode":   "region",
	}, http.StatusCreated, &out)
	return stringValue(out["id"])
}

func listOutputs(t *testing.T, app http.Handler, token string, userID string) []map[string]interface{} {
	t.Helper()
	path := "/api/outputs"
	if userID != "" {
		path += "?userId=" + userID
	}
	var out []map[string]interface{}
	requestJSON(t, app, http.MethodGet, path, token, nil, http.StatusOK, &out)
	return out
}

func listUsers(t *testing.T, app http.Handler, token string) []map[string]interface{} {
	t.Helper()
	var out []map[string]interface{}
	requestJSON(t, app, http.MethodGet, "/api/admin/users", token, nil, http.StatusOK, &out)
	return out
}

func requestJSON(t *testing.T, app http.Handler, method string, path string, token string, body interface{}, want int, out interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, path, jsonBody(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if res.Code != want {
		t.Fatalf("%s %s status = %d, want %d, body %s", method, path, res.Code, want, res.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(res.Body.Bytes(), out); err != nil {
			t.Fatalf("decode response: %v; body %s", err, res.Body.String())
		}
	}
}

func jsonBody(body interface{}) *bytes.Reader {
	if body == nil {
		return bytes.NewReader(nil)
	}
	data, _ := json.Marshal(body)
	return bytes.NewReader(data)
}

func stringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
