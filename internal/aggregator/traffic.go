package aggregator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"sub-nest/internal/domain"
)

const trafficQueryLimit = 2 * 1024 * 1024
const trafficDebugPreviewLimit = 2048

func (f *Fetcher) QueryTraffic(source domain.Source) (domain.TrafficInfo, error) {
	cfg := normalizeTrafficQuery(source)
	now := time.Now()
	info := source.TrafficInfo
	info.LastCheckedAt = &now
	info.LastStatus = "checking"
	info.LastError = ""
	info.Debug = nil

	if cfg.Mode == "" || cfg.Mode == "disabled" {
		info.LastStatus = ""
		return info, nil
	}

	var err error
	switch cfg.Mode {
	case "subscription-header":
		info, err = f.queryTrafficFromSubscriptionHeader(source, info)
	case "subscription-body-regex":
		info, err = f.queryTrafficFromSubscriptionBody(source, info)
	case "custom-http":
		info, err = f.queryTrafficFromCustomHTTP(cfg, info)
	case "manual":
		info.LastStatus = "ok"
	default:
		err = fmt.Errorf("unsupported traffic query mode %q", cfg.Mode)
	}
	if err != nil {
		info.LastStatus = "error"
		info.LastError = err.Error()
		return info, err
	}
	info.LastStatus = "ok"
	info.LastError = ""
	if info.TotalBytes > 0 && info.RemainingBytes == 0 {
		used := info.UploadBytes + info.DownloadBytes
		if info.TotalBytes > used {
			info.RemainingBytes = info.TotalBytes - used
		}
	}
	return info, nil
}

func normalizeTrafficQuery(source domain.Source) domain.TrafficQueryConfig {
	cfg := source.TrafficQuery
	cfg.Mode = strings.TrimSpace(cfg.Mode)
	if cfg.Mode == "" {
		cfg.Mode = "disabled"
	}
	if cfg.Mode == "auto" {
		cfg.Mode = "subscription-header"
	}
	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.Method = strings.ToUpper(strings.TrimSpace(cfg.Method))
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	cfg.Parser.Type = strings.TrimSpace(cfg.Parser.Type)
	if cfg.Parser.Type == "" {
		if cfg.Mode == "subscription-body-regex" {
			cfg.Parser.Type = "regex"
		} else {
			cfg.Parser.Type = "json-path"
		}
	}
	return cfg
}

func (f *Fetcher) queryTrafficFromSubscriptionHeader(source domain.Source, info domain.TrafficInfo) (domain.TrafficInfo, error) {
	if source.SourceType == "file" || strings.TrimSpace(source.URL) == "" {
		return info, errors.New("文件订阅不支持自动读取响应头")
	}
	resp, err := f.doTrafficRequest(http.MethodGet, source.URL, nil, nil)
	info.Debug = trafficDebugFromRequest(http.MethodGet, source.URL, "subscription-header")
	if err != nil {
		info.Debug.Status = err.Error()
		return info, err
	}
	defer resp.Body.Close()
	fillTrafficDebugResponse(info.Debug, resp, nil)
	if err := checkTrafficStatus(resp); err != nil {
		return info, err
	}
	return parseSubscriptionUserInfo(resp.Header.Get("Subscription-Userinfo"), info)
}

func (f *Fetcher) queryTrafficFromSubscriptionBody(source domain.Source, info domain.TrafficInfo) (domain.TrafficInfo, error) {
	if source.SourceType == "file" || strings.TrimSpace(source.URL) == "" {
		return info, errors.New("文件订阅不支持远程正文解析")
	}
	resp, err := f.doTrafficRequest(http.MethodGet, source.URL, nil, nil)
	info.Debug = trafficDebugFromRequest(http.MethodGet, source.URL, "regex")
	if err != nil {
		info.Debug.Status = err.Error()
		return info, err
	}
	defer resp.Body.Close()
	if err := checkTrafficStatus(resp); err != nil {
		fillTrafficDebugResponse(info.Debug, resp, nil)
		return info, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, trafficQueryLimit))
	fillTrafficDebugResponse(info.Debug, resp, body)
	if err != nil {
		return info, err
	}
	return parseTrafficByRegex(string(body), source.TrafficQuery.Parser, info)
}

