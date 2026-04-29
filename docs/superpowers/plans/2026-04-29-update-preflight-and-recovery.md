# Update Preflight & Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a preflight phase to b-ui's auto-update that verifies the GitHub release asset is downloadable before starting the install, and recover online cluster status after manual updates complete previously-failed auto-updates.

**Architecture:** Extend the panel update state machine with a `preflight` phase that polls the release tarball via HTTP HEAD before launching the systemd install unit. Add a `recovered` signal to the version reconciliation flow so the cluster sync layer can re-report online status when a manual update resolves a previous failure.

**Tech Stack:** Go 1.x, net/http, runtime package for arch detection

---

### Task 1: Add release asset URL builder and arch mapping

**Files:**
- Modify: `b-ui/src/backend/internal/domain/services/panel_update.go` (after line 520)
- Test: `b-ui/src/backend/internal/domain/services/panel_update_test.go`

- [ ] **Step 1: Write the failing tests for `releaseAssetURL` and `panelUpdateAssetArch`**

```go
func TestPanelUpdateAssetArch(t *testing.T) {
	tests := []struct {
		goarch  string
		goarm   string
		want    string
		wantOK  bool
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd b-ui/src/backend && go test ./internal/domain/services/ -run "TestPanelUpdateAssetArch|TestReleaseAssetURL" -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement `panelUpdateAssetArch` and `releaseAssetURL`**

Add after `panelInstallScriptURL()` (after line 520 in panel_update.go):

```go
func panelUpdateAssetArch(goarch, goarm string) (string, bool) {
	switch goarch {
	case "amd64":
		return "amd64", true
	case "arm64":
		return "arm64", true
	case "386":
		return "386", true
	case "s390x":
		return "s390x", true
	case "arm":
		switch goarm {
		case "5":
			return "armv5", true
		case "6":
			return "armv6", true
		default:
			return "armv7", true
		}
	default:
		return "", false
	}
}

func releaseAssetURL(targetVersion, arch string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/b-ui-linux-%s.tar.gz",
		panelUpdateRepoOwner, panelUpdateRepoName, targetVersion, arch)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd b-ui/src/backend && go test ./internal/domain/services/ -run "TestPanelUpdateAssetArch|TestReleaseAssetURL" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd b-ui
git add src/backend/internal/domain/services/panel_update.go src/backend/internal/domain/services/panel_update_test.go
git commit -m "feat(panel-update): add release asset arch mapping and URL builder"
```

---

### Task 2: Add preflight asset check logic

**Files:**
- Modify: `b-ui/src/backend/internal/domain/services/panel_update.go`
- Test: `b-ui/src/backend/internal/domain/services/panel_update_test.go`

- [ ] **Step 1: Write the failing test for `preflightReleaseAsset`**

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd b-ui/src/backend && go test ./internal/domain/services/ -run "TestPreflightReleaseAsset" -v`
Expected: FAIL — types/functions not defined

- [ ] **Step 3: Implement `preflightAssetChecker`**

Add to `panel_update.go`, after the `releaseAssetURL` function. Also add `"net/http/httptest"` to test imports only.

Constants at top of file (add after existing constants around line 27):

```go
const (
	panelUpdatePreflightMaxAttempts = 30
	panelUpdatePreflightInterval    = 30 * time.Second
)
```

Implementation:

```go
type preflightAssetChecker struct {
	client         *http.Client
	state          *PanelUpdateState
	baseDownloadURL string
	maxAttempts    int
	interval       time.Duration
}

func newPreflightAssetChecker(client *http.Client, state *PanelUpdateState) *preflightAssetChecker {
	return &preflightAssetChecker{
		client:         client,
		state:          state,
		baseDownloadURL: fmt.Sprintf("https://github.com/%s/%s/releases/download", panelUpdateRepoOwner, panelUpdateRepoName),
		maxAttempts:    panelUpdatePreflightMaxAttempts,
		interval:       panelUpdatePreflightInterval,
	}
}

func (c *preflightAssetChecker) check(targetVersion, arch string) error {
	assetURL := c.baseDownloadURL + "/" + targetVersion + "/b-ui-linux-" + arch + ".tar.gz"
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		resp, err := c.client.Head(assetURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		c.state.UpdatedAt = time.Now().Unix()
		_ = savePanelUpdateState(c.state)
		if attempt < c.maxAttempts {
			time.Sleep(c.interval)
		}
	}
	return fmt.Errorf("preflight: release asset not available after %d attempts", c.maxAttempts)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd b-ui/src/backend && go test ./internal/domain/services/ -run "TestPreflightReleaseAsset" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd b-ui
git add src/backend/internal/domain/services/panel_update.go src/backend/internal/domain/services/panel_update_test.go
git commit -m "feat(panel-update): add preflight asset checker with HEAD polling"
```

