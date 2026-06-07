package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	requestText(t, app, http.MethodGet, "/s/main.pac", "", http.StatusOK)

	requestJSON(t, app, http.MethodPut, "/api/settings/user-token", adminToken, map[string]string{
		"token": "user-sub-token",
	}, http.StatusOK, nil)

	requestJSON(t, app, http.MethodGet, "/s/main", "", nil, http.StatusUnauthorized, nil)
	requestJSON(t, app, http.MethodGet, "/s/main?token=wrong-token", "", nil, http.StatusUnauthorized, nil)
	requestJSON(t, app, http.MethodGet, "/s/main?token=user-sub-token", "", nil, http.StatusOK, nil)
	requestJSON(t, app, http.MethodGet, "/s/main.pac", "", nil, http.StatusUnauthorized, nil)
	pac := requestText(t, app, http.MethodGet, "/s/main.pac?token=user-sub-token", "", http.StatusOK)
	if !strings.Contains(pac, "FindProxyForURL") || !strings.Contains(pac, "127.0.0.1") {
		t.Fatalf("unexpected PAC body:\n%s", pac)
	}
}

func TestPublicPACSyncsOnlineRules(t *testing.T) {
	ruleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("server=/online.example/114.114.114.114\n"))
	}))
	defer ruleServer.Close()

	app, st := newTestServer(t)
	_ = setupAdmin(t, app, "admin-token-pac-sync")
	seedOutputConfig(t, st)
	err := st.Update(func(cfg *domain.Config) error {
		cfg.Outputs[0].PAC = domain.NormalizePACConfig(domain.PACConfig{
			Enabled:              true,
			Proxy:                "PROXY 127.0.0.1:7890; DIRECT",
			RuleSourceURL:        ruleServer.URL,
			RuleSourceFormat:     "dnsmasq",
			RuleRefreshHours:     24,
			DirectDomainSuffixes: []string{"manual.example"},
			DirectCIDRs:          []string{"10.0.0.0/8"},
		})
		return nil
	})
	if err != nil {
		t.Fatalf("seed pac config: %v", err)
	}

	pac := requestText(t, app, http.MethodGet, "/s/main.pac", "", http.StatusOK)
	if !strings.Contains(pac, `"online.example"`) || !strings.Contains(pac, `"manual.example"`) {
		t.Fatalf("unexpected PAC body:\n%s", pac)
	}
	cfg := st.Snapshot()
	if cfg.Outputs[0].PAC.LastSyncStatus != "ok" || len(cfg.Outputs[0].PAC.CachedDomainSuffixes) != 1 {
		t.Fatalf("PAC sync status = %#v", cfg.Outputs[0].PAC)
	}
}

