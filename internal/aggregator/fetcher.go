package aggregator

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"sub-nest/internal/domain"
)

type Fetcher struct {
	client *http.Client
}

type RefreshProgressFunc func(status string, message string, percent int, nodes []domain.Node)

func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (f *Fetcher) Refresh(source domain.Source) (domain.Source, error) {
	return f.RefreshWithProgress(source, nil)
}

func (f *Fetcher) RefreshWithProgress(source domain.Source, progress RefreshProgressFunc) (domain.Source, error) {
	now := time.Now()
	source.LastRefreshedAt = &now
	source.LastStatus = "refreshing"
	source.RefreshProgress = "拉取订阅源"
	source.RefreshPercent = 5
	if progress != nil {
		progress(source.LastStatus, source.RefreshProgress, source.RefreshPercent, source.CachedNodes)
	}

	if source.SourceType == "file" || source.FileContent != "" {
		return refreshFromBody(source, []byte(source.FileContent), now, progress)
	}

	req, err := http.NewRequest(http.MethodGet, source.URL, nil)
	if err != nil {
		source.LastStatus = "error"
		source.LastError = "订阅链接无效"
		source.RefreshProgress = ""
		return source, err
	}
	req.Header.Set("User-Agent", "sub-nest/0.1")

	resp, err := f.client.Do(req)
	if err != nil {
		source.LastStatus = "error"
		source.LastError = err.Error()
		source.RefreshProgress = ""
		return source, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
		source.LastStatus = "error"
		source.LastError = err.Error()
		source.RefreshProgress = ""
		return source, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		source.LastStatus = "error"
		source.LastError = err.Error()
		source.RefreshProgress = ""
		return source, err
	}
	return refreshFromBody(source, body, now, progress)
}

func refreshFromBody(source domain.Source, body []byte, now time.Time, progress RefreshProgressFunc) (domain.Source, error) {
	if strings.TrimSpace(string(body)) == "" {
		err := fmt.Errorf("empty subscription")
		source.LastStatus = "error"
		source.LastError = err.Error()
		source.RefreshProgress = ""
		return source, err
	}
	parsed, err := ParseSubscription(body, source.ID, source.Name)
	if err != nil {
		source.LastStatus = "error"
		source.LastError = err.Error()
		source.RefreshProgress = ""
		return source, err
	}
	source.LastFormat = parsed.Format
	source.LastNodeCount = len(parsed.Nodes)
	source.RefreshProgress = fmt.Sprintf("解析到 %d 个节点，准备测试出口", len(parsed.Nodes))
	source.RefreshPercent = 20
	if progress != nil {
		progress("refreshing", source.RefreshProgress, source.RefreshPercent, parsed.Nodes)
	}
	probedNodes := probeNodeRegions(parsed.Nodes, func(done int, total int, node domain.Node) {
		if progress != nil {
			percent := 20
			if total > 0 {
				percent = 20 + done*75/total
			}
			progress("refreshing", fmt.Sprintf("测试出口 %d/%d：%s", done, total, node.Name), percent, parsed.Nodes)
		}
	})
	probedNodes = SortNodesByDelay(probedNodes)
	source.LastStatus = "ok"
	source.LastError = ""
	source.RefreshProgress = ""
	source.RefreshPercent = 100
	source.LastSuccessAt = &now
	source.CachedNodes = probedNodes
	return source, nil
}