---

### Task 3: Wire preflight into `StartUpdate` flow

**Files:**
- Modify: `b-ui/src/backend/internal/domain/services/panel_update.go` (lines 106-163, the `StartUpdate` function)

- [ ] **Step 1: Write the failing test for StartUpdate with preflight**

```go
func TestStartUpdateEntersPreflightThenRunning(t *testing.T) {
	preflightComplete := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		close(preflightComplete)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	stateDir := t.TempDir()
	origStateFile := panelUpdateStateFilePath
	panelUpdateStateFilePath = func() string { return filepath.Join(stateDir, "state.json") }
	defer func() { panelUpdateStateFilePath = origStateFile }()

	logDir := t.TempDir()
	origLogFile := panelUpdateLogFilePath
	panelUpdateLogFilePath = func() string { return filepath.Join(logDir, "update.log") }
	defer func() { panelUpdateLogFilePath = origLogFile }()

	origLaunch := launchDetachedPanelUpdate
	launchDetachedPanelUpdate = func(targetVersion string, force bool, startedAt int64, logPath string) error {
		return nil
	}
	defer func() { launchDetachedPanelUpdate = origLaunch }()

	s := &PanelService{}
	result, err := s.StartUpdate("v99.0.0", true)
	if err != nil {
		t.Fatalf("StartUpdate returned error: %v", err)
	}
	if result.TargetVersion != "v99.0.0" {
		t.Fatalf("target = %q, want v99.0.0", result.TargetVersion)
	}

	state, _ := loadPanelUpdateState()
	if state == nil || state.Phase != "completed" {
		t.Fatalf("state = %#v, want completed", state)
	}
}
```

Note: This test requires making `panelUpdateStateFilePath`, `panelUpdateLogFilePath`, and `launchDetachedPanelUpdate` overridable. If they are not currently variable, they need to be converted to function variables.

- [ ] **Step 2: Make state/log file paths and launch function overridable for testing**

Convert the following to package-level function variables in `panel_update.go`:

```go
var panelUpdateStateFilePath = func() string {
	return filepath.Join(os.TempDir(), "b-ui-panel-update-state.json")
}

var panelUpdateLogFilePath = func() string {
	return filepath.Join(os.TempDir(), "b-ui-panel-update.log")
}

var launchDetachedPanelUpdate = func(targetVersion string, force bool, startedAt int64, logPath string) error {
	cmd := buildPanelUpdateCommand(targetVersion, force, startedAt, logPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("start panel update: %w: %s", err, message)
		}
		return fmt.Errorf("start panel update: %w", err)
	}
	return nil
}
```

Remove the old `panelUpdateStateFilePath`, `panelUpdateLogFilePath`, and `launchDetachedPanelUpdate` function definitions. Update all call sites — they already call these as functions so the call syntax doesn't change.

- [ ] **Step 3: Modify `StartUpdate` to include preflight phase**

Replace `StartUpdate` (lines 106-163) with:

```go
func (s *PanelService) StartUpdate(targetVersion string, force bool) (*PanelUpdateStartResult, error) {
	supported, reason := panelUpdateCapability()
	if !supported {
		return nil, errors.New(reason)
	}

	resolvedVersion := canonicalizeReleaseTag(targetVersion)
	if resolvedVersion == "" {
		latestVersion, err := fetchLatestReleaseTag()
		if err != nil {
			return nil, err
		}
		resolvedVersion = latestVersion
	}

	if state, err := loadPanelUpdateState(); err == nil && state != nil && state.Phase == "running" {
		return nil, errors.New("panel update already in progress")
	}

	currentVersion := canonicalizeReleaseTag(config.GetVersion())
	comparison := compareReleaseTags(currentVersion, resolvedVersion)
	if !force && comparison != "older" {
		return nil, fmt.Errorf("current version %s requires a force update for target %s", fallbackVersion(currentVersion), resolvedVersion)
	}

	logPath := panelUpdateLogFilePath()
	startedAt := time.Now().Unix()
	if err := initializePanelUpdateLog(logPath, resolvedVersion, force, startedAt); err != nil {
		return nil, err
	}

	state := &PanelUpdateState{
		Phase:         "preflight",
		TargetVersion: resolvedVersion,
		Force:         force,
		StartedAt:     startedAt,
		UpdatedAt:     startedAt,
		LogPath:       logPath,
	}
	if err := savePanelUpdateState(state); err != nil {
		return nil, err
	}

	if err := runPreflightCheck(state, resolvedVersion); err != nil {
		state.Phase = "failed"
		state.Message = "preflight_failed"
		state.UpdatedAt = time.Now().Unix()
		_ = savePanelUpdateState(state)
		return nil, err
	}

	state.Phase = "running"
	state.UpdatedAt = time.Now().Unix()
	if err := savePanelUpdateState(state); err != nil {
		return nil, err
	}

	if err := launchDetachedPanelUpdate(resolvedVersion, force, startedAt, logPath); err != nil {
		state.Phase = "failed"
		state.Message = "launch_failed"
		state.UpdatedAt = time.Now().Unix()
		_ = savePanelUpdateState(state)
		return nil, err
	}

	return &PanelUpdateStartResult{
		TargetVersion: resolvedVersion,
		Force:         force,
		StartedAt:     startedAt,
		LogPath:       logPath,
	}, nil
}

func runPreflightCheck(state *PanelUpdateState, targetVersion string) error {
	arch, ok := panelUpdateAssetArch(runtime.GOARCH, os.Getenv("GOARM"))
	if !ok {
		return fmt.Errorf("preflight: unsupported architecture %s", runtime.GOARCH)
	}
	checker := newPreflightAssetChecker(&http.Client{Timeout: 15 * time.Second}, state)
	return checker.check(targetVersion, arch)
}
```

- [ ] **Step 4: Run all panel_update tests**

Run: `cd b-ui/src/backend && go test ./internal/domain/services/ -run "TestPanelUpdate|TestStartUpdate" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd b-ui
git add src/backend/internal/domain/services/panel_update.go src/backend/internal/domain/services/panel_update_test.go
git commit -m "feat(panel-update): wire preflight check into StartUpdate flow"
```

---

### Task 4: Handle preflight phase in state reconciliation

**Files:**
- Modify: `b-ui/src/backend/internal/domain/services/panel_update.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd b-ui/src/backend && go test ./internal/domain/services/ -run "TestReconcilePanelUpdateStateMarksInactivePreflightTaskFailed" -v`
Expected: FAIL — preflight phase not handled in reconcile

- [ ] **Step 3: Update `reconcilePanelUpdateState` to handle preflight phase**

Change the guard condition in `reconcilePanelUpdateState` (line 336) from `state.Phase != "running"` to also include `"preflight"`:

```go
func reconcilePanelUpdateState(state *PanelUpdateState, now time.Time, isUnitActive func() (bool, error)) (*PanelUpdateState, bool) {
	if state == nil || (state.Phase != "running" && state.Phase != "preflight") {
		return state, false
	}
	// ... rest unchanged
}
```

- [ ] **Step 4: Run tests to verify**

Run: `cd b-ui/src/backend && go test ./internal/domain/services/ -run "TestReconcile" -v`
Expected: PASS (all reconcile tests)

- [ ] **Step 5: Commit**

```bash
cd b-ui
git add src/backend/internal/domain/services/panel_update.go src/backend/internal/domain/services/panel_update_test.go
git commit -m "fix(panel-update): handle preflight phase in state reconciliation"
```

---

### Task 5: Add `recovered` signal to version reconciliation

**Files:**
- Modify: `b-ui/src/backend/internal/domain/services/panel_update.go`

- [ ] **Step 1: Write the failing test for the recovered return value**

