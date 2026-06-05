package domain

import (
	"net/url"
	"strings"
)

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

func originalNodeName(node Node) string {
	if value, ok := node.Extra["_original_name"].(string); ok && value != "" {
		return value
	}
	return node.Name
}
