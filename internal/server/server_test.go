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

	"sub-nest/internal/store"
)

func TestMultiUserInviteIsolationAndPublicRoutes(t *testing.T) {
	app := newTestServer(t)

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

	req := httptest.NewRequest(http.MethodPut, "/api/outputs/"+adminOutputID, jsonBody(map[string]interface{}{
		"name":      "steal",
		"slug":      "stolen",
		"enabled":   true,
		"format":    "clash",
		"sourceIds": []string{userSourceID},
		"filter":    map[string]interface{}{},
	}))
	req.Header.Set("Authorization", "Bearer "+userToken)
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("user updating admin output status = %d, want %d", res.Code, http.StatusBadRequest)
	}

	req = httptest.NewRequest(http.MethodGet, "/s/main", nil)
	res = httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("legacy admin public route status = %d, body %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/u/alice/s/main", nil)
	res = httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("user public route status = %d, body %s", res.Code, res.Body.String())
	}
}

func TestInviteCodeIsOneTime(t *testing.T) {
	app := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token-2")
	invite := createInvite(t, app, adminToken)
	_ = registerUser(t, app, invite, "alice", "alice-token-2")

	req := httptest.NewRequest(http.MethodPost, "/api/register", jsonBody(map[string]string{
		"inviteCode": invite,
		"userSlug":   "bob",
		"token":      "bob-token-2",
	}))
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("second register status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestRegisterRejectsDuplicateToken(t *testing.T) {
	app := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token-duplicate")
	invite := createInvite(t, app, adminToken)

	req := httptest.NewRequest(http.MethodPost, "/api/register", jsonBody(map[string]string{
		"inviteCode": invite,
		"userSlug":   "alice",
		"token":      "admin-token-duplicate",
	}))
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("duplicate admin token register status = %d, want %d", res.Code, http.StatusConflict)
	}
}

func TestAdminCanManageTargetUser(t *testing.T) {
	app := newTestServer(t)
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

	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/"+aliceID, jsonBody(map[string]bool{"enabled": false}))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("disable user status = %d, body %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/u/alice/s/main", nil)
	res = httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("disabled user public route status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return New(st, "", slog.New(slog.NewTextHandler(os.Stdout, nil))).Routes()
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
