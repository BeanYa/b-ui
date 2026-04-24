package service

import (
	"os"
	"strings"
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

func TestBuildPanelUpdateCommandPassesEnvironmentToSystemdUnit(t *testing.T) {
	cmd := buildPanelUpdateCommand("v0.1.15", true, 1713950000, "/tmp/b-ui-panel-update.log")
	args := strings.Join(cmd.Args, "\n")

	expected := []string{
		"--setenv=INSTALL_SCRIPT_URL=https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh",
		"--setenv=INSTALL_MODE=--force-update",
		"--setenv=TARGET_VERSION=v0.1.15",
		"--setenv=UPDATE_FORCE_JSON=true",
		"--setenv=UPDATE_STARTED_AT=1713950000",
		"--setenv=UPDATE_LOG_FILE=/tmp/b-ui-panel-update.log",
	}
	for _, arg := range expected {
		if !strings.Contains(args, arg) {
			t.Fatalf("systemd-run args did not contain %q; args:\n%s", arg, args)
		}
	}
}

func TestHydratePanelUpdateStateReadsLogText(t *testing.T) {
	logFile, err := os.CreateTemp(t.TempDir(), "panel-update-*.log")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := logFile.WriteString("准备更新面板\n下载安装脚本\n执行安装脚本\n"); err != nil {
		t.Fatal(err)
	}
	if err := logFile.Close(); err != nil {
		t.Fatal(err)
	}

	state := &PanelUpdateState{
		Phase:         "running",
		TargetVersion: "v0.1.15",
		LogPath:       logFile.Name(),
	}

	hydratePanelUpdateStateLog(state)

	if !strings.Contains(state.LogText, "下载安装脚本") {
		t.Fatalf("LogText = %q, want hydrated log content", state.LogText)
	}
}
