package api

import "testing"

func TestClusterMessagePathUsesBaseURLPrefix(t *testing.T) {
	if got := ClusterMessagePath("/panel/"); got != "/panel/_cluster/v1/events" {
		t.Fatalf("expected /panel/_cluster/v1/events, got %q", got)
	}
	if got := ClusterMessagePath("/"); got != "/_cluster/v1/events" {
		t.Fatalf("expected /_cluster/v1/events, got %q", got)
	}
}
