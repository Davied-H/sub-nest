package aggregator

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"

	"sub-nest/internal/domain"
)

type regionInfo struct {
	Group string
	Code  string
}

type cidrRegion struct {
	network *net.IPNet
	info    regionInfo
}

var regionOrder = []string{"香港节点", "台湾节点", "日本节点", "新加坡节点", "美国节点", "荷兰节点", "其他节点"}

var domainRegionHints = []struct {
	key  string
	info regionInfo
}{
	{"hongkong", regionInfo{"香港节点", "HK"}},
	{"hong-kong", regionInfo{"香港节点", "HK"}},
	{".hk", regionInfo{"香港节点", "HK"}},
	{"-hk", regionInfo{"香港节点", "HK"}},
	{"hk-", regionInfo{"香港节点", "HK"}},
	{"taiwan", regionInfo{"台湾节点", "TW"}},
	{".tw", regionInfo{"台湾节点", "TW"}},
	{"-tw", regionInfo{"台湾节点", "TW"}},
	{"tw-", regionInfo{"台湾节点", "TW"}},
	{"japan", regionInfo{"日本节点", "JP"}},
	{"tokyo", regionInfo{"日本节点", "JP"}},
	{".jp", regionInfo{"日本节点", "JP"}},
	{"-jp", regionInfo{"日本节点", "JP"}},
	{"jp-", regionInfo{"日本节点", "JP"}},
	{"singapore", regionInfo{"新加坡节点", "SG"}},
	{".sg", regionInfo{"新加坡节点", "SG"}},
	{"-sg", regionInfo{"新加坡节点", "SG"}},
	{"sg-", regionInfo{"新加坡节点", "SG"}},
	{"unitedstates", regionInfo{"美国节点", "US"}},
	{"usa", regionInfo{"美国节点", "US"}},
	{".us", regionInfo{"美国节点", "US"}},
	{"-us", regionInfo{"美国节点", "US"}},
	{"us-", regionInfo{"美国节点", "US"}},
	{"netherlands", regionInfo{"荷兰节点", "NL"}},
	{"holland", regionInfo{"荷兰节点", "NL"}},
	{"amsterdam", regionInfo{"荷兰节点", "NL"}},
	{".nl", regionInfo{"荷兰节点", "NL"}},
	{"-nl", regionInfo{"荷兰节点", "NL"}},
	{"nl-", regionInfo{"荷兰节点", "NL"}},
}

