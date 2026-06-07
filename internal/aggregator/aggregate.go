package aggregator

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"sub-nest/internal/domain"
)

type Result struct {
	Nodes             []domain.Node
	RegionCounts      map[string]int
	Groups            []domain.GroupPreview
	SourceGroups      []domain.GroupPreview
	DuplicateCount    int
	FilteredCount     int
	UnavailableCount  int
	ExcludedNodes     []domain.Node
	FailedSources     []domain.SourceView
	UsedCachedSources int
}

func Build(cfg domain.Config, output domain.Output) Result {
	sourceMap := map[string]domain.Source{}
	for _, source := range cfg.Sources {
		sourceMap[source.ID] = source
	}

	nodes := []domain.Node{}
	failed := []domain.SourceView{}
	usedCached := 0
	for _, id := range output.SourceIDs {
		source, ok := sourceMap[id]
		if !ok || !source.Enabled {
			continue
		}
		if source.LastStatus != "ok" && len(source.CachedNodes) == 0 {
			failed = append(failed, domain.SourceToView(source, false))
			continue
		}
		if source.LastStatus != "ok" {
			usedCached++
		}
		nodes = append(nodes, source.CachedNodes...)
	}

	deduped, dupCount := dedupe(nodes)
	available, unavailableNodes := splitUnavailableNodes(deduped)
	unavailableCount := len(unavailableNodes)
	filtered, filteredCount := filterNodes(available, output.Filter)
	renamed := renameNodes(filtered, output.RenameRules)
	ordered := SortNodesByDelay(renamed)
	named := applyOutputNames(ordered, output.NodeNameOverrides)
	groups, regionCounts := groupNodes(named)
	sourceGroups := groupNodesBySource(named, output.SourceIDs, sourceMap)

	return Result{
		Nodes:             named,
		RegionCounts:      regionCounts,
		Groups:            groups,
		SourceGroups:      sourceGroups,
		DuplicateCount:    dupCount,
		FilteredCount:     filteredCount,
		UnavailableCount:  unavailableCount,
		ExcludedNodes:     unavailableNodes,
		FailedSources:     failed,
		UsedCachedSources: usedCached,
	}
}

func Preview(cfg domain.Config, output domain.Output) domain.Preview {
	result := Build(cfg, output)
	return domain.Preview{
		OutputID:          output.ID,
		Slug:              output.Slug,
		NodeCount:         len(result.Nodes),
		DuplicateCount:    result.DuplicateCount,
		FilteredCount:     result.FilteredCount,
		UnavailableCount:  result.UnavailableCount,
		FailedSources:     result.FailedSources,
		RegionCounts:      result.RegionCounts,
		Groups:            result.Groups,
		SourceGroups:      result.SourceGroups,
		Nodes:             previewNodes(result.Nodes),
		ExcludedNodes:     previewNodes(result.ExcludedNodes),
		GeneratedAt:       time.Now(),
		UsedCachedSources: result.UsedCachedSources,
	}
}