```go
func TestReconcilePanelUpdateStateWithCurrentVersionReturnsRecovered(t *testing.T) {
	now := time.Now()
	state := &PanelUpdateState{
		Phase:         "failed",
		TargetVersion: "v0.1.17",
		StartedAt:     now.Add(-10 * time.Minute).Unix(),
		UpdatedAt:     now.Add(-9 * time.Minute).Unix(),
		Message:       "install_failed",
	}

	reconciled, changed, recovered := reconcilePanelUpdateStateWithCurrentVersion(state, "v0.1.18", now)

	if !changed {
		t.Fatal("expected failed state to be cleared")
	}
	if reconciled != nil {
		t.Fatalf("reconciled state = %#v, want nil", reconciled)
	}
	if !recovered {
		t.Fatal("expected recovered = true when failed state is cleared by current version")
	}
}

func TestReconcilePanelUpdateStateWithCurrentVersionNoRecoveryForRunning(t *testing.T) {
	now := time.Now()
	state := &PanelUpdateState{
		Phase:         "running",
		TargetVersion: "v0.1.18",
		StartedAt:     now.Add(-2 * time.Minute).Unix(),
		UpdatedAt:     now.Add(-2 * time.Minute).Unix(),
	}

	reconciled, changed, recovered := reconcilePanelUpdateStateWithCurrentVersion(state, "v0.1.18", now)

	if !changed {
		t.Fatal("expected running state to be completed")
	}
	if recovered {
		t.Fatal("expected recovered = false for running state completion")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd b-ui/src/backend && go test ./internal/domain/services/ -run "TestReconcilePanelUpdateStateWithCurrentVersion" -v`
Expected: FAIL — function returns 2 values, tests expect 3

- [ ] **Step 3: Update `reconcilePanelUpdateStateWithCurrentVersion` to return `recovered`**

Change signature from returning `(*PanelUpdateState, bool)` to `(*PanelUpdateState, bool, bool)`:

```go
func reconcilePanelUpdateStateWithCurrentVersion(state *PanelUpdateState, currentVersion string, now time.Time) (*PanelUpdateState, bool, bool) {
	if state == nil || state.TargetVersion == "" || currentVersion == "" {
		return state, false, false
	}

	if compareReleaseTags(currentVersion, state.TargetVersion) == "older" {
		return state, false, false
	}

	switch state.Phase {
	case "failed":
		return nil, true, true
	case "running", "preflight":
		state.Phase = "completed"
		state.Message = "current_version_reached"
		state.UpdatedAt = now.Unix()
		return state, true, false
	default:
		return state, false, false
	}
}
```

Update all call sites to accept the third return value:

In `GetUpdateInfo()` (line 69):
```go
if reconciledState, changed, _ := reconcilePanelUpdateStateWithCurrentVersion(state, currentVersion, time.Now()); changed {
```

In test `TestReconcilePanelUpdateStateClearsFailedStateCoveredByCurrentVersion`:
```go
reconciled, changed, _ := reconcilePanelUpdateStateWithCurrentVersion(state, "v0.1.18", time.Now())
```

In test `TestReconcilePanelUpdateStateCompletesRunningStateCoveredByCurrentVersion`:
```go
reconciled, changed, _ := reconcilePanelUpdateStateWithCurrentVersion(state, "v0.1.18", now)
```

- [ ] **Step 4: Run all tests**

Run: `cd b-ui/src/backend && go test ./internal/domain/services/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd b-ui
git add src/backend/internal/domain/services/panel_update.go src/backend/internal/domain/services/panel_update_test.go
git commit -m "feat(panel-update): add recovered signal to version reconciliation"
```

---

### Task 6: Propagate recovery signal through `PanelUpdateInfo`

**Files:**
- Modify: `b-ui/src/backend/internal/domain/services/panel_update.go`

- [ ] **Step 1: Add `Recovered` field to `PanelUpdateInfo`**

Add to `PanelUpdateInfo` struct (line 29):

```go
type PanelUpdateInfo struct {
	Supported         bool              `json:"supported"`
	UnsupportedReason string            `json:"unsupportedReason,omitempty"`
	CurrentVersion    string            `json:"currentVersion"`
	LatestVersion     string            `json:"latestVersion,omitempty"`
	Comparison        string            `json:"comparison"`
	UpdateAvailable   bool              `json:"updateAvailable"`
	ForceRequired     bool              `json:"forceRequired"`
	UpdateState       *PanelUpdateState `json:"updateState,omitempty"`
	Recovered         bool              `json:"recovered,omitempty"`
}
```

- [ ] **Step 2: Set `Recovered` in `GetUpdateInfo`**

Update the reconcile call in `GetUpdateInfo()` to capture and set `Recovered`:

