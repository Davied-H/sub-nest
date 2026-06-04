package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"

	"sub-nest/internal/domain"
)

type exitIPResponse struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
}

type ProbeProgressFunc func(done int, total int, node domain.Node)

func probeNodeRegions(nodes []domain.Node, progress ProbeProgressFunc) []domain.Node {
	bin := findMihomoBinary()
	now := time.Now()
	out := make([]domain.Node, len(nodes))
	if len(nodes) == 0 {
		return out
	}
	if bin == "" {
		for index, node := range nodes {
			node.Alive = nil
			node.ProbeStatus = "skipped"
			node.ProbeError = "mihomo/clash core not found"
			node.RegionSource = "geoip"
			node.ProbeChecked = &now
			enrichNodeRegion(&node)
			out[index] = node
			if progress != nil {
				progress(index+1, len(nodes), node)
			}
		}
		return out
	}

	workers := min(6, len(nodes))
	jobs := make(chan int)
	var done atomic.Int32
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				node := nodes[index]
				if err := probeNodeRegion(bin, &node); err != nil {
					alive := false
					node.Alive = &alive
					node.ExcludedReason = "出口测试失败"
					node.ProbeStatus = "failed"
					node.ProbeError = err.Error()
					node.RegionSource = "geoip"
					node.ProbeChecked = &now
					enrichNodeRegion(&node)
				}
				out[index] = node
				if progress != nil {
					progress(int(done.Add(1)), len(nodes), node)
				}
			}
		}()
	}
	for index, node := range nodes {
		_ = node
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	return out
}

func probeNodeRegion(bin string, node *domain.Node) error {
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "subagg-probe-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	httpPort, err := freePort()
	if err != nil {
		return err
	}
	socksPort, err := freePort()
	if err != nil {
		return err
	}
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := writeProbeConfig(configPath, *node, httpPort, socksPort); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, bin, "-d", tmpDir, "-f", configPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	var lastErr error
	for i := 0; i < 8; i++ {
		resp, err := fetchExitIP(client)
		if err == nil {
			info := regionFromCountryCode(resp.Country)
			if info.Group == "" {
				ip := net.ParseIP(resp.IP)
				info = inferRegionFromGeoIP(ip)
				if info.Group == "" {
					info = inferRegionFromIP(ip)
				}
			}
			if info.Group == "" {
				info = regionInfo{"其他节点", "OTHER"}
			}
			now := time.Now()
			alive := true
			node.Alive = &alive
			node.ExcludedReason = ""
			node.Region = info.Group
			node.RegionCode = info.Code
			node.ExitIP = resp.IP
			node.RegionSource = "probe"
			node.ProbeStatus = "ok"
			node.ProbeError = ""
			node.ProbeChecked = &now
			return nil
		}
		lastErr = err
		time.Sleep(900 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = errors.New("exit probe failed")
	}
	return lastErr
}

func writeProbeConfig(path string, node domain.Node, httpPort int, socksPort int) error {
	proxy := map[string]interface{}{
		"name":   "probe",
		"type":   node.Type,
		"server": node.Server,
		"port":   node.Port,
	}
	for key, value := range node.Extra {
		if key == "ps" || key == "add" || key == "port" || key == "user" || key == "_original_name" {
			continue
		}
		proxy[key] = value
	}
	doc := map[string]interface{}{
		"mixed-port":          0,
		"port":                httpPort,
		"socks-port":          socksPort,
		"allow-lan":           false,
		"mode":                "global",
		"log-level":           "silent",
		"external-controller": "127.0.0.1:0",
		"proxies":             []map[string]interface{}{proxy},
		"proxy-groups": []map[string]interface{}{
			{"name": "GLOBAL", "type": "select", "proxies": []string{"probe"}},
		},
		"rules": []string{"MATCH,probe"},
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func fetchExitIP(client *http.Client) (exitIPResponse, error) {
	for _, endpoint := range []string{
		"https://ipinfo.io/json",
		"https://api.country.is/",
		"https://api.ipify.org?format=json",
	} {
		req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
		req.Header.Set("User-Agent", "sub-nest/0.1")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		_ = resp.Body.Close()
		if readErr != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			continue
		}
		ip := stringValue(payload["ip"])
		country := stringValue(payload["country"])
		if ip != "" {
			return exitIPResponse{IP: ip, Country: country}, nil
		}
	}
	return exitIPResponse{}, errors.New("cannot fetch exit ip through proxy")
}

func findMihomoBinary() string {
	for _, candidate := range []string{
		os.Getenv("SUBAGG_MIHOMO_BIN"),
		"mihomo",
		"clash-meta",
		"clash",
	} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if strings.ContainsRune(candidate, filepath.Separator) {
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return candidate
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	return ""
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(portText)
}

func stringValue(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