func (f *Fetcher) queryTrafficFromCustomHTTP(cfg domain.TrafficQueryConfig, info domain.TrafficInfo) (domain.TrafficInfo, error) {
	info.Debug = trafficDebugFromRequest(cfg.Method, cfg.URL, cfg.Parser.Type)
	if cfg.URL == "" {
		return info, errors.New("自定义查询 URL 不能为空")
	}
	resp, err := f.doTrafficRequest(cfg.Method, cfg.URL, cfg.Headers, strings.NewReader(cfg.Body))
	if err != nil {
		info.Debug.Status = err.Error()
		return info, err
	}
	defer resp.Body.Close()
	if err := checkTrafficStatus(resp); err != nil {
		fillTrafficDebugResponse(info.Debug, resp, nil)
		return info, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, trafficQueryLimit))
	fillTrafficDebugResponse(info.Debug, resp, body)
	if err != nil {
		return info, err
	}
	switch cfg.Parser.Type {
	case "regex":
		return parseTrafficByRegex(string(body), cfg.Parser, info)
	case "subscription-header":
		return parseSubscriptionUserInfo(resp.Header.Get("Subscription-Userinfo"), info)
	default:
		return parseTrafficByJSONPath(body, cfg.Parser, info)
	}
}

func (f *Fetcher) doTrafficRequest(method string, url string, headers map[string]string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sub-nest/0.3")
	for key, value := range headers {
		key = strings.TrimSpace(key)
		if key != "" {
			req.Header.Set(key, value)
		}
	}
	if method != http.MethodGet && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return f.client.Do(req)
}

func checkTrafficStatus(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("traffic query returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func trafficDebugFromRequest(method string, url string, parserType string) *domain.TrafficDebug {
	return &domain.TrafficDebug{
		Method:     method,
		URL:        url,
		ParserType: parserType,
	}
}

func fillTrafficDebugResponse(debug *domain.TrafficDebug, resp *http.Response, body []byte) {
	if debug == nil || resp == nil {
		return
	}
	debug.Status = resp.Status
	debug.StatusCode = resp.StatusCode
	debug.ContentType = resp.Header.Get("Content-Type")
	if body != nil {
		debug.BodyPreview = previewTrafficBody(body)
	}
}

func previewTrafficBody(body []byte) string {
	value := strings.TrimSpace(string(body))
	if len(value) <= trafficDebugPreviewLimit {
		return value
	}
	return value[:trafficDebugPreviewLimit] + "...(truncated)"
}

func parseSubscriptionUserInfo(value string, info domain.TrafficInfo) (domain.TrafficInfo, error) {
	if info.Debug != nil {
		info.Debug.Header = value
	}
	if strings.TrimSpace(value) == "" {
		return info, errors.New("响应头 subscription-userinfo 为空")
	}
	for _, part := range strings.Split(value, ";") {
		key, raw, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "upload":
			info.UploadBytes = n
		case "download":
			info.DownloadBytes = n
		case "total":
			info.TotalBytes = n
		case "expire":
			if n > 0 {
				t := time.Unix(n, 0)
				info.ExpireAt = &t
			}
		}
	}
	if info.UploadBytes == 0 && info.DownloadBytes == 0 && info.TotalBytes == 0 && info.ExpireAt == nil {
		return info, errors.New("响应头 subscription-userinfo 未解析到流量信息")
	}
	return info, nil
}