```go
func (s *PanelService) GetUpdateInfo() (*PanelUpdateInfo, error) {
	state, err := loadPanelUpdateState()
	if err != nil {
		return nil, err
	}

	currentVersion := canonicalizeReleaseTag(config.GetVersion())
	var recovered bool
	if reconciledState, changed, rec := reconcilePanelUpdateStateWithCurrentVersion(state, currentVersion, time.Now()); changed {
		state = reconciledState
		recovered = rec
		_ = saveOrClearPanelUpdateState(state)
	}

	supported, reason := panelUpdateCapability()
	info := &PanelUpdateInfo{
		Supported:         supported,
		UnsupportedReason: reason,
		CurrentVersion:    currentVersion,
		Comparison:        "unknown",
		ForceRequired:     true,
		UpdateState:       state,
		Recovered:         recovered,
	}
	if !supported {
		return info, nil
	}

	latestVersion, err := resolvePanelUpdateLatestVersion(state, fetchLatestReleaseTag)
	if err != nil {
		return nil, err
	}
	info.LatestVersion = latestVersion
	info.Comparison = compareReleaseTags(currentVersion, latestVersion)

	switch info.Comparison {
	case "older":
		info.UpdateAvailable = true
		info.ForceRequired = false
	case "same", "newer", "unknown":
		info.UpdateAvailable = false
		info.ForceRequired = true
	}

	return info, nil
}
```

- [ ] **Step 3: Run tests**

Run: `cd b-ui/src/backend && go test ./internal/domain/services/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
cd b-ui
git add src/backend/internal/domain/services/panel_update.go
git commit -m "feat(panel-update): propagate recovered signal through PanelUpdateInfo"
```

---

### Task 7: Wire recovery signal into cluster sync

**Files:**
- Modify: `b-ui/src/backend/internal/domain/services/cluster_sync.go`

- [ ] **Step 1: Add recovery handling in `CheckAndBroadcastUpdate`**

In `CheckAndBroadcastUpdate()` (around line 262), after `GetUpdateInfo()` returns, check for recovery:

```go
func (s *ClusterSyncService) CheckAndBroadcastUpdate(ctx context.Context, domain *model.ClusterDomain) (*ClusterPanelUpdateCheckResult, error) {
	if domain == nil {
		return nil, errClusterDomainNotFound
	}
	info, err := s.getPanelUpdater().GetUpdateInfo()
	if err != nil {
		return nil, err
	}
	currentVersion := canonicalizeReleaseTag(info.CurrentVersion)
	if currentVersion == "" {
		currentVersion = canonicalizeReleaseTag(config.GetVersion())
	}

	if info.Recovered {
		local, localErr := s.getLocalIdentity().GetOrCreate()
		if localErr == nil {
			_ = s.markLocalMemberOnline(ctx, domain, local.NodeID, currentVersion)
			_ = s.publishPanelUpdateStatus(ctx, domain, ClusterPanelUpdateStatusOnline, "", currentVersion, local.NodeID)
		}
	}

	latestVersion := canonicalizeReleaseTag(info.LatestVersion)
	// ... rest unchanged
```

- [ ] **Step 2: Add recovery handling in `HandlePanelUpdateAvailable`**

Same pattern at the top of `HandlePanelUpdateAvailable()` (around line 328), after `config.GetVersion()`:

```go
func (s *ClusterSyncService) HandlePanelUpdateAvailable(ctx context.Context, domain *model.ClusterDomain, targetVersion string) (*ClusterPanelUpdateCheckResult, error) {
	if domain == nil {
		return nil, errClusterDomainNotFound
	}

	info, infoErr := s.getPanelUpdater().GetUpdateInfo()
	currentVersion := canonicalizeReleaseTag(config.GetVersion())

	if infoErr == nil && info != nil && info.Recovered {
		local, localErr := s.getLocalIdentity().GetOrCreate()
		if localErr == nil {
			_ = s.markLocalMemberOnline(ctx, domain, local.NodeID, currentVersion)
			_ = s.publishPanelUpdateStatus(ctx, domain, ClusterPanelUpdateStatusOnline, "", currentVersion, local.NodeID)
		}
	}

	latestVersion := canonicalizeReleaseTag(targetVersion)
	// ... rest unchanged
```

- [ ] **Step 3: Build to verify compilation**

Run: `cd b-ui/src/backend && go build ./...`
Expected: success

- [ ] **Step 4: Run all tests**

Run: `cd b-ui/src/backend && go test ./internal/domain/services/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd b-ui
git add src/backend/internal/domain/services/cluster_sync.go
git commit -m "feat(cluster-sync): recover online status after manual update"
```

---

### Task 8: Run full test suite and verify

**Files:** None

- [ ] **Step 1: Run the complete Go test suite**

Run: `cd b-ui/src/backend && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 2: Run `go vet` for static analysis**

Run: `cd b-ui/src/backend && go vet ./...`
Expected: No issues
