package service

import (
	"testing"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

func TestExpandPeerRouteBroadcastSkipsSourceAndExcludedNodes(t *testing.T) {
	members := []model.ClusterMember{
		{NodeID: "node-a", BaseURL: "https://node-a.example.com"},
		{NodeID: "node-b", BaseURL: "https://node-b.example.com"},
		{NodeID: "node-c", BaseURL: "https://node-c.example.com"},
	}
	targets := ExpandClusterPeerRoute(RoutePlan{
		Mode:     RouteModeBroadcast,
		Selector: &TargetSelector{Exclude: []string{"node-c"}},
	}, members, "node-a")
	if len(targets) != 1 || targets[0].NodeID != "node-b" {
		t.Fatalf("expected only node-b, got %#v", targets)
	}
}

func TestExpandPeerRouteMulticastUsesFixedTargets(t *testing.T) {
	members := []model.ClusterMember{
		{NodeID: "node-a", BaseURL: "https://node-a.example.com"},
		{NodeID: "node-b", BaseURL: "https://node-b.example.com"},
		{NodeID: "node-c", BaseURL: "https://node-c.example.com"},
	}
	targets := ExpandClusterPeerRoute(RoutePlan{Mode: RouteModeMulticast, Targets: []string{"node-c", "node-b"}}, members, "node-a")
	if len(targets) != 2 || targets[0].NodeID != "node-c" || targets[1].NodeID != "node-b" {
		t.Fatalf("expected fixed multicast order c,b, got %#v", targets)
	}
}