func TestRuleSetSyncAggregatesDedupesAndExcludesDomains(t *testing.T) {
	ruleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			_, _ = w.Write([]byte("DOMAIN-SUFFIX,shared.example\nDOMAIN-SUFFIX,one.example\n"))
		case "/b":
			_, _ = w.Write([]byte("DOMAIN-SUFFIX,shared.example\nDOMAIN-SUFFIX,blocked.example\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ruleServer.Close()

	app, st := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token-rule-set")
	seedOutputConfig(t, st)
	err := st.Update(func(cfg *domain.Config) error {
		cfg.RuleSources = []domain.RuleSource{
			{ID: "source-a", OwnerUserID: "admin", Name: "Source A", URL: ruleServer.URL + "/a", Format: "clash-domain", RefreshHours: 24},
			{ID: "source-b", OwnerUserID: "admin", Name: "Source B", URL: ruleServer.URL + "/b", Format: "clash-domain", RefreshHours: 24},
		}
		cfg.RuleSets = []domain.RuleSet{{
			ID:                     "aggregate",
			OwnerUserID:            "admin",
			Name:                   "Aggregate",
			SourceIDs:              []string{"source-a", "source-b"},
			DirectDomainSuffixes:   []string{"manual.example"},
			ExcludedDomainSuffixes: []string{"blocked.example"},
		}}
		cfg.Outputs[0].PAC = domain.NormalizePACConfig(domain.PACConfig{
			Enabled:   true,
			Proxy:     "PROXY 127.0.0.1:7890; DIRECT",
			RuleSetID: "aggregate",
		})
		return nil
	})
	if err != nil {
		t.Fatalf("seed rule config: %v", err)
	}

	requestJSON(t, app, http.MethodPost, "/api/rule-sets/aggregate/sync", adminToken, map[string]string{}, http.StatusOK, nil)
	cfg := st.Snapshot()
	if got := len(cfg.RuleSets[0].CachedDomainSuffixes); got != 2 {
		t.Fatalf("cached domains = %d (%#v), want 2", got, cfg.RuleSets[0].CachedDomainSuffixes)
	}
	pac := requestText(t, app, http.MethodGet, "/s/main.pac", "", http.StatusOK)
	for _, want := range []string{`"shared.example"`, `"one.example"`, `"manual.example"`} {
		if !strings.Contains(pac, want) {
			t.Fatalf("PAC missing %s:\n%s", want, pac)
		}
	}
	if strings.Contains(pac, "blocked.example") {
		t.Fatalf("PAC should exclude blocked.example:\n%s", pac)
	}
}

func TestRuleConfigSaveOmitsAndPreservesCachedDomains(t *testing.T) {
	app, st := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token-rule-cache")
	now := time.Now()
	err := st.Update(func(cfg *domain.Config) error {
		cfg.RuleSources = []domain.RuleSource{{
			ID:                   "source-a",
			OwnerUserID:          "admin",
			Name:                 "Source A",
			URL:                  "https://example.com/a.list",
			Format:               "domain-list",
			RefreshHours:         24,
			CachedDomainSuffixes: []string{"cached.example"},
			LastSyncedAt:         &now,
			LastSyncStatus:       "ok",
		}}
		cfg.RuleSets = []domain.RuleSet{{
			ID:                   "aggregate",
			OwnerUserID:          "admin",
			Name:                 "Aggregate",
			SourceIDs:            []string{"source-a"},
			CachedDomainSuffixes: []string{"cached.example"},
			LastSyncedAt:         &now,
			LastSyncStatus:       "ok",
		}}
		return nil
	})
	if err != nil {
		t.Fatalf("seed rule cache: %v", err)
	}

	var sources []map[string]interface{}
	requestJSON(t, app, http.MethodGet, "/api/rule-sources", adminToken, nil, http.StatusOK, &sources)
	if got := sources[0]["cachedDomainCount"].(float64); got != 1 {
		t.Fatalf("source cachedDomainCount = %v, want 1", got)
	}
	if _, ok := sources[0]["cachedDomainSuffixes"]; ok {
		t.Fatalf("rule source response should omit cachedDomainSuffixes: %#v", sources[0])
	}

	var sets []map[string]interface{}
	requestJSON(t, app, http.MethodGet, "/api/rule-sets", adminToken, nil, http.StatusOK, &sets)
	if got := sets[0]["cachedDomainCount"].(float64); got != 1 {
		t.Fatalf("set cachedDomainCount = %v, want 1", got)
	}
	if _, ok := sets[0]["cachedDomainSuffixes"]; ok {
		t.Fatalf("rule set response should omit cachedDomainSuffixes: %#v", sets[0])
	}

	requestJSON(t, app, http.MethodPut, "/api/rule-sources", adminToken, []map[string]interface{}{{
		"id":           "source-a",
		"name":         "Source A Updated",
		"url":          "https://example.com/a.list",
		"format":       "domain-list",
		"refreshHours": 24,
	}}, http.StatusOK, nil)
	requestJSON(t, app, http.MethodPut, "/api/rule-sets", adminToken, []map[string]interface{}{{
		"id":                     "aggregate",
		"name":                   "Aggregate Updated",
		"sourceIds":              []string{"source-a"},
		"directDomainSuffixes":   []string{"manual.example"},
		"excludedDomainSuffixes": []string{},
		"directCidrs":            []string{},
	}}, http.StatusOK, nil)

	cfg := st.Snapshot()
	if got := cfg.RuleSources[0].CachedDomainSuffixes; len(got) != 1 || got[0] != "cached.example" {
		t.Fatalf("source cache = %#v, want cached.example", got)
	}
	if got := cfg.RuleSets[0].CachedDomainSuffixes; len(got) != 1 || got[0] != "cached.example" {
		t.Fatalf("set cache = %#v, want cached.example", got)
	}

	requestJSON(t, app, http.MethodPut, "/api/rule-sources", adminToken, []map[string]interface{}{{
		"id":           "source-a",
		"name":         "Source A New URL",
		"url":          "https://example.com/changed.list",
		"format":       "domain-list",
		"refreshHours": 24,
	}}, http.StatusOK, nil)
	cfg = st.Snapshot()
	if got := cfg.RuleSources[0].CachedDomainSuffixes; len(got) != 0 {
		t.Fatalf("source cache after url change = %#v, want empty", got)
	}
	if got := cfg.RuleSets[0].CachedDomainSuffixes; len(got) != 0 {
		t.Fatalf("set cache after url change = %#v, want empty", got)
	}
}

func TestRuleSetDomainsSearch(t *testing.T) {
	app, st := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token-rule-domains")
	now := time.Now()
	err := st.Update(func(cfg *domain.Config) error {
		cfg.RuleSources = []domain.RuleSource{{
			ID:                   "source-a",
			OwnerUserID:          "admin",
			Name:                 "Source A",
			URL:                  "https://example.com/a.list",
			Format:               "domain-list",
			RefreshHours:         24,
			CachedDomainSuffixes: []string{"alpha.example", "media.example", "blocked.example"},
			LastSyncedAt:         &now,
			LastSyncStatus:       "ok",
		}}
		cfg.RuleSets = []domain.RuleSet{{
			ID:                     "aggregate",
			OwnerUserID:            "admin",
			Name:                   "Aggregate",
			SourceIDs:              []string{"source-a"},
			DirectDomainSuffixes:   []string{"manual.example"},
			ExcludedDomainSuffixes: []string{"blocked.example"},
			CachedDomainSuffixes:   []string{"alpha.example", "orphan.example"},
			LastSyncedAt:           &now,
			LastSyncStatus:         "ok",
		}}
		return nil
	})
	if err != nil {
		t.Fatalf("seed rule domains: %v", err)
	}

	var searched map[string]interface{}
	requestJSON(t, app, http.MethodGet, "/api/rule-sets/aggregate/domains?q=media", adminToken, nil, http.StatusOK, &searched)
	if searched["total"].(float64) != 5 || searched["matched"].(float64) != 1 {
		t.Fatalf("unexpected counts: %#v", searched)
	}
	domains := searched["domains"].([]interface{})
	if len(domains) != 1 {
		t.Fatalf("domains = %#v, want one", domains)
	}
	row := domains[0].(map[string]interface{})
	if row["domain"] != "media.example" || row["source"] != "Source A" || row["type"] != "cache" {
		t.Fatalf("unexpected row: %#v", row)
	}

	var limited map[string]interface{}
	requestJSON(t, app, http.MethodGet, "/api/rule-sets/aggregate/domains?q=example&limit=2", adminToken, nil, http.StatusOK, &limited)
	if limited["matched"].(float64) != 5 || limited["truncated"].(bool) != true {
		t.Fatalf("unexpected limited result: %#v", limited)
	}
	if got := len(limited["domains"].([]interface{})); got != 2 {
		t.Fatalf("limited rows = %d, want 2", got)
	}
}

func TestUpdateTrafficQuerySettings(t *testing.T) {
	app, _ := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token-traffic-settings")

	var settings map[string]interface{}
	requestJSON(t, app, http.MethodGet, "/api/settings", adminToken, nil, http.StatusOK, &settings)
	if settings["trafficQueryMinutes"].(float64) != 5 {
		t.Fatalf("default trafficQueryMinutes = %#v, want 5", settings["trafficQueryMinutes"])
	}
	if settings["refreshMinutes"].(float64) != 60 {
		t.Fatalf("default refreshMinutes = %#v, want 60", settings["refreshMinutes"])
	}

	requestJSON(t, app, http.MethodPut, "/api/settings/traffic-query", adminToken, map[string]int{
		"minutes": 7,
	}, http.StatusOK, &settings)
	if settings["trafficQueryMinutes"].(float64) != 7 {
		t.Fatalf("trafficQueryMinutes = %#v, want 7", settings["trafficQueryMinutes"])
	}
	requestJSON(t, app, http.MethodPut, "/api/settings/refresh", adminToken, map[string]int{
		"minutes": 15,
	}, http.StatusOK, &settings)
	if settings["refreshMinutes"].(float64) != 15 {
		t.Fatalf("refreshMinutes = %#v, want 15", settings["refreshMinutes"])
	}
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

func TestDeleteInviteCode(t *testing.T) {
	app, _ := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token-delete-invite")
	var invite map[string]interface{}
	requestJSON(t, app, http.MethodPost, "/api/admin/invite-codes", adminToken, map[string]string{"label": "delete-me"}, http.StatusCreated, &invite)
	inviteID := stringValue(invite["id"])
	inviteCode := stringValue(invite["code"])
	if inviteID == "" || inviteCode == "" {
		t.Fatalf("invite response = %#v", invite)
	}

	requestJSON(t, app, http.MethodDelete, "/api/admin/invite-codes/"+inviteID, adminToken, nil, http.StatusNoContent, nil)
	var invites []map[string]interface{}
	requestJSON(t, app, http.MethodGet, "/api/admin/invite-codes", adminToken, nil, http.StatusOK, &invites)
	if len(invites) != 0 {
		t.Fatalf("invites = %#v, want empty", invites)
	}
	requestJSON(t, app, http.MethodPost, "/api/register", "", map[string]string{
		"inviteCode": inviteCode,
		"userSlug":   "alice",
		"token":      "alice-token-delete-invite",
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

func TestQuerySourceTrafficUsesDraftAndReturnsDebug(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"remaining":"5 GB","total":"20 GB"}}`))
	}))
	defer upstream.Close()

	app, _ := newTestServer(t)
	adminToken := setupAdmin(t, app, "admin-token-traffic-debug")
	var created map[string]interface{}
	requestJSON(t, app, http.MethodPost, "/api/sources", adminToken, map[string]interface{}{
		"name":       "traffic-source",
		"url":        upstream.URL,
		"sourceType": "url",
		"enabled":    true,
		"tags":       []string{},
		"trafficQuery": map[string]interface{}{
			"mode": "custom-http",
			"url":  upstream.URL,
			"parser": map[string]interface{}{
				"type":      "json-path",
				"remaining": "$.data.missing",
			},
		},
	}, http.StatusCreated, &created)
	sourceID := stringValue(created["id"])

	var queried map[string]interface{}
	requestJSON(t, app, http.MethodPost, "/api/sources/"+sourceID+"/traffic-query", adminToken, map[string]interface{}{
		"source": map[string]interface{}{
			"name":       "traffic-source",
			"url":        upstream.URL,
			"sourceType": "url",
			"trafficQuery": map[string]interface{}{
				"mode": "custom-http",
				"url":  upstream.URL,
				"parser": map[string]interface{}{
					"type":      "json-path",
					"remaining": "$.data.remaining",
					"total":     "$.data.total",
				},
			},
		},
	}, http.StatusOK, &queried)
	info, ok := queried["trafficInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("trafficInfo missing: %#v", queried)
	}
	if info["lastStatus"] != "ok" || info["remainingBytes"].(float64) == 0 {
		t.Fatalf("unexpected traffic info: %#v", info)
	}
	debug, ok := info["debug"].(map[string]interface{})
	if !ok {
		t.Fatalf("debug missing: %#v", info)
	}
	if debug["statusCode"].(float64) != 200 || debug["bodyPreview"] == "" {
		t.Fatalf("unexpected debug: %#v", debug)
	}
	paths, ok := debug["paths"].([]interface{})
	if !ok || len(paths) == 0 {
		t.Fatalf("debug paths missing: %#v", debug)
	}
}

func TestPublicSubscriptionIncludesTrafficHeader(t *testing.T) {
	app, st := newTestServer(t)
	_ = setupAdmin(t, app, "admin-token-public-traffic")
	seedOutputConfig(t, st)
	expire := time.Unix(1893456000, 0)
	err := st.Update(func(cfg *domain.Config) error {
		cfg.Sources[0].TrafficInfo = domain.TrafficInfo{
			UploadBytes:    10,
			DownloadBytes:  20,
			TotalBytes:     100,
			RemainingBytes: 70,
			ExpireAt:       &expire,
			LastStatus:     "ok",
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed traffic info: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/s/main", nil)
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", res.Code, res.Body.String())
	}
	got := res.Header().Get("Subscription-Userinfo")
	want := "upload=10; download=20; total=100; expire=1893456000"
	if got != want {
		t.Fatalf("Subscription-Userinfo = %q, want %q", got, want)
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

func requestText(t *testing.T, app http.Handler, method string, path string, token string, want int) string {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if res.Code != want {
		t.Fatalf("%s %s status = %d, want %d, body %s", method, path, res.Code, want, res.Body.String())
	}
	return res.Body.String()
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
