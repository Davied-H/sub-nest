package aggregator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"sub-nest/internal/domain"
)

func TestQueryTrafficFromSubscriptionHeader(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1024; download=2048; total=4096; expire=1893456000")
		_, _ = w.Write([]byte("ss://example"))
	}))
	defer upstream.Close()

	info, err := NewFetcher().QueryTraffic(domain.Source{
		URL: upstream.URL,
		TrafficQuery: domain.TrafficQueryConfig{
			Mode: "subscription-header",
		},
	})
	if err != nil {
		t.Fatalf("QueryTraffic error: %v", err)
	}
	if info.UploadBytes != 1024 || info.DownloadBytes != 2048 || info.TotalBytes != 4096 || info.RemainingBytes != 1024 {
		t.Fatalf("unexpected traffic info: %+v", info)
	}
	if info.ExpireAt == nil {
		t.Fatal("expected expire time")
	}
}

func TestQueryTrafficFromRegexBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("剩余流量 12.5 GB，到期 2026-12-31"))
	}))
	defer upstream.Close()

	info, err := NewFetcher().QueryTraffic(domain.Source{
		URL: upstream.URL,
		TrafficQuery: domain.TrafficQueryConfig{
			Mode: "subscription-body-regex",
			Parser: domain.TrafficParser{
				Remaining: `剩余流量\s*([0-9.]+\s*GB)`,
				Expire:    `到期\s*([0-9-]+)`,
			},
		},
	})
	if err != nil {
		t.Fatalf("QueryTraffic error: %v", err)
	}
	if info.RemainingBytes != int64(12.5*1024*1024*1024) {
		t.Fatalf("remaining = %d", info.RemainingBytes)
	}
	if info.ExpireAt == nil {
		t.Fatal("expected expire time")
	}
}

func TestQueryTrafficFromCustomHTTPJSONPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test") != "ok" {
			t.Fatalf("missing custom header")
		}
		_, _ = w.Write([]byte(`{"data":{"used":{"upload":"1 GB","download":"2 GB"},"total":"10 GB","expire":"1893456000"}}`))
	}))
	defer upstream.Close()

	info, err := NewFetcher().QueryTraffic(domain.Source{
		TrafficQuery: domain.TrafficQueryConfig{
			Mode: "custom-http",
			URL:  upstream.URL,
			Headers: map[string]string{
				"X-Test": "ok",
			},
			Parser: domain.TrafficParser{
				Type:     "json-path",
				Upload:   "$.data.used.upload",
				Download: "$.data.used.download",
				Total:    "$.data.total",
				Expire:   "$.data.expire",
			},
		},
	})
	if err != nil {
		t.Fatalf("QueryTraffic error: %v", err)
	}
	if info.UploadBytes != 1024*1024*1024 || info.DownloadBytes != 2*1024*1024*1024 || info.TotalBytes != 10*1024*1024*1024 {
		t.Fatalf("unexpected traffic info: %+v", info)
	}
}
