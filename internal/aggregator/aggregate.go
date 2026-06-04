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
	named := applyOutputNames(renamed, output.NodeNameOverrides)
	groups, regionCounts := groupNodes(named)

	return Result{
		Nodes:             named,
		RegionCounts:      regionCounts,
		Groups:            groups,
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
		Nodes:             previewNodes(result.Nodes),
		ExcludedNodes:     previewNodes(result.ExcludedNodes),
		GeneratedAt:       time.Now(),
		UsedCachedSources: result.UsedCachedSources,
	}
}

func Render(output domain.Output, result Result) ([]byte, string, error) {
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
				{"name": "节点选择", "type": "select", "proxies": groupNames(result.Groups, true)},
				{"name": "自动选择", "type": "url-test", "proxies": allNodeNames(result.Nodes), "url": "http://www.gstatic.com/generate_204", "interval": 300},
				{"name": "故障切换", "type": "fallback", "proxies": allNodeNames(result.Nodes), "url": "http://www.gstatic.com/generate_204", "interval": 300},
			},
			"rules": []string{"MATCH,节点选择"},
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
		data, err := yaml.Marshal(doc)
		if err != nil {
			return nil, "", err
		}
		return data, "application/x-yaml; charset=utf-8", nil
	}
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
		sort.Strings(buckets[name])
		groups = append(groups, domain.GroupPreview{Name: name, Nodes: buckets[name]})
		counts[name] = len(buckets[name])
	}
	return groups, counts
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
			Server:         node.Server,
			Port:           node.Port,
			Region:         node.Region,
			RegionCode:     node.RegionCode,
			ResolvedIP:     node.ResolvedIP,
			ExitIP:         node.ExitIP,
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
