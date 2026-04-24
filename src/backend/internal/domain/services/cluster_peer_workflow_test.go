package service

import "testing"

func TestNextChainStepStopsOnFailureByDefault(t *testing.T) {
	route := RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
		{StepID: "step-a", NodeID: "node-a"},
		{StepID: "step-b", NodeID: "node-b"},
	}}
	next, ok := NextClusterPeerChainStep(route, "step-a", false)
	if ok || next.StepID != "" {
		t.Fatalf("expected chain to stop on failure, got %#v %v", next, ok)
	}
}

func TestNextChainStepContinuesOnSuccess(t *testing.T) {
	route := RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
		{StepID: "step-a", NodeID: "node-a"},
		{StepID: "step-b", NodeID: "node-b"},
	}}
	next, ok := NextClusterPeerChainStep(route, "step-a", true)
	if !ok || next.StepID != "step-b" {
		t.Fatalf("expected step-b, got %#v %v", next, ok)
	}
}

func TestNextChainStepContinuesOnFailureWhenAllowed(t *testing.T) {
	route := RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
		{StepID: "step-a", NodeID: "node-a", ContinueOnFailure: true},
		{StepID: "step-b", NodeID: "node-b"},
	}}
	next, ok := NextClusterPeerChainStep(route, "step-a", false)
	if !ok || next.StepID != "step-b" {
		t.Fatalf("expected step-b after allowed failure, got %#v %v", next, ok)
	}
}

func TestNextChainStepReturnsFalseForNonChainRoute(t *testing.T) {
	route := RoutePlan{Mode: RouteModeDirect, Chain: []RouteStep{
		{StepID: "step-a", NodeID: "node-a"},
		{StepID: "step-b", NodeID: "node-b"},
	}}
	next, ok := NextClusterPeerChainStep(route, "step-a", true)
	if ok || next.StepID != "" {
		t.Fatalf("expected no next step for non-chain route, got %#v %v", next, ok)
	}
}

func TestNextChainStepReturnsFalseForLastOrMissingStep(t *testing.T) {
	route := RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
		{StepID: "step-a", NodeID: "node-a"},
		{StepID: "step-b", NodeID: "node-b"},
	}}

	next, ok := NextClusterPeerChainStep(route, "step-b", true)
	if ok || next.StepID != "" {
		t.Fatalf("expected no next step after last step, got %#v %v", next, ok)
	}

	next, ok = NextClusterPeerChainStep(route, "step-missing", true)
	if ok || next.StepID != "" {
		t.Fatalf("expected no next step for missing step, got %#v %v", next, ok)
	}
}
