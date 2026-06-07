package domain

import (
	"net/url"
	"strings"
	"time"
)

const DefaultPACRuleSourceURL = "https://cdn.jsdelivr.net/gh/ACL4SSR/ACL4SSR@master/Clash/ChinaDomain.list"
const LegacyDefaultPACRuleSourceURL = "https://raw.githubusercontent.com/felixonmars/dnsmasq-china-list/master/accelerated-domains.china.conf"
const LegacyCDNPACRuleSourceURL = "https://cdn.jsdelivr.net/gh/felixonmars/dnsmasq-china-list@master/accelerated-domains.china.conf"
const DefaultRuleSourceID = "acl4ssr-china-domain"
const DefaultRuleSetID = "china-direct"
const DefaultPACRuleSourceLocalPath = "rules/pac/acl4ssr-china-domain.list"

var DefaultRuleSourceIDs = []string{
	DefaultRuleSourceID,
	"blackmatrix7-china",
	"loyalsoldier-direct",
}

func DefaultPACConfig() PACConfig {
	return PACConfig{
		Enabled:              true,
		EnabledSet:           true,
		Proxy:                "PROXY 127.0.0.1:7890; SOCKS5 127.0.0.1:7890; DIRECT",
		RuleSetID:            DefaultRuleSetID,
		RuleSourceURL:        DefaultPACRuleSourceURL,
		RuleSourceFormat:     "clash-domain",
		RuleRefreshHours:     24,
		DomainKeywords:       []string{},
		DirectDomainSuffixes: []string{},
		DirectCIDRs:          []string{},
	}
}

func NormalizePACConfig(cfg PACConfig) PACConfig {
	defaults := DefaultPACConfig()
	legacySource := cfg.RuleSourceURL == LegacyDefaultPACRuleSourceURL || cfg.RuleSourceURL == LegacyCDNPACRuleSourceURL
	if !cfg.EnabledSet {
		cfg.Enabled = defaults.Enabled
		cfg.EnabledSet = true
	}
	if cfg.Proxy == "" {
		cfg.Proxy = defaults.Proxy
	}
	if cfg.RuleSetID == "" {
		cfg.RuleSetID = defaults.RuleSetID
	}
	if cfg.RuleSourceURL == "" || legacySource {
		cfg.RuleSourceURL = defaults.RuleSourceURL
		if legacySource {
			cfg.CachedDomainSuffixes = nil
			cfg.LastSyncedAt = nil
			cfg.LastSyncStatus = ""
			cfg.LastSyncError = ""
		}
	}
	if cfg.RuleSourceFormat == "" || cfg.RuleSourceURL == defaults.RuleSourceURL {
		cfg.RuleSourceFormat = defaults.RuleSourceFormat
	}
	if cfg.RuleRefreshHours <= 0 {
		cfg.RuleRefreshHours = defaults.RuleRefreshHours
	}
	if cfg.DomainKeywords == nil {
		cfg.DomainKeywords = defaults.DomainKeywords
	}
	if cfg.DirectDomainSuffixes == nil || isLegacyPACDirectDomains(cfg.DirectDomainSuffixes) {
		cfg.DirectDomainSuffixes = defaults.DirectDomainSuffixes
	}
	if cfg.DirectCIDRs == nil {
		cfg.DirectCIDRs = defaults.DirectCIDRs
	}
	cfg.DomainKeywords = NormalizeStringList(cfg.DomainKeywords)
	cfg.DirectDomainSuffixes = NormalizeStringList(cfg.DirectDomainSuffixes)
	cfg.DirectCIDRs = NormalizeStringList(cfg.DirectCIDRs)
	cfg.CachedDomainSuffixes = NormalizeStringList(cfg.CachedDomainSuffixes)
	return cfg
}

func DefaultRuleSource(ownerID string) RuleSource {
	return DefaultRuleSources(ownerID)[0]
}

