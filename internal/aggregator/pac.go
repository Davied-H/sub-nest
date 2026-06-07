package aggregator

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"sub-nest/internal/domain"
)

func FetchPACRuleSource(ctx context.Context, cfg domain.PACConfig) ([]string, error) {
	cfg = domain.NormalizePACConfig(cfg)
	if strings.TrimSpace(cfg.RuleSourceURL) == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.RuleSourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Sub-Nest/1.0; +https://github.com/Davied-H/sub-nest)")
	req.Header.Set("Accept", "text/plain,*/*;q=0.8")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("规则源返回 HTTP %d", res.StatusCode)
	}
	body := io.LimitReader(res.Body, 8*1024*1024)
	return ParsePACRuleSource(body, cfg.RuleSourceFormat)
}

func LoadRuleSource(source domain.RuleSource) ([]string, error) {
	source = domain.NormalizeRuleSource(source)
	if strings.TrimSpace(source.LocalPath) == "" {
		return nil, os.ErrNotExist
	}
	file, err := os.Open(source.LocalPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ParsePACRuleSource(file, source.Format)
}

func FetchRuleSource(ctx context.Context, source domain.RuleSource) ([]string, error) {
	source = domain.NormalizeRuleSource(source)
	if strings.TrimSpace(source.URL) == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Sub-Nest/1.0; +https://github.com/Davied-H/sub-nest)")
	req.Header.Set("Accept", "text/plain,*/*;q=0.8")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("规则源返回 HTTP %d", res.StatusCode)
	}
	body := io.LimitReader(res.Body, 8*1024*1024)
	return ParsePACRuleSource(body, source.Format)
}

func ParsePACRuleSource(reader io.Reader, format string) ([]string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "dnsmasq"
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	out := []string{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if domainName := parsePACRuleLine(line, format); domainName != "" {
			out = append(out, domainName)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return domain.NormalizeStringList(out), nil
}

func parsePACRuleLine(line string, format string) string {
	switch format {
	case "dnsmasq":
		if strings.HasPrefix(line, "server=/") || strings.HasPrefix(line, "address=/") {
			parts := strings.Split(line, "/")
			if len(parts) >= 3 {
				return normalizeDomainSuffix(parts[1])
			}
		}
	case "clash-domain":
		if domainName, ok := strings.CutPrefix(line, "DOMAIN-SUFFIX,"); ok {
			return normalizeDomainSuffix(domainName)
		}
		if domainName, ok := strings.CutPrefix(line, "DOMAIN,"); ok {
			return normalizeDomainSuffix(domainName)
		}
	case "domain-list":
		line = strings.TrimPrefix(line, "+.")
		line = strings.TrimPrefix(line, ".")
		return normalizeDomainSuffix(line)
	case "yaml-payload":
		if !strings.HasPrefix(line, "-") {
			return ""
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		line = strings.Trim(line, `"'`)
		line = strings.TrimPrefix(line, "+.")
		line = strings.TrimPrefix(line, ".")
		return normalizeDomainSuffix(line)
	}
	return ""
}

func normalizeDomainSuffix(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, ".")
	if value == "" || strings.ContainsAny(value, "/ *") {
		return ""
	}
	return value
}

func BuildPACConfig(cfg domain.PACConfig) domain.PACConfig {
	return domain.NormalizePACConfig(cfg)
}

func PACSyncDue(cfg domain.PACConfig, now time.Time) bool {
	cfg = domain.NormalizePACConfig(cfg)
	return domain.PACCacheExpired(cfg, now)
}

func RuleSourceSyncDue(source domain.RuleSource, now time.Time) bool {
	source = domain.NormalizeRuleSource(source)
	if len(source.CachedDomainSuffixes) == 0 {
		if domains, err := LoadRuleSource(source); err == nil && len(domains) > 0 {
			return true
		}
	}
	return domain.RuleSourceCacheExpired(source, now)
}
