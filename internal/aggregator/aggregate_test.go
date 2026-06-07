package aggregator

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"sub-nest/internal/domain"
)

func TestSortNodesByDelay(t *testing.T) {
	alive := true
	dead := false
	nodes := []domain.Node{
		{Name: "failed", Server: "d.example", Port: 4, Alive: &dead, DelayMS: 10},
		{Name: "unknown", Server: "c.example", Port: 3},
		{Name: "fast", Server: "a.example", Port: 1, Alive: &alive, DelayMS: 32},
		{Name: "slow", Server: "b.example", Port: 2, Alive: &alive, DelayMS: 180},
		{Name: "alive-without-delay", Server: "e.example", Port: 5, Alive: &alive},
	}

	gotNodes := SortNodesByDelay(nodes)
	got := make([]string, 0, len(gotNodes))
	for _, node := range gotNodes {
		got = append(got, node.Name)
	}
	want := []string{"fast", "slow", "alive-without-delay", "unknown", "failed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SortNodesByDelay names = %#v, want %#v", got, want)
	}
}

func TestPreviewKeepsDelayOrderInNodesAndGroups(t *testing.T) {
	alive := true
	cfg := domain.Config{
		Sources: []domain.Source{{
			ID:          "source-1",
			OwnerUserID: "admin",
			Enabled:     true,
			LastStatus:  "ok",
			CachedNodes: []domain.Node{
				{Name: "slow", Type: "ss", Server: "hk-slow.example", Port: 2, Region: "香港节点", RegionCode: "HK", Alive: &alive, DelayMS: 220, Extra: map[string]interface{}{"_original_name": "slow"}},
				{Name: "fast", Type: "ss", Server: "hk-fast.example", Port: 1, Region: "香港节点", RegionCode: "HK", Alive: &alive, DelayMS: 40, Extra: map[string]interface{}{"_original_name": "fast"}},
			},
		}},
	}
	output := domain.Output{
		ID:        "output-1",
		Slug:      "main",
		Enabled:   true,
		SourceIDs: []string{"source-1"},
	}

	preview := Preview(cfg, output)
	if len(preview.Nodes) != 2 {
		t.Fatalf("preview nodes = %d, want 2", len(preview.Nodes))
	}
	if preview.Nodes[0].OriginalName != "fast" || preview.Nodes[0].DelayMS != 40 {
		t.Fatalf("first preview node = %#v, want fast with 40ms", preview.Nodes[0])
	}
	for _, group := range preview.Groups {
		if group.Name == "香港节点" {
			want := []string{"香港 01", "香港 02"}
			if !reflect.DeepEqual(group.Nodes, want) {
				t.Fatalf("Hong Kong group nodes = %#v, want %#v", group.Nodes, want)
			}
			return
		}
	}
	t.Fatal("Hong Kong group not found")
}

func TestPreviewIncludesSourceGroups(t *testing.T) {
	alive := true
	cfg := domain.Config{
		Sources: []domain.Source{
			{
				ID:          "source-a",
				Name:        "Alpha",
				OwnerUserID: "admin",
				Enabled:     true,
				LastStatus:  "ok",
				CachedNodes: []domain.Node{
					{Name: "a-node", Type: "ss", Server: "a.example", Port: 1, Region: "日本节点", SourceID: "source-a", Source: "Alpha", Alive: &alive, Extra: map[string]interface{}{"_original_name": "a-node"}},
				},
			},
			{
				ID:          "source-b",
				Name:        "Beta",
				OwnerUserID: "admin",
				Enabled:     true,
				LastStatus:  "ok",
				CachedNodes: []domain.Node{
					{Name: "b-node", Type: "ss", Server: "b.example", Port: 2, Region: "美国节点", SourceID: "source-b", Source: "Beta", Alive: &alive, Extra: map[string]interface{}{"_original_name": "b-node"}},
				},
			},
		},
	}
	output := domain.Output{
		ID:        "output-1",
		Slug:      "main",
		Enabled:   true,
		SourceIDs: []string{"source-a", "source-b"},
	}

	preview := Preview(cfg, output)
	if len(preview.SourceGroups) != 2 {
		t.Fatalf("source groups = %d, want 2", len(preview.SourceGroups))
	}
	want := []domain.GroupPreview{
		{Name: "来源 / Alpha", Nodes: []string{"日本 01"}},
		{Name: "来源 / Beta", Nodes: []string{"美国 01"}},
	}
	if !reflect.DeepEqual(preview.SourceGroups, want) {
		t.Fatalf("source groups = %#v, want %#v", preview.SourceGroups, want)
	}
	if preview.Nodes[0].Source != "Alpha" {
		t.Fatalf("node source = %q, want Alpha", preview.Nodes[0].Source)
	}
}