func DefaultRuleSources(ownerID string) []RuleSource {
	return []RuleSource{
		{
			ID:           DefaultRuleSourceID,
			OwnerUserID:  ownerID,
			Name:         "ACL4SSR 国内域名",
			URL:          DefaultPACRuleSourceURL,
			Format:       "clash-domain",
			RefreshHours: 24,
			LocalPath:    DefaultPACRuleSourceLocalPath,
		},
		{
			ID:           "blackmatrix7-china",
			OwnerUserID:  ownerID,
			Name:         "blackmatrix7 China",
			URL:          "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/China/China.list",
			Format:       "clash-domain",
			RefreshHours: 24,
		},
		{
			ID:           "loyalsoldier-direct",
			OwnerUserID:  ownerID,
			Name:         "Loyalsoldier Direct",
			URL:          "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/direct.txt",
			Format:       "yaml-payload",
			RefreshHours: 24,
		},
	}
}

func DefaultRuleSourceByID(ownerID string, id string) (RuleSource, bool) {
	for _, source := range DefaultRuleSources(ownerID) {
		if source.ID == id {
			return source, true
		}
	}
	return RuleSource{
		OwnerUserID: ownerID,
	}, false
}

func DefaultRuleSet(ownerID string) RuleSet {
	return RuleSet{
		ID:                     DefaultRuleSetID,
		OwnerUserID:            ownerID,
		Name:                   "国内直连",
		SourceIDs:              append([]string{}, DefaultRuleSourceIDs...),
		DomainKeywords:         []string{},
		DirectDomainSuffixes:   []string{},
		ExcludedDomainSuffixes: []string{},
		DirectCIDRs:            []string{},
	}
}

func NormalizeRuleSource(source RuleSource) RuleSource {
	source.ID = normalizeID(source.ID)
	if source.ID == "" {
		source.ID = strings.ToLower(strings.ReplaceAll(source.Name, " ", "-"))
	}
	source.Name = strings.TrimSpace(source.Name)
	if source.Name == "" {
		source.Name = source.ID
	}
	source.URL = strings.TrimSpace(source.URL)
	source.Format = strings.ToLower(strings.TrimSpace(source.Format))
	if source.Format == "" {
		source.Format = "domain-list"
	}
	if source.RefreshHours <= 0 {
		source.RefreshHours = 24
	}
	source.LocalPath = strings.TrimSpace(source.LocalPath)
	source.CachedDomainSuffixes = NormalizeStringList(source.CachedDomainSuffixes)
	return source
}

func NormalizeRuleSet(set RuleSet) RuleSet {
	set.ID = normalizeID(set.ID)
	if set.ID == "" {
		set.ID = strings.ToLower(strings.ReplaceAll(set.Name, " ", "-"))
	}
	set.Name = strings.TrimSpace(set.Name)
	if set.Name == "" {
		set.Name = set.ID
	}
	set.SourceIDs = NormalizeStringList(set.SourceIDs)
	set.DomainKeywords = NormalizeStringList(set.DomainKeywords)
	set.DirectDomainSuffixes = NormalizeStringList(set.DirectDomainSuffixes)
	set.ExcludedDomainSuffixes = NormalizeStringList(set.ExcludedDomainSuffixes)
	set.DirectCIDRs = NormalizeStringList(set.DirectCIDRs)
	set.CachedDomainSuffixes = NormalizeStringList(set.CachedDomainSuffixes)
	return set
}

func normalizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "-", "_", "-", "/", "-", "\\", "-")
	value = replacer.Replace(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

func NormalizeStringList(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func isLegacyPACDirectDomains(values []string) bool {
	legacy := []string{"cn", "中国", "中国互联网络信息中心", "local"}
	normalized := NormalizeStringList(values)
	if len(normalized) != len(legacy) {
		return false
	}
	for i := range legacy {
		if strings.ToLower(normalized[i]) != strings.ToLower(legacy[i]) {
			return false
		}
	}
	return true
}

func PACCacheExpired(cfg PACConfig, now time.Time) bool {
	if cfg.RuleSourceURL == "" {
		return false
	}
	if cfg.LastSyncStatus == "error" && len(cfg.CachedDomainSuffixes) == 0 {
		return true
	}
	if cfg.LastSyncedAt == nil {
		return true
	}
	return now.Sub(*cfg.LastSyncedAt) >= time.Duration(cfg.RuleRefreshHours)*time.Hour
}

func RuleSourceCacheExpired(source RuleSource, now time.Time) bool {
	source = NormalizeRuleSource(source)
	if source.LastSyncStatus == "error" && len(source.CachedDomainSuffixes) == 0 {
		return true
	}
	if source.LastSyncedAt == nil {
		return true
	}
	return now.Sub(*source.LastSyncedAt) >= time.Duration(source.RefreshHours)*time.Hour
}

func MaskURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		if len(raw) <= 18 {
			return strings.Repeat("*", len(raw))
		}
		return raw[:10] + "..." + raw[len(raw)-6:]
	}
	u.User = nil
	if u.RawQuery != "" {
		u.RawQuery = "..."
	}
	if len(u.Path) > 12 {
		u.Path = u.Path[:6] + "..." + u.Path[len(u.Path)-4:]
	}
	return u.String()
}

func SourceToView(source Source, includeURL bool) SourceView {
	view := SourceView{
		ID:              source.ID,
		Name:            source.Name,
		URLMasked:       sourceDisplay(source),
		SourceType:      normalizedSourceType(source),
		FileName:        source.FileName,
		TrafficQuery:    source.TrafficQuery,
		TrafficInfo:     source.TrafficInfo,
		Enabled:         source.Enabled,
		Remark:          source.Remark,
		Tags:            source.Tags,
		LastStatus:      source.LastStatus,
		LastFormat:      source.LastFormat,
		LastNodeCount:   source.LastNodeCount,
		LastError:       source.LastError,
		RefreshProgress: source.RefreshProgress,
		RefreshPercent:  source.RefreshPercent,
		LastRefreshedAt: source.LastRefreshedAt,
		LastSuccessAt:   source.LastSuccessAt,
		NodeStats:       NodeStatsFromNodes(source.CachedNodes),
	}
	if includeURL {
		view.URL = source.URL
		view.FileContent = source.FileContent
		view.Nodes = NodePreviewsFromNodes(source.CachedNodes)
	}
	return view
}

func normalizedSourceType(source Source) string {
	if source.SourceType == "file" || source.FileContent != "" {
		return "file"
	}
	return "url"
}

func sourceDisplay(source Source) string {
	if normalizedSourceType(source) == "file" {
		if source.FileName != "" {
			return "文件：" + source.FileName
		}
		return "文件订阅"
	}
	return MaskURL(source.URL)
}

func NodeStatsFromNodes(nodes []Node) NodeStats {
	stats := NodeStats{Total: len(nodes)}
	for _, node := range nodes {
		if node.Alive == nil {
			stats.Unchecked++
			continue
		}
		if *node.Alive {
			stats.Alive++
		} else {
			stats.Dead++
		}
	}
	return stats
}

func NodePreviewsFromNodes(nodes []Node) []NodePreview {
	out := make([]NodePreview, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, NodePreview{
			Key:            strings.ToLower(node.SourceID + "|" + node.Type + "|" + node.Server + "|" + node.Name),
			Name:           node.Name,
			OriginalName:   originalNodeName(node),
			SourceID:       node.SourceID,
			Source:         node.Source,
			Server:         node.Server,
			Port:           node.Port,
			Region:         node.Region,
			RegionCode:     node.RegionCode,
			ResolvedIP:     node.ResolvedIP,
			ExitIP:         node.ExitIP,
			DelayMS:        node.DelayMS,
			Alive:          node.Alive,
			ExcludedReason: node.ExcludedReason,
			RegionSource:   node.RegionSource,
			ProbeStatus:    node.ProbeStatus,
			ProbeError:     node.ProbeError,
		})
	}
	return out
}

func originalNodeName(node Node) string {
	if value, ok := node.Extra["_original_name"].(string); ok && value != "" {
		return value
	}
	return node.Name
}