func parseTrafficByRegex(body string, parser domain.TrafficParser, info domain.TrafficInfo) (domain.TrafficInfo, error) {
	var found bool
	var err error
	if info.UploadBytes, found, err = parseRegexBytesDebug(body, "已上传", parser.Upload, info.Debug, found); err != nil {
		return info, fmt.Errorf("解析已上传失败: %w", err)
	}
	if info.DownloadBytes, found, err = parseRegexBytesMergeDebug(body, "已下载", parser.Download, info.DownloadBytes, info.Debug, found); err != nil {
		return info, fmt.Errorf("解析已下载失败: %w", err)
	}
	if info.TotalBytes, found, err = parseRegexBytesMergeDebug(body, "总量", parser.Total, info.TotalBytes, info.Debug, found); err != nil {
		return info, fmt.Errorf("解析总量失败: %w", err)
	}
	if info.RemainingBytes, found, err = parseRegexBytesMergeDebug(body, "剩余", parser.Remaining, info.RemainingBytes, info.Debug, found); err != nil {
		return info, fmt.Errorf("解析剩余失败: %w", err)
	}
	if parser.Expire != "" {
		if value, ok := firstRegexMatch(body, parser.Expire); ok {
			appendTrafficPathDebug(info.Debug, "到期", parser.Expire, true, value, nil)
			if t, err := parseTrafficTime(value); err == nil {
				info.ExpireAt = &t
				found = true
			} else {
				return info, fmt.Errorf("解析到期时间失败: %w", err)
			}
		} else {
			appendTrafficPathDebug(info.Debug, "到期", parser.Expire, false, "", nil)
		}
	}
	if !found {
		return info, errors.New("未匹配到流量信息")
	}
	return info, nil
}

func parseRegexBytesMergeDebug(body string, label string, pattern string, current int64, debug *domain.TrafficDebug, found bool) (int64, bool, error) {
	value, ok, err := parseRegexBytesWithValue(body, pattern)
	if err != nil {
		appendTrafficPathDebug(debug, label, pattern, false, "", err)
		return current, found, err
	}
	if !ok {
		appendTrafficPathDebug(debug, label, pattern, false, "", nil)
		return current, found, nil
	}
	appendTrafficPathDebug(debug, label, pattern, true, strconv.FormatInt(value, 10), nil)
	return value, true, nil
}

func parseRegexBytesDebug(body string, label string, pattern string, debug *domain.TrafficDebug, found bool) (int64, bool, error) {
	return parseRegexBytesMergeDebug(body, label, pattern, 0, debug, found)
}

func parseRegexBytesMerge(body string, pattern string, current int64, found bool) (int64, bool, error) {
	value, ok, err := parseRegexBytes(body, pattern)
	if err != nil || !ok {
		return current, found, err
	}
	return value, true, nil
}

func parseRegexBytes(body string, pattern string) (int64, bool, error) {
	return parseRegexBytesWithValue(body, pattern)
}

func parseRegexBytesWithValue(body string, pattern string) (int64, bool, error) {
	if strings.TrimSpace(pattern) == "" {
		return 0, false, nil
	}
	value, ok := firstRegexMatch(body, pattern)
	if !ok {
		return 0, false, nil
	}
	n, err := parseTrafficSize(value)
	return n, err == nil, err
}

func firstRegexMatch(body string, pattern string) (string, bool) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", false
	}
	match := re.FindStringSubmatch(body)
	if len(match) == 0 {
		return "", false
	}
	if len(match) > 1 {
		return strings.TrimSpace(match[1]), true
	}
	return strings.TrimSpace(match[0]), true
}

func parseTrafficByJSONPath(body []byte, parser domain.TrafficParser, info domain.TrafficInfo) (domain.TrafficInfo, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var data interface{}
	if err := decoder.Decode(&data); err != nil {
		return info, fmt.Errorf("响应不是有效 JSON: %w", err)
	}
	var found bool
	var err error
	if info.UploadBytes, found, err = jsonPathBytesDebug(data, "已上传", parser.Upload, info.Debug, found); err != nil {
		return info, fmt.Errorf("解析已上传失败: %w", err)
	}
	if info.DownloadBytes, found, err = jsonPathBytesMergeDebug(data, "已下载", parser.Download, info.DownloadBytes, info.Debug, found); err != nil {
		return info, fmt.Errorf("解析已下载失败: %w", err)
	}
	if info.TotalBytes, found, err = jsonPathBytesMergeDebug(data, "总量", parser.Total, info.TotalBytes, info.Debug, found); err != nil {
		return info, fmt.Errorf("解析总量失败: %w", err)
	}
	if info.RemainingBytes, found, err = jsonPathBytesMergeDebug(data, "剩余", parser.Remaining, info.RemainingBytes, info.Debug, found); err != nil {
		return info, fmt.Errorf("解析剩余失败: %w", err)
	}
	if parser.Expire != "" {
		if value, ok := jsonPathValue(data, parser.Expire); ok {
			appendTrafficPathDebug(info.Debug, "到期", parser.Expire, true, fmt.Sprint(value), nil)
			if t, err := parseTrafficTime(fmt.Sprint(value)); err == nil {
				info.ExpireAt = &t
				found = true
			} else {
				return info, fmt.Errorf("解析到期时间失败: %w", err)
			}
		} else {
			appendTrafficPathDebug(info.Debug, "到期", parser.Expire, false, "", nil)
		}
	}
	if !found {
		return info, errors.New("未解析到流量信息")
	}
	return info, nil
}