func Render(output domain.Output, result Result, resolvedConfigs ...domain.PACConfig) ([]byte, string, error) {
	resolvedPAC := output.PAC
	if len(resolvedConfigs) > 0 {
		resolvedPAC = resolvedConfigs[0]
	}
	switch strings.ToLower(output.Format) {
	case "base64":
		lines := make([]string, 0, len(result.Nodes))
		for _, node := range result.Nodes {
			if node.Raw != "" {
				lines = append(lines, node.Raw)
			}
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n")))
		return []byte(encoded), "text/plain; charset=utf-8", nil
	default:
		doc := map[string]interface{}{
			"mixed-port": 7890,
			"allow-lan":  false,
			"mode":       "rule",
			"log-level":  "info",
			"proxies":    proxiesForYAML(result.Nodes),
			"proxy-groups": []map[string]interface{}{
				{"name": "节点选择", "type": "select", "proxies": groupNames(append(result.Groups, result.SourceGroups...), true)},
				{"name": "自动选择", "type": "url-test", "proxies": allNodeNames(result.Nodes), "url": "http://www.gstatic.com/generate_204", "interval": 300},
				{"name": "故障切换", "type": "fallback", "proxies": allNodeNames(result.Nodes), "url": "http://www.gstatic.com/generate_204", "interval": 300},
			},
			"rules": clashDirectRules(resolvedPAC),
		}
		for _, group := range result.Groups {
			if strings.HasSuffix(group.Name, "节点") && len(group.Nodes) > 0 {
				doc["proxy-groups"] = append(doc["proxy-groups"].([]map[string]interface{}), map[string]interface{}{
					"name":    group.Name,
					"type":    "select",
					"proxies": group.Nodes,
				})
			}
		}
		for _, group := range result.SourceGroups {
			if len(group.Nodes) == 0 {
				continue
			}
			doc["proxy-groups"] = append(doc["proxy-groups"].([]map[string]interface{}), map[string]interface{}{
				"name":    group.Name,
				"type":    "select",
				"proxies": group.Nodes,
			})
		}
		data, err := yaml.Marshal(doc)
		if err != nil {
			return nil, "", err
		}
		return data, "application/x-yaml; charset=utf-8", nil
	}
}

func RenderPAC(output domain.Output, result Result, resolvedConfigs ...domain.PACConfig) ([]byte, string, error) {
	resolvedPAC := output.PAC
	if len(resolvedConfigs) > 0 {
		resolvedPAC = resolvedConfigs[0]
	}
	pacConfig := domain.NormalizePACConfig(resolvedPAC)
	proxy := pacConfig.Proxy
	if len(result.Nodes) == 0 {
		proxy = "DIRECT"
	}
	pac := fmt.Sprintf(`// Generated by Sub Nest for %s.
var directDomainSuffixes = %s;

function FindProxyForURL(url, host) {
  host = host.toLowerCase();

  if (isPlainHostName(host) ||
      dnsDomainIs(host, ".local") ||
      shExpMatch(host, "*.local") ||
      isInNet(host, "0.0.0.0", "255.0.0.0") ||
      isInNet(host, "10.0.0.0", "255.0.0.0") ||
      isInNet(host, "127.0.0.0", "255.0.0.0") ||
      isInNet(host, "169.254.0.0", "255.255.0.0") ||
      isInNet(host, "172.16.0.0", "255.240.0.0") ||
      isInNet(host, "192.168.0.0", "255.255.0.0")) {
    return "DIRECT";
  }

  for (var i = 0; i < directDomainSuffixes.length; i++) {
    var suffix = directDomainSuffixes[i];
    if (host === suffix || dnsDomainIs(host, "." + suffix)) {
      return "DIRECT";
    }
  }

  return %q;
}
`, output.Slug, jsonStringArray(pacDomainSuffixes(pacConfig)), proxy)
	return []byte(pac), "application/x-ns-proxy-autoconfig; charset=utf-8", nil
}

func clashDirectRules(cfg domain.PACConfig) []string {
	cfg = domain.NormalizePACConfig(cfg)
	rules := []string{}
	for _, keyword := range cfg.DomainKeywords {
		rules = append(rules, "DOMAIN-KEYWORD,"+keyword+",节点选择")
	}
	for _, suffix := range pacDomainSuffixes(cfg) {
		if suffix == "local" {
			continue
		}
		rules = append(rules, "DOMAIN-SUFFIX,"+suffix+",DIRECT")
	}
	for _, cidr := range cfg.DirectCIDRs {
		rules = append(rules, "IP-CIDR,"+cidr+",DIRECT,no-resolve")
	}
	rules = append(rules, "GEOIP,CN,DIRECT", "MATCH,节点选择")
	return rules
}

func pacDomainSuffixes(cfg domain.PACConfig) []string {
	merged := append([]string{}, cfg.DirectDomainSuffixes...)
	merged = append(merged, cfg.CachedDomainSuffixes...)
	return domain.NormalizeStringList(merged)
}

func jsonStringArray(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

func dedupe(nodes []domain.Node) ([]domain.Node, int) {
	seen := map[string]bool{}
	out := make([]domain.Node, 0, len(nodes))
	dup := 0
	for _, node := range nodes {
		key := strings.ToLower(fmt.Sprintf("%s|%s|%d|%s", node.Name, node.Server, node.Port, node.Type))
		if node.Server != "" {
			key = strings.ToLower(fmt.Sprintf("%s|%d|%s", node.Server, node.Port, node.Type))
		}
		if seen[key] {
			dup++
			continue
		}
		seen[key] = true
		out = append(out, node)
	}
	return out, dup
}

func filterNodes(nodes []domain.Node, rules domain.FilterRules) ([]domain.Node, int) {
	include := lowerTrimmed(rules.IncludeKeywords)
	exclude := lowerTrimmed(rules.ExcludeKeywords)
	var re *regexp.Regexp
	if strings.TrimSpace(rules.Regex) != "" {
		if compiled, err := regexp.Compile(rules.Regex); err == nil {
			re = compiled
		}
	}
	out := make([]domain.Node, 0, len(nodes))
	dropped := 0
	for _, node := range nodes {
		name := strings.ToLower(node.Name)
		if len(include) > 0 && !containsAny(name, include) {
			dropped++
			continue
		}
		if containsAny(name, exclude) {
			dropped++
			continue
		}
		if re != nil && !re.MatchString(node.Name) {
			dropped++
			continue
		}
		out = append(out, node)
	}
	return out, dropped
}

func splitUnavailableNodes(nodes []domain.Node) ([]domain.Node, []domain.Node) {
	out := make([]domain.Node, 0, len(nodes))
	excluded := []domain.Node{}
	for _, node := range nodes {
		if node.Alive != nil && !*node.Alive {
			excluded = append(excluded, node)
			continue
		}
		out = append(out, node)
	}
	return out, excluded
}

func renameNodes(nodes []domain.Node, rules []domain.RenameRule) []domain.Node {
	out := make([]domain.Node, 0, len(nodes))
	for _, node := range nodes {
		name := node.Name
		for _, rule := range rules {
			if strings.TrimSpace(rule.Pattern) == "" {
				continue
			}
			if re, err := regexp.Compile(rule.Pattern); err == nil {
				name = re.ReplaceAllString(name, rule.Replacement)
				continue
			}
			name = strings.ReplaceAll(name, rule.Pattern, rule.Replacement)
		}
		node.Name = strings.TrimSpace(name)
		out = append(out, node)
	}
	return out
}

func applyOutputNames(nodes []domain.Node, overrides map[string]string) []domain.Node {
	out := make([]domain.Node, 0, len(nodes))
	counters := map[string]int{}
	for _, node := range nodes {
		if node.Region == "" {
			enrichNodeRegion(&node)
		}
		originalName := node.Name
		if override := strings.TrimSpace(overrides[nodeKey(node)]); override != "" {
			node.Name = override
			out = append(out, node)
			continue
		}
		group := node.Region
		if group == "" {
			group = "其他节点"
		}
		counters[group]++
		node.Name = fmt.Sprintf("%s %02d", strings.TrimSuffix(group, "节点"), counters[group])
		if node.Name == "其他 00" {
			node.Name = originalName
		}
		out = append(out, node)
	}
	return out
}

func groupNodes(nodes []domain.Node) ([]domain.GroupPreview, map[string]int) {
	buckets := map[string][]string{
		"香港节点":  {},
		"台湾节点":  {},
		"日本节点":  {},
		"新加坡节点": {},
		"美国节点":  {},
		"荷兰节点":  {},
		"其他节点":  {},
	}
	for _, node := range nodes {
		group := node.Region
		if group == "" {
			enrichNodeRegion(&node)
			group = node.Region
		}
		if group == "" {
			group = detectRegion(node.Name, node.Server)
		}
		buckets[group] = append(buckets[group], node.Name)
	}
	groups := make([]domain.GroupPreview, 0, len(regionOrder))
	counts := map[string]int{}
	for _, name := range regionOrder {
		groups = append(groups, domain.GroupPreview{Name: name, Nodes: buckets[name]})
		counts[name] = len(buckets[name])
	}
	return groups, counts
}

func groupNodesBySource(nodes []domain.Node, sourceIDs []string, sourceMap map[string]domain.Source) []domain.GroupPreview {
	buckets := map[string][]string{}
	namesByID := map[string]string{}
	usedNames := map[string]int{}
	groups := make([]domain.GroupPreview, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		source, ok := sourceMap[id]
		if !ok || !source.Enabled {
			continue
		}
		name := sourceGroupName(source.Name)
		if usedNames[name] > 0 {
			name = fmt.Sprintf("%s #%d", name, usedNames[name]+1)
		}
		usedNames[sourceGroupName(source.Name)]++
		namesByID[id] = name
		buckets[name] = []string{}
		groups = append(groups, domain.GroupPreview{Name: name})
	}
	for _, node := range nodes {
		name := namesByID[node.SourceID]
		if name == "" {
			name = sourceGroupName(node.Source)
		}
		buckets[name] = append(buckets[name], node.Name)
	}
	for i := range groups {
		groups[i].Nodes = buckets[groups[i].Name]
	}
	return groups
}

func sourceGroupName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未知订阅"
	}
	return "来源 / " + name
}