func TestRenderIncludesSourceProxyGroups(t *testing.T) {
	alive := true
	cfg := domain.Config{
		Sources: []domain.Source{{
			ID:          "source-a",
			Name:        "Alpha",
			OwnerUserID: "admin",
			Enabled:     true,
			LastStatus:  "ok",
			CachedNodes: []domain.Node{
				{Name: "alpha-node", Type: "ss", Server: "alpha.example", Port: 1, Region: "香港节点", SourceID: "source-a", Source: "Alpha", Alive: &alive, Extra: map[string]interface{}{"_original_name": "alpha-node"}},
			},
		}},
	}
	output := domain.Output{
		ID:        "output-1",
		Slug:      "main",
		Enabled:   true,
		Format:    "clash",
		SourceIDs: []string{"source-a"},
		PAC: domain.PACConfig{
			DirectDomainSuffixes: []string{"example.cn", "qq.com"},
			DirectCIDRs:          []string{"10.0.0.0/8"},
			CachedDomainSuffixes: []string{"online.example"},
		},
	}

	data, _, err := Render(output, Build(cfg, output))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse rendered yaml: %v\n%s", err, string(data))
	}
	text := string(data)
	if !strings.Contains(text, "来源 / Alpha") {
		t.Fatalf("rendered yaml missing source group:\n%s", text)
	}
	groups, ok := doc["proxy-groups"].([]interface{})
	if !ok {
		t.Fatalf("proxy-groups has unexpected type %#v", doc["proxy-groups"])
	}
	var found bool
	for _, item := range groups {
		group, ok := item.(map[string]interface{})
		if !ok || group["name"] != "来源 / Alpha" {
			continue
		}
		found = true
		proxies, ok := group["proxies"].([]interface{})
		if !ok || len(proxies) != 1 || proxies[0] != "香港 01" {
			t.Fatalf("source group proxies = %#v, want 香港 01", group["proxies"])
		}
	}
	if !found {
		t.Fatalf("source proxy group not found in %#v", groups)
	}
}

func TestRenderIncludesChinaDirectRules(t *testing.T) {
	alive := true
	cfg := domain.Config{
		Sources: []domain.Source{{
			ID:          "source-a",
			OwnerUserID: "admin",
			Enabled:     true,
			LastStatus:  "ok",
			CachedNodes: []domain.Node{
				{Name: "alpha-node", Type: "ss", Server: "alpha.example", Port: 1, Alive: &alive},
			},
		}},
	}
	output := domain.Output{
		ID:        "output-1",
		Slug:      "main",
		Enabled:   true,
		Format:    "clash",
		SourceIDs: []string{"source-a"},
		PAC: domain.PACConfig{
			DomainKeywords:       []string{"poe"},
			DirectDomainSuffixes: []string{"example.cn"},
			DirectCIDRs:          []string{"10.0.0.0/8"},
			CachedDomainSuffixes: []string{"online.example"},
		},
	}

	data, _, err := Render(output, Build(cfg, output))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse rendered yaml: %v\n%s", err, string(data))
	}
	rules, ok := doc["rules"].([]interface{})
	if !ok {
		t.Fatalf("rules has unexpected type %#v", doc["rules"])
	}
	got := make([]string, 0, len(rules))
	for _, rule := range rules {
		got = append(got, rule.(string))
	}
	for _, want := range []string{"DOMAIN-KEYWORD,poe,节点选择", "DOMAIN-SUFFIX,example.cn,DIRECT", "DOMAIN-SUFFIX,online.example,DIRECT", "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve", "GEOIP,CN,DIRECT", "MATCH,节点选择"} {
		if !containsString(got, want) {
			t.Fatalf("rules missing %q in %#v", want, got)
		}
	}
}

func TestRenderPACDirectsChinaDomains(t *testing.T) {
	data, contentType, err := RenderPAC(domain.Output{
		Slug: "main",
		PAC: domain.PACConfig{
			Proxy:                "PROXY 127.0.0.1:7890; DIRECT",
			DirectDomainSuffixes: []string{"manual.example"},
			CachedDomainSuffixes: []string{"online.example"},
		},
	}, Result{Nodes: []domain.Node{{Name: "node"}}})
	if err != nil {
		t.Fatalf("RenderPAC: %v", err)
	}
	if contentType != "application/x-ns-proxy-autoconfig; charset=utf-8" {
		t.Fatalf("content type = %q", contentType)
	}
	text := string(data)
	for _, want := range []string{`"manual.example"`, `"online.example"`, `directDomainSuffixes`, `return "DIRECT";`, `PROXY 127.0.0.1:7890; DIRECT`, `FindProxyForURL`} {
		if !strings.Contains(text, want) {
			t.Fatalf("PAC missing %q:\n%s", want, text)
		}
	}
}

func TestParsePACRuleSourceDNSMasq(t *testing.T) {
	got, err := ParsePACRuleSource(strings.NewReader(`
# comment
server=/qq.com/114.114.114.114
address=/taobao.com/223.5.5.5
server=/qq.com/114.114.114.114
`), "dnsmasq")
	if err != nil {
		t.Fatalf("ParsePACRuleSource: %v", err)
	}
	want := []string{"qq.com", "taobao.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("domains = %#v, want %#v", got, want)
	}
}

func TestParsePACRuleSourceClashDomain(t *testing.T) {
	got, err := ParsePACRuleSource(strings.NewReader(`
# comment
DOMAIN-SUFFIX,qq.com
DOMAIN,example.cn
IP-CIDR,10.0.0.0/8
`), "clash-domain")
	if err != nil {
		t.Fatalf("ParsePACRuleSource: %v", err)
	}
	want := []string{"qq.com", "example.cn"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("domains = %#v, want %#v", got, want)
	}
}

func TestParsePACRuleSourceYAMLPayload(t *testing.T) {
	got, err := ParsePACRuleSource(strings.NewReader(`
payload:
  - 'qq.com'
  - "+.taobao.com"
  - ".example.cn"
  - 'qq.com'
`), "yaml-payload")
	if err != nil {
		t.Fatalf("ParsePACRuleSource: %v", err)
	}
	want := []string{"qq.com", "taobao.com", "example.cn"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("domains = %#v, want %#v", got, want)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
