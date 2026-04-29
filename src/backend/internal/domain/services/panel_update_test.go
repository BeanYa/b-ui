package service

import (
	"net/http"
	"net/http/httptest"
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

func TestReconcilePanelUpdateStateMarksInactivePreflightTaskFailed(t *testing.T) {
	startedAt := time.Now().Add(-20 * time.Minute).Unix()
	state := &PanelUpdateState{
		Phase:         "preflight",
		TargetVersion: "v0.1.20",
		StartedAt:     startedAt,
		UpdatedAt:     startedAt,
	}

	reconciled, changed := reconcilePanelUpdateState(state, time.Now(), func() (bool, error) {
		return false, nil
	})

	if !changed {
		t.Fatal("expected stale preflight state to be changed")
	}
	if reconciled.Phase != "failed" {
		t.Fatalf("phase = %q, want failed", reconciled.Phase)
	}
	if reconciled.Message != "update_task_stopped" {
		t.Fatalf("message = %q, want update_task_stopped", reconciled.Message)
	}
}

func TestReconcilePanelUpdateStateClearsFailedStateCoveredByCurrentVersion(t *testing.T) {
	state := &PanelUpdateState{
		Phase:         "failed",
		TargetVersion: "v0.1.17",
		StartedAt:     time.Now().Add(-10 * time.Minute).Unix(),
		UpdatedAt:     time.Now().Add(-9 * time.Minute).Unix(),
		LogPath:       "/tmp/b-ui-panel-update.log",
		Message:       "update_task_stopped",
	}

	reconciled, changed := reconcilePanelUpdateStateWithCurrentVersion(state, "v0.1.18", time.Now())

	if !changed {
		t.Fatal("expected stale failed update state to be cleared")
	}
	if reconciled != nil {
		t.Fatalf("reconciled state = %#v, want nil", reconciled)
	}
}

func TestReconcilePanelUpdateStateCompletesRunningStateCoveredByCurrentVersion(t *testing.T) {
	now := time.Now()
	state := &PanelUpdateState{
		Phase:         "running",
		TargetVersion: "v0.1.18",
		StartedAt:     now.Add(-2 * time.Minute).Unix(),
		UpdatedAt:     now.Add(-2 * time.Minute).Unix(),
		LogPath:       "/tmp/b-ui-panel-update.log",
	}

	reconciled, changed := reconcilePanelUpdateStateWithCurrentVersion(state, "v0.1.18", now)

	if !changed {
		t.Fatal("expected running update state to be completed")
	}
	if reconciled == nil || reconciled.Phase != "completed" {
		t.Fatalf("phase = %#v, want completed state", reconciled)
	}
	if reconciled.Message != "current_version_reached" {
		t.Fatalf("message = %q, want current_version_reached", reconciled.Message)
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
		"--no-block",
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

func TestPanelUpdateAssetArch(t *testing.T) {
	tests := []struct {
		goarch string
		goarm  string
		want   string
		wantOK bool
	}{
		{goarch: "amd64", want: "amd64", wantOK: true},
		{goarch: "arm64", want: "arm64", wantOK: true},
		{goarch: "386", want: "386", wantOK: true},
		{goarch: "s390x", want: "s390x", wantOK: true},
		{goarch: "arm", goarm: "7", want: "armv7", wantOK: true},
		{goarch: "arm", goarm: "6", want: "armv6", wantOK: true},
		{goarch: "arm", goarm: "5", want: "armv5", wantOK: true},
		{goarch: "arm", goarm: "", want: "armv7", wantOK: true},
		{goarch: "mips", want: "", wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.goarch+"/"+tc.goarm, func(t *testing.T) {
			got, ok := panelUpdateAssetArch(tc.goarch, tc.goarm)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("arch = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReleaseAssetURL(t *testing.T) {
	url := releaseAssetURL("v0.1.20", "amd64")
	expected := "https://github.com/BeanYa/b-ui/releases/download/v0.1.20/b-ui-linux-amd64.tar.gz"
	if url != expected {
		t.Fatalf("url = %q, want %q", url, expected)
	}
}

func TestPreflightReleaseAssetReturnsImmediatelyOnSuccess(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	state := &PanelUpdateState{
		Phase:         "preflight",
		TargetVersion: "v0.1.20",
		StartedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}

	checker := newPreflightAssetChecker(server.Client(), state)
	checker.baseDownloadURL = server.URL + "/download"

	err := checker.check("v0.1.20", "amd64")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPreflightReleaseAssetFailsAfterMaxAttempts(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	state := &PanelUpdateState{
		Phase:         "preflight",
		TargetVersion: "v0.1.20",
		StartedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}

	checker := newPreflightAssetChecker(server.Client(), state)
	checker.baseDownloadURL = server.URL + "/download"
	checker.maxAttempts = 3
	checker.interval = 100 * time.Millisecond

	err := checker.check("v0.1.20", "amd64")
	if err == nil {
		t.Fatal("expected error after max attempts, got nil")
	}
	if !strings.Contains(err.Error(), "preflight") {
		t.Fatalf("error should mention preflight, got: %v", err)
	}
}

func TestPreflightReleaseAssetSucceedsOnRetry(t *testing.T) {
	attempt := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 3 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	state := &PanelUpdateState{
		Phase:         "preflight",
		TargetVersion: "v0.1.20",
		StartedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}

	checker := newPreflightAssetChecker(server.Client(), state)
	checker.baseDownloadURL = server.URL + "/download"
	checker.maxAttempts = 5
	checker.interval = 100 * time.Millisecond

	err := checker.check("v0.1.20", "amd64")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if attempt != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempt)
	}
}