var ipRegions = mustCIDRRegions(map[string]regionInfo{
	"1.32.0.0/12":     {"香港节点", "HK"},
	"14.0.128.0/17":   {"香港节点", "HK"},
	"14.136.0.0/14":   {"香港节点", "HK"},
	"27.96.0.0/12":    {"香港节点", "HK"},
	"42.200.0.0/13":   {"香港节点", "HK"},
	"45.64.0.0/11":    {"香港节点", "HK"},
	"58.64.128.0/17":  {"香港节点", "HK"},
	"103.0.0.0/8":     {"香港节点", "HK"},
	"118.140.0.0/14":  {"香港节点", "HK"},
	"123.136.0.0/13":  {"香港节点", "HK"},
	"203.80.0.0/13":   {"香港节点", "HK"},
	"210.0.128.0/17":  {"香港节点", "HK"},
	"218.188.0.0/14":  {"香港节点", "HK"},
	"1.160.0.0/12":    {"台湾节点", "TW"},
	"36.224.0.0/12":   {"台湾节点", "TW"},
	"59.112.0.0/12":   {"台湾节点", "TW"},
	"60.248.0.0/13":   {"台湾节点", "TW"},
	"61.216.0.0/13":   {"台湾节点", "TW"},
	"101.8.0.0/13":    {"台湾节点", "TW"},
	"111.240.0.0/12":  {"台湾节点", "TW"},
	"118.160.0.0/13":  {"台湾节点", "TW"},
	"140.112.0.0/12":  {"台湾节点", "TW"},
	"1.0.16.0/20":     {"日本节点", "JP"},
	"14.0.0.0/12":     {"日本节点", "JP"},
	"27.80.0.0/12":    {"日本节点", "JP"},
	"36.0.0.0/10":     {"日本节点", "JP"},
	"43.224.0.0/11":   {"日本节点", "JP"},
	"49.96.0.0/12":    {"日本节点", "JP"},
	"59.128.0.0/11":   {"日本节点", "JP"},
	"60.32.0.0/11":    {"日本节点", "JP"},
	"106.128.0.0/10":  {"日本节点", "JP"},
	"133.0.0.0/8":     {"日本节点", "JP"},
	"153.128.0.0/9":   {"日本节点", "JP"},
	"160.16.0.0/12":   {"日本节点", "JP"},
	"1.8.0.0/16":      {"新加坡节点", "SG"},
	"13.212.0.0/14":   {"新加坡节点", "SG"},
	"18.136.0.0/13":   {"新加坡节点", "SG"},
	"43.128.0.0/13":   {"新加坡节点", "SG"},
	"45.32.96.0/19":   {"新加坡节点", "SG"},
	"52.74.0.0/15":    {"新加坡节点", "SG"},
	"54.169.0.0/16":   {"新加坡节点", "SG"},
	"103.1.0.0/16":    {"新加坡节点", "SG"},
	"119.73.0.0/16":   {"新加坡节点", "SG"},
	"8.0.0.0/8":       {"美国节点", "US"},
	"13.0.0.0/8":      {"美国节点", "US"},
	"18.0.0.0/8":      {"美国节点", "US"},
	"20.0.0.0/8":      {"美国节点", "US"},
	"23.0.0.0/8":      {"美国节点", "US"},
	"34.0.0.0/8":      {"美国节点", "US"},
	"35.0.0.0/8":      {"美国节点", "US"},
	"44.0.0.0/8":      {"美国节点", "US"},
	"52.0.0.0/8":      {"美国节点", "US"},
	"54.0.0.0/8":      {"美国节点", "US"},
	"63.0.0.0/8":      {"美国节点", "US"},
	"64.0.0.0/8":      {"美国节点", "US"},
	"66.0.0.0/8":      {"美国节点", "US"},
	"67.0.0.0/8":      {"美国节点", "US"},
	"68.0.0.0/8":      {"美国节点", "US"},
	"69.0.0.0/8":      {"美国节点", "US"},
	"70.0.0.0/8":      {"美国节点", "US"},
	"72.0.0.0/8":      {"美国节点", "US"},
	"73.0.0.0/8":      {"美国节点", "US"},
	"74.0.0.0/8":      {"美国节点", "US"},
	"96.0.0.0/8":      {"美国节点", "US"},
	"104.0.0.0/8":     {"美国节点", "US"},
	"107.0.0.0/8":     {"美国节点", "US"},
	"108.0.0.0/8":     {"美国节点", "US"},
	"142.0.0.0/8":     {"美国节点", "US"},
	"172.0.0.0/8":     {"美国节点", "US"},
	"173.0.0.0/8":     {"美国节点", "US"},
	"184.0.0.0/8":     {"美国节点", "US"},
	"192.0.0.0/8":     {"美国节点", "US"},
	"198.0.0.0/8":     {"美国节点", "US"},
	"199.0.0.0/8":     {"美国节点", "US"},
	"204.0.0.0/8":     {"美国节点", "US"},
	"205.0.0.0/8":     {"美国节点", "US"},
	"206.0.0.0/8":     {"美国节点", "US"},
	"207.0.0.0/8":     {"美国节点", "US"},
	"208.0.0.0/8":     {"美国节点", "US"},
	"209.0.0.0/8":     {"美国节点", "US"},
	"65.49.192.0/18":  {"美国节点", "US"},
	"67.209.176.0/20": {"美国节点", "US"},
	"67.216.192.0/19": {"美国节点", "US"},
	"93.179.96.0/20":  {"美国节点", "US"},
	"104.245.96.0/20": {"荷兰节点", "NL"},
	"212.50.224.0/19": {"日本节点", "JP"},
})

var geoIPDB struct {
	once sync.Once
	db   *geoip2.Reader
}

func enrichNodeRegion(node *domain.Node) {
	info, resolvedIP := inferRegion(node.Name, node.Server)
	node.Region = info.Group
	node.RegionCode = info.Code
	node.ResolvedIP = resolvedIP
}

