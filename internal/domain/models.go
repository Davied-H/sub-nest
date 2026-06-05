package domain

import "time"

type Source struct {
	ID              string             `json:"id"`
	OwnerUserID     string             `json:"ownerUserId,omitempty"`
	Name            string             `json:"name"`
	URL             string             `json:"url"`
	SourceType      string             `json:"sourceType,omitempty"`
	FileName        string             `json:"fileName,omitempty"`
	FileContent     string             `json:"fileContent,omitempty"`
	TrafficQuery    TrafficQueryConfig `json:"trafficQuery"`
	TrafficInfo     TrafficInfo        `json:"trafficInfo"`
	Enabled         bool               `json:"enabled"`
	Remark          string             `json:"remark"`
	Tags            []string           `json:"tags"`
	LastStatus      string             `json:"lastStatus"`
	LastFormat      string             `json:"lastFormat"`
	LastNodeCount   int                `json:"lastNodeCount"`
	LastError       string             `json:"lastError"`
	RefreshProgress string             `json:"refreshProgress"`
	RefreshPercent  int                `json:"refreshPercent"`
	LastRefreshedAt *time.Time         `json:"lastRefreshedAt,omitempty"`
	LastSuccessAt   *time.Time         `json:"lastSuccessAt,omitempty"`
	CachedNodes     []Node             `json:"cachedNodes,omitempty"`
}

type Output struct {
	ID                string            `json:"id"`
	OwnerUserID       string            `json:"ownerUserId,omitempty"`
	Slug              string            `json:"slug"`
	Name              string            `json:"name"`
	Enabled           bool              `json:"enabled"`
	Format            string            `json:"format"`
	SourceIDs         []string          `json:"sourceIds"`
	Filter            FilterRules       `json:"filter"`
	RenameRules       []RenameRule      `json:"renameRules"`
	NodeNameOverrides map[string]string `json:"nodeNameOverrides,omitempty"`
	GroupMode         string            `json:"groupMode"`
	LastGeneratedAt   *time.Time        `json:"lastGeneratedAt,omitempty"`
	LastNodeCount     int               `json:"lastNodeCount"`
	LastDroppedCount  int               `json:"lastDroppedCount"`
}

type FilterRules struct {
	IncludeKeywords []string `json:"includeKeywords"`
	ExcludeKeywords []string `json:"excludeKeywords"`
	Regex           string   `json:"regex"`
}

type RenameRule struct {
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
}

type TrafficQueryConfig struct {
	Mode    string            `json:"mode"`
	URL     string            `json:"url,omitempty"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Parser  TrafficParser     `json:"parser,omitempty"`
}

type TrafficParser struct {
	Type      string `json:"type,omitempty"`
	Upload    string `json:"upload,omitempty"`
	Download  string `json:"download,omitempty"`
	Total     string `json:"total,omitempty"`
	Remaining string `json:"remaining,omitempty"`
	Expire    string `json:"expire,omitempty"`
}

type TrafficInfo struct {
	UploadBytes    int64      `json:"uploadBytes"`
	DownloadBytes  int64      `json:"downloadBytes"`
	TotalBytes     int64      `json:"totalBytes"`
	RemainingBytes int64      `json:"remainingBytes"`
	ExpireAt       *time.Time `json:"expireAt,omitempty"`
	LastCheckedAt  *time.Time `json:"lastCheckedAt,omitempty"`
	LastStatus     string     `json:"lastStatus"`
	LastError      string     `json:"lastError"`
}

type Node struct {
	Name           string                 `json:"name" yaml:"name"`
	Type           string                 `json:"type" yaml:"type"`
	Server         string                 `json:"server" yaml:"server"`
	Port           int                    `json:"port" yaml:"port"`
	Region         string                 `json:"region,omitempty" yaml:"-"`
	RegionCode     string                 `json:"regionCode,omitempty" yaml:"-"`
	ResolvedIP     string                 `json:"resolvedIp,omitempty" yaml:"-"`
	ExitIP         string                 `json:"exitIp,omitempty" yaml:"-"`
	Alive          *bool                  `json:"alive,omitempty" yaml:"-"`
	ExcludedReason string                 `json:"excludedReason,omitempty" yaml:"-"`
	RegionSource   string                 `json:"regionSource,omitempty" yaml:"-"`
	ProbeStatus    string                 `json:"probeStatus,omitempty" yaml:"-"`
	ProbeError     string                 `json:"probeError,omitempty" yaml:"-"`
	ProbeChecked   *time.Time             `json:"probeChecked,omitempty" yaml:"-"`
	Raw            string                 `json:"raw,omitempty" yaml:"-"`
	SourceID       string                 `json:"sourceId,omitempty" yaml:"-"`
	Source         string                 `json:"source,omitempty" yaml:"-"`
	Extra          map[string]interface{} `json:"extra,omitempty" yaml:",inline"`
}

type Settings struct {
	AdminTokenHash string `json:"adminTokenHash"`
	UserTokenHash  string `json:"userTokenHash"`
	PublicBaseURL  string `json:"publicBaseUrl"`
	RefreshMinutes int    `json:"refreshMinutes"`
}

type SettingsView struct {
	PublicBaseURL string `json:"publicBaseUrl"`
	HasUserToken  bool   `json:"hasUserToken"`
}

type User struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	TokenHash   string     `json:"tokenHash,omitempty"`
	Role        string     `json:"role"`
	Enabled     bool       `json:"enabled"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
}

