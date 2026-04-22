package api

import "testing"

func TestClusterMessagePathUsesBaseURLPrefix(t *testing.T) {
	if got := ClusterMessagePath("/panel/"); got != "/panel/cluster/message" {
		t.Fatalf("expected /panel/cluster/message, got %q", got)
	}
	if got := ClusterMessagePath("/"); got != "/cluster/message" {
		t.Fatalf("expected /cluster/message, got %q", got)
	}
}