func SortNodesByDelay(nodes []domain.Node) []domain.Node {
	out := append([]domain.Node(nil), nodes...)
	sort.SliceStable(out, func(i, j int) bool {
		return compareNodesByDelay(out[i], out[j]) < 0
	})
	return out
}

func compareNodesByDelay(a domain.Node, b domain.Node) int {
	if aliveRank(a) != aliveRank(b) {
		return aliveRank(a) - aliveRank(b)
	}
	if delayRank(a) != delayRank(b) {
		return delayRank(a) - delayRank(b)
	}
	if a.DelayMS > 0 && b.DelayMS > 0 && a.DelayMS != b.DelayMS {
		return a.DelayMS - b.DelayMS
	}
	if a.Region != b.Region {
		return strings.Compare(a.Region, b.Region)
	}
	if a.Name != b.Name {
		return strings.Compare(a.Name, b.Name)
	}
	if a.Server != b.Server {
		return strings.Compare(a.Server, b.Server)
	}
	return a.Port - b.Port
}

func aliveRank(node domain.Node) int {
	if node.Alive == nil {
		return 1
	}
	if *node.Alive {
		return 0
	}
	return 2
}

func delayRank(node domain.Node) int {
	if node.DelayMS > 0 {
		return 0
	}
	return 1
}

