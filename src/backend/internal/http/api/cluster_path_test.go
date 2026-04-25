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

func TestClusterInfoPath(t *testing.T) {
	if got := ClusterInfoPath("/panel/"); got != "/panel/_cluster/v1/info" {
		t.Fatalf("expected /panel/_cluster/v1/info, got %q", got)
	}
	if got := ClusterInfoPath("/"); got != "/_cluster/v1/info" {
		t.Fatalf("expected /_cluster/v1/info, got %q", got)
	}
}

func TestClusterActionPath(t *testing.T) {
	if got := ClusterActionPath("/panel/"); got != "/panel/_cluster/v1/action" {
		t.Fatalf("expected /panel/_cluster/v1/action, got %q", got)
	}
	if got := ClusterActionPath("/"); got != "/_cluster/v1/action" {
		t.Fatalf("expected /_cluster/v1/action, got %q", got)
	}
}