func inferRegion(name string, server string) (regionInfo, string) {
	if info := inferRegionFromText(name); info.Group != "" {
		return info, ""
	}
	if info := inferRegionFromText(server); info.Group != "" {
		return info, ""
	}
	if ip := net.ParseIP(strings.Trim(server, "[]")); ip != nil {
		if info := inferRegionFromGeoIP(ip); info.Group != "" {
			return info, ip.String()
		}
		if info := inferRegionFromIP(ip); info.Group != "" {
			return info, ip.String()
		}
		return regionInfo{"其他节点", "OTHER"}, ip.String()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, server)
	if err != nil {
		return regionInfo{"其他节点", "OTHER"}, ""
	}
	for _, addr := range addrs {
		if info := inferRegionFromGeoIP(addr.IP); info.Group != "" {
			return info, addr.IP.String()
		}
		if info := inferRegionFromIP(addr.IP); info.Group != "" {
			return info, addr.IP.String()
		}
	}
	if len(addrs) > 0 {
		return regionInfo{"其他节点", "OTHER"}, addrs[0].IP.String()
	}
	return regionInfo{"其他节点", "OTHER"}, ""
}

func inferRegionFromText(text string) regionInfo {
	lower := strings.ToLower(text)
	compact := strings.NewReplacer(" ", "", "_", "-", ".", "").Replace(lower)
	switch {
	case strings.Contains(text, "香港") || strings.Contains(lower, "hong kong") || strings.Contains(text, "🇭🇰"):
		return regionInfo{"香港节点", "HK"}
	case strings.Contains(text, "台湾") || strings.Contains(text, "台灣") || strings.Contains(lower, "taiwan") || strings.Contains(text, "🇹🇼"):
		return regionInfo{"台湾节点", "TW"}
	case strings.Contains(text, "日本") || strings.Contains(lower, "japan") || strings.Contains(lower, "tokyo") || strings.Contains(text, "🇯🇵"):
		return regionInfo{"日本节点", "JP"}
	case strings.Contains(text, "新加坡") || strings.Contains(lower, "singapore") || strings.Contains(text, "🇸🇬"):
		return regionInfo{"新加坡节点", "SG"}
	case strings.Contains(text, "美国") || strings.Contains(text, "美國") || strings.Contains(lower, "united states") || strings.Contains(lower, "usa") || strings.Contains(text, "🇺🇸"):
		return regionInfo{"美国节点", "US"}
	}
	for _, hint := range domainRegionHints {
		if strings.Contains(lower, hint.key) || strings.Contains(compact, strings.ReplaceAll(strings.ReplaceAll(hint.key, "-", ""), ".", "")) {
			return hint.info
		}
	}
	return regionInfo{}
}

func inferRegionFromIP(ip net.IP) regionInfo {
	for _, candidate := range ipRegions {
		if candidate.network.Contains(ip) {
			return candidate.info
		}
	}
	return regionInfo{}
}

func inferRegionFromGeoIP(ip net.IP) regionInfo {
	db := getGeoIPDB()
	if db == nil {
		return regionInfo{}
	}
	record, err := db.Country(ip)
	if err != nil {
		return regionInfo{}
	}
	return regionFromCountryCode(record.Country.IsoCode)
}

func getGeoIPDB() *geoip2.Reader {
	geoIPDB.once.Do(func() {
		for _, path := range []string{
			os.Getenv("SUBAGG_GEOIP_DB"),
			filepath.Join("data", "GeoLite2-Country.mmdb"),
		} {
			if strings.TrimSpace(path) == "" {
				continue
			}
			db, err := geoip2.Open(path)
			if err == nil {
				geoIPDB.db = db
				return
			}
		}
	})
	return geoIPDB.db
}

func regionFromCountryCode(code string) regionInfo {
	switch strings.ToUpper(code) {
	case "HK":
		return regionInfo{"香港节点", "HK"}
	case "TW":
		return regionInfo{"台湾节点", "TW"}
	case "JP":
		return regionInfo{"日本节点", "JP"}
	case "SG":
		return regionInfo{"新加坡节点", "SG"}
	case "US":
		return regionInfo{"美国节点", "US"}
	case "NL":
		return regionInfo{"荷兰节点", "NL"}
	default:
		return regionInfo{}
	}
}

func mustCIDRRegions(values map[string]regionInfo) []cidrRegion {
	regions := make([]cidrRegion, 0, len(values))
	for cidr, info := range values {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(err)
		}
		regions = append(regions, cidrRegion{network: network, info: info})
	}
	sort.Slice(regions, func(i, j int) bool {
		iOnes, _ := regions[i].network.Mask.Size()
		jOnes, _ := regions[j].network.Mask.Size()
		return iOnes > jOnes
	})
	return regions
}