func detectRegion(values ...string) string {
	for _, value := range values {
		if info := inferRegionFromText(value); info.Group != "" {
			return info.Group
		}
	}
	return "其他节点"
}

func proxiesForYAML(nodes []domain.Node) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(nodes))
	for _, node := range nodes {
		proxy := map[string]interface{}{
			"name":   node.Name,
			"type":   node.Type,
			"server": node.Server,
			"port":   node.Port,
		}
		for key, value := range node.Extra {
			if key == "ps" || key == "add" || key == "user" || key == "_original_name" {
				continue
			}
			proxy[key] = value
		}
		out = append(out, proxy)
	}
	return out
}

func allNodeNames(nodes []domain.Node) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	return names
}

func groupNames(groups []domain.GroupPreview, includeAuto bool) []string {
	names := []string{}
	if includeAuto {
		names = append(names, "自动选择", "故障切换")
	}
	for _, group := range groups {
		if len(group.Nodes) > 0 {
			names = append(names, group.Name)
		}
	}
	return names
}

func lowerTrimmed(values []string) []string {
	out := []string{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func previewNodes(nodes []domain.Node) []domain.NodePreview {
	out := make([]domain.NodePreview, 0, len(nodes))
	for _, node := range nodes {
		if node.Region == "" {
			enrichNodeRegion(&node)
		}
		out = append(out, domain.NodePreview{
			Key:            nodeKey(node),
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

func nodeKey(node domain.Node) string {
	return strings.ToLower(fmt.Sprintf("%s|%s|%s|%d|%s", node.SourceID, node.Type, node.Server, node.Port, originalNodeName(node)))
}

func originalNodeName(node domain.Node) string {
	if value, ok := node.Extra["_original_name"].(string); ok && value != "" {
		return value
	}
	return node.Name
}
