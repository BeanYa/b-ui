package service

import (
	"testing"
	"time"
)

func TestCompareReleaseTags(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		target   string
		expected string
	}{
		{name: "older current version", current: "v0.1.5", target: "v0.1.6", expected: "older"},
		{name: "same version", current: "v0.1.6", target: "v0.1.6", expected: "same"},
		{name: "newer current version", current: "v0.1.7", target: "v0.1.6", expected: "newer"},
		{name: "ignores tag prefix", current: "0.1.6", target: "v0.1.6", expected: "same"},
		{name: "compares missing patch segment", current: "v0.1", target: "v0.1.1", expected: "older"},
		{name: "prerelease stays below stable", current: "v0.1.6-beta.1", target: "v0.1.6", expected: "older"},
		{name: "invalid version returns unknown", current: "dev-build", target: "v0.1.6", expected: "unknown"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := compareReleaseTags(testCase.current, testCase.target); actual != testCase.expected {
				t.Fatalf("compareReleaseTags(%q, %q) = %q, want %q", testCase.current, testCase.target, actual, testCase.expected)
			}
		})
	}
}

func TestReconcilePanelUpdateStateMarksInactiveRunningTaskFailed(t *testing.T) {
	startedAt := time.Now().Add(-2 * time.Minute).Unix()
	state := &PanelUpdateState{
		Phase:         "running",
		TargetVersion: "v0.1.11",
		StartedAt:     startedAt,
		UpdatedAt:     startedAt,
		LogPath:       "/tmp/b-ui-panel-update.log",
	}

	reconciled, changed := reconcilePanelUpdateState(state, time.Now(), func() (bool, error) {
		return false, nil
	})

	if !changed {
		t.Fatal("expected stale running update state to be changed")
	}
	if reconciled.Phase != "failed" {
		t.Fatalf("phase = %q, want failed", reconciled.Phase)
	}
	if reconciled.Message != "update_task_stopped" {
		t.Fatalf("message = %q, want update_task_stopped", reconciled.Message)
	}
}

func TestResolvePanelUpdateLatestVersionUsesRunningTargetWithoutGithubFetch(t *testing.T) {
	state := &PanelUpdateState{
		Phase:         "running",
		TargetVersion: "v0.1.11",
	}

	latest, err := resolvePanelUpdateLatestVersion(state, func() (string, error) {
		t.Fatal("running update polling must not fetch GitHub release metadata")
		return "", nil
	})

	if err != nil {
		t.Fatalf("resolvePanelUpdateLatestVersion returned error: %v", err)
	}
	if latest != "v0.1.11" {
		t.Fatalf("latest = %q, want v0.1.11", latest)
	}
}

func TestResolvePanelUpdateLatestVersionIgnoresFailedTarget(t *testing.T) {
	state := &PanelUpdateState{
		Phase:         "failed",
		TargetVersion: "v0.1.12",
	}

	latest, err := resolvePanelUpdateLatestVersion(state, func() (string, error) {
		return "v0.1.14", nil
	})

	if err != nil {
		t.Fatalf("resolvePanelUpdateLatestVersion returned error: %v", err)
	}
	if latest != "v0.1.14" {
		t.Fatalf("latest = %q, want v0.1.14", latest)
	}
}
