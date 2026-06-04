package aggregator

import (
	"testing"

	"sub-nest/internal/domain"
)

func TestInferRegionFromNameAndServer(t *testing.T) {
	tests := []struct {
		name   string
		server string
		want   string
	}{
		{name: "香港 IEPL 01", server: "example.com", want: "香港节点"},
		{name: "Example-Node@example.invalid:12103", server: "not-a-real-region-hint.invalid", want: "其他节点"},
		{name: "plain", server: "tokyo-01.example.net", want: "日本节点"},
		{name: "plain", server: "sg-node.example.net", want: "新加坡节点"},
		{name: "plain", server: "8.8.8.8", want: "美国节点"},
		{name: "plain", server: "203.0.113.10", want: "其他节点"},
	}
	for _, tt := range tests {
		info, _ := inferRegion(tt.name, tt.server)
		if info.Group != tt.want {
			t.Fatalf("inferRegion(%q, %q) = %q, want %q", tt.name, tt.server, info.Group, tt.want)
		}
	}
}

func TestGroupNodesUsesServerRegion(t *testing.T) {
	nodes := []domain.Node{
		{Name: "node-a", Server: "tokyo-01.example.net", Port: 443, Type: "trojan"},
		{Name: "node-b", Server: "us-node.example.net", Port: 443, Type: "trojan"},
	}
	_, counts := groupNodes(nodes)
	if counts["日本节点"] != 1 {
		t.Fatalf("expected one Japan node, got %d", counts["日本节点"])
	}
	if counts["美国节点"] != 1 {
		t.Fatalf("expected one US node, got %d", counts["美国节点"])
	}
}