func jsonPathBytesMergeDebug(data interface{}, label string, path string, current int64, debug *domain.TrafficDebug, found bool) (int64, bool, error) {
	value, ok := jsonPathValue(data, path)
	if strings.TrimSpace(path) == "" {
		return current, found, nil
	}
	if !ok {
		appendTrafficPathDebug(debug, label, path, false, "", nil)
		return current, found, nil
	}
	n, err := parseTrafficSize(fmt.Sprint(value))
	appendTrafficPathDebug(debug, label, path, err == nil, fmt.Sprint(value), err)
	return n, err == nil || found, err
}

func jsonPathBytesDebug(data interface{}, label string, path string, debug *domain.TrafficDebug, found bool) (int64, bool, error) {
	return jsonPathBytesMergeDebug(data, label, path, 0, debug, found)
}

func jsonPathBytesMerge(data interface{}, path string, current int64, found bool) (int64, bool, error) {
	value, ok, err := jsonPathBytes(data, path)
	if err != nil || !ok {
		return current, found, err
	}
	return value, true, nil
}

func jsonPathBytes(data interface{}, path string) (int64, bool, error) {
	if strings.TrimSpace(path) == "" {
		return 0, false, nil
	}
	value, ok := jsonPathValue(data, path)
	if !ok {
		return 0, false, nil
	}
	n, err := parseTrafficSize(fmt.Sprint(value))
	return n, err == nil, err
}

func jsonPathValue(data interface{}, path string) (interface{}, bool) {
	path = strings.TrimSpace(strings.TrimPrefix(path, "$."))
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return data, true
	}
	current := data
	for _, part := range strings.Split(path, ".") {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func appendTrafficPathDebug(debug *domain.TrafficDebug, label string, path string, found bool, value string, err error) {
	if debug == nil || strings.TrimSpace(path) == "" {
		return
	}
	item := domain.TrafficPathDebug{
		Label: label,
		Path:  path,
		Found: found,
		Value: previewTrafficDebugValue(value),
	}
	if err != nil {
		item.Error = err.Error()
	}
	debug.Paths = append(debug.Paths, item)
}

func previewTrafficDebugValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 160 {
		return value
	}
	return value[:160] + "...(truncated)"
}

func parseTrafficSize(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("空数值")
	}
	re := regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*([kmgtp]?i?b?)?`)
	match := re.FindStringSubmatch(value)
	if len(match) == 0 {
		return 0, fmt.Errorf("无法解析流量大小 %q", value)
	}
	n, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, err
	}
	unit := strings.ToLower(match[2])
	multiplier := float64(1)
	switch unit {
	case "k", "kb", "kib":
		multiplier = 1024
	case "m", "mb", "mib":
		multiplier = 1024 * 1024
	case "g", "gb", "gib":
		multiplier = 1024 * 1024 * 1024
	case "t", "tb", "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "p", "pb", "pib":
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
	}
	return int64(n * multiplier), nil
}

func parseTrafficTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("空时间")
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		if n > 1_000_000_000_000 {
			return time.UnixMilli(n), nil
		}
		return time.Unix(n, 0), nil
	}
	layouts := []string{time.RFC3339, "2006-01-02", "2006/01/02", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析时间 %q", value)
}