type InviteCode struct {
	ID               string     `json:"id"`
	CodeHash         string     `json:"codeHash,omitempty"`
	Label            string     `json:"label"`
	CreatedAt        time.Time  `json:"createdAt"`
	UsedAt           *time.Time `json:"usedAt,omitempty"`
	UsedByUserID     string     `json:"usedByUserId,omitempty"`
	CreatedByAdminID string     `json:"createdByAdminId"`
}

type Config struct {
	Version     int          `json:"version"`
	Settings    Settings     `json:"settings"`
	Users       []User       `json:"users"`
	InviteCodes []InviteCode `json:"inviteCodes"`
	Sources     []Source     `json:"sources"`
	Outputs     []Output     `json:"outputs"`
	Updated     time.Time    `json:"updated"`
}

type SourceView struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	URLMasked       string             `json:"urlMasked"`
	URL             string             `json:"url,omitempty"`
	FileContent     string             `json:"fileContent,omitempty"`
	SourceType      string             `json:"sourceType,omitempty"`
	FileName        string             `json:"fileName,omitempty"`
	TrafficQuery    TrafficQueryConfig `json:"trafficQuery"`
	TrafficInfo     TrafficInfo        `json:"trafficInfo"`
	Enabled         bool               `json:"enabled"`
	Remark          string             `json:"remark"`
	Tags            []string           `json:"tags"`
	LastStatus      string             `json:"lastStatus"`
	LastFormat      string             `json:"lastFormat"`
	LastNodeCount   int                `json:"lastNodeCount"`
	LastError       string             `json:"lastError"`
	RefreshProgress string             `json:"refreshProgress"`
	RefreshPercent  int                `json:"refreshPercent"`
	LastRefreshedAt *time.Time         `json:"lastRefreshedAt,omitempty"`
	LastSuccessAt   *time.Time         `json:"lastSuccessAt,omitempty"`
	NodeStats       NodeStats          `json:"nodeStats"`
	Nodes           []NodePreview      `json:"nodes,omitempty"`
}

type NodeStats struct {
	Total     int `json:"total"`
	Alive     int `json:"alive"`
	Dead      int `json:"dead"`
	Unchecked int `json:"unchecked"`
}

type Dashboard struct {
	SourceCount      int        `json:"sourceCount"`
	EnabledSources   int        `json:"enabledSources"`
	HealthySources   int        `json:"healthySources"`
	UnhealthySources int        `json:"unhealthySources"`
	OutputCount      int        `json:"outputCount"`
	EnabledOutputs   int        `json:"enabledOutputs"`
	TotalCachedNodes int        `json:"totalCachedNodes"`
	LastRefreshAt    *time.Time `json:"lastRefreshAt,omitempty"`
	PublicExampleURL string     `json:"publicExampleUrl"`
	NeedsAdminSetup  bool       `json:"needsAdminSetup"`
}

type Preview struct {
	OutputID          string         `json:"outputId"`
	Slug              string         `json:"slug"`
	NodeCount         int            `json:"nodeCount"`
	DuplicateCount    int            `json:"duplicateCount"`
	FilteredCount     int            `json:"filteredCount"`
	UnavailableCount  int            `json:"unavailableCount"`
	FailedSources     []SourceView   `json:"failedSources"`
	RegionCounts      map[string]int `json:"regionCounts"`
	Groups            []GroupPreview `json:"groups"`
	Nodes             []NodePreview  `json:"nodes"`
	ExcludedNodes     []NodePreview  `json:"excludedNodes"`
	GeneratedAt       time.Time      `json:"generatedAt"`
	UsedCachedSources int            `json:"usedCachedSources"`
}

type GroupPreview struct {
	Name  string   `json:"name"`
	Nodes []string `json:"nodes"`
}

type NodePreview struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	OriginalName   string `json:"originalName"`
	Server         string `json:"server"`
	Port           int    `json:"port"`
	Region         string `json:"region"`
	RegionCode     string `json:"regionCode"`
	ResolvedIP     string `json:"resolvedIp"`
	ExitIP         string `json:"exitIp"`
	Alive          *bool  `json:"alive"`
	ExcludedReason string `json:"excludedReason"`
	RegionSource   string `json:"regionSource"`
	ProbeStatus    string `json:"probeStatus"`
	ProbeError     string `json:"probeError"`
}
