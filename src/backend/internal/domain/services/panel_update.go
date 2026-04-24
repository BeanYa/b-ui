package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alireza0/s-ui/src/backend/internal/domain/config"
)

const (
	panelUpdateRepoOwner = "BeanYa"
	panelUpdateRepoName  = "b-ui"
	panelUpdateUnitName  = "b-ui-panel-update"

	panelUpdateRunningGracePeriod = 30 * time.Second
)

type PanelUpdateInfo struct {
	Supported         bool              `json:"supported"`
	UnsupportedReason string            `json:"unsupportedReason,omitempty"`
	CurrentVersion    string            `json:"currentVersion"`
	LatestVersion     string            `json:"latestVersion,omitempty"`
	Comparison        string            `json:"comparison"`
	UpdateAvailable   bool              `json:"updateAvailable"`
	ForceRequired     bool              `json:"forceRequired"`
	UpdateState       *PanelUpdateState `json:"updateState,omitempty"`
}

type PanelUpdateState struct {
	Phase         string `json:"phase"`
	TargetVersion string `json:"targetVersion"`
	Force         bool   `json:"force"`
	StartedAt     int64  `json:"startedAt"`
	UpdatedAt     int64  `json:"updatedAt"`
	LogPath       string `json:"logPath,omitempty"`
	Message       string `json:"message,omitempty"`
}

type PanelUpdateStartResult struct {
	TargetVersion string `json:"targetVersion"`
	Force         bool   `json:"force"`
	StartedAt     int64  `json:"startedAt"`
	LogPath       string `json:"logPath,omitempty"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func (s *PanelService) GetUpdateInfo() (*PanelUpdateInfo, error) {
	state, err := loadPanelUpdateState()
	if err != nil {
		return nil, err
	}

	currentVersion := canonicalizeReleaseTag(config.GetVersion())
	supported, reason := panelUpdateCapability()
	info := &PanelUpdateInfo{
		Supported:         supported,
		UnsupportedReason: reason,
		CurrentVersion:    currentVersion,
		Comparison:        "unknown",
		ForceRequired:     true,
		UpdateState:       state,
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
	state := &PanelUpdateState{
		Phase:         "running",
		TargetVersion: resolvedVersion,
		Force:         force,
		StartedAt:     startedAt,
		UpdatedAt:     startedAt,
		LogPath:       logPath,
	}
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

func panelUpdateCapability() (bool, string) {
	if runtime.GOOS != "linux" {
		return false, "automatic panel update currently requires a Linux host"
	}
	if _, err := exec.LookPath("systemd-run"); err != nil {
		return false, "automatic panel update requires systemd-run on the host"
	}
	if _, err := exec.LookPath("bash"); err != nil {
		return false, "automatic panel update requires bash on the host"
	}
	if _, err := exec.LookPath("curl"); err != nil {
		return false, "automatic panel update requires curl on the host"
	}
	return true, ""
}

func launchDetachedPanelUpdate(targetVersion string, force bool, startedAt int64, logPath string) error {
	installMode := "--update"
	if force {
		installMode = "--force-update"
	}

	updateScript := `set -eu
mkdir -p "$(dirname "$UPDATE_STATE_FILE")"
: >"$UPDATE_LOG_FILE"
write_state() {
  local phase="$1"
  local message="${2:-}"
  local now
  now=$(date +%s)
  if [ -n "$message" ]; then
    printf '{"phase":"%s","targetVersion":"%s","force":%s,"startedAt":%s,"updatedAt":%s,"logPath":"%s","message":"%s"}\n' "$phase" "$TARGET_VERSION" "$UPDATE_FORCE_JSON" "$UPDATE_STARTED_AT" "$now" "$UPDATE_LOG_FILE" "$message" > "$UPDATE_STATE_FILE"
    return
  fi
  printf '{"phase":"%s","targetVersion":"%s","force":%s,"startedAt":%s,"updatedAt":%s,"logPath":"%s"}\n' "$phase" "$TARGET_VERSION" "$UPDATE_FORCE_JSON" "$UPDATE_STARTED_AT" "$now" "$UPDATE_LOG_FILE" > "$UPDATE_STATE_FILE"
}
if bash <(curl -Ls "$INSTALL_SCRIPT_URL") "$INSTALL_MODE" "$TARGET_VERSION" >>"$UPDATE_LOG_FILE" 2>&1; then
  write_state completed
else
  write_state failed install_failed
  exit 1
fi`

	cmd := exec.Command(
		"systemd-run",
		"--unit="+panelUpdateUnitName,
		"--collect",
		"--quiet",
		"/usr/bin/env",
		"bash",
		"-lc",
		updateScript,
	)
	cmd.Env = append(os.Environ(),
		"INSTALL_SCRIPT_URL="+panelInstallScriptURL(),
		"INSTALL_MODE="+installMode,
		"TARGET_VERSION="+targetVersion,
		"UPDATE_FORCE_JSON="+strconv.FormatBool(force),
		"UPDATE_STARTED_AT="+strconv.FormatInt(startedAt, 10),
		"UPDATE_STATE_FILE="+panelUpdateStateFilePath(),
		"UPDATE_LOG_FILE="+logPath,
	)

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

func loadPanelUpdateState() (*PanelUpdateState, error) {
	content, err := os.ReadFile(panelUpdateStateFilePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	state := &PanelUpdateState{}
	if err := json.Unmarshal(content, state); err != nil {
		return nil, err
	}
	if reconciledState, changed := reconcilePanelUpdateState(state, time.Now(), panelUpdateUnitActive); changed {
		_ = savePanelUpdateState(reconciledState)
		state = reconciledState
	}
	return state, nil
}

func reconcilePanelUpdateState(state *PanelUpdateState, now time.Time, isUnitActive func() (bool, error)) (*PanelUpdateState, bool) {
	if state == nil || state.Phase != "running" {
		return state, false
	}

	lastUpdate := state.UpdatedAt
	if lastUpdate == 0 {
		lastUpdate = state.StartedAt
	}
	if lastUpdate > 0 && now.Unix()-lastUpdate < int64(panelUpdateRunningGracePeriod/time.Second) {
		return state, false
	}

	active, err := isUnitActive()
	if err != nil || active {
		return state, false
	}

	state.Phase = "failed"
	state.Message = "update_task_stopped"
	state.UpdatedAt = now.Unix()
	return state, true
}

func panelUpdateUnitActive() (bool, error) {
	err := exec.Command("systemctl", "is-active", "--quiet", panelUpdateUnitName).Run()
	if err == nil {
		return true, nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}
	return false, err
}

func savePanelUpdateState(state *PanelUpdateState) error {
	if err := os.MkdirAll(filepath.Dir(panelUpdateStateFilePath()), 0o755); err != nil {
		return err
	}

	content, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(panelUpdateStateFilePath(), content, 0o644)
}

func panelUpdateStateFilePath() string {
	return filepath.Join(os.TempDir(), "b-ui-panel-update-state.json")
}

func panelUpdateLogFilePath() string {
	return filepath.Join(os.TempDir(), "b-ui-panel-update.log")
}

func resolvePanelUpdateLatestVersion(state *PanelUpdateState, fetchLatest func() (string, error)) (string, error) {
	if state != nil && state.TargetVersion != "" {
		return canonicalizeReleaseTag(state.TargetVersion), nil
	}
	return fetchLatest()
}

func fetchLatestReleaseTag() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	latestURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", panelUpdateRepoOwner, panelUpdateRepoName)
	listURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=20", panelUpdateRepoOwner, panelUpdateRepoName)

	if tag, err := fetchReleaseTag(client, latestURL); err == nil && tag != "" {
		return canonicalizeReleaseTag(tag), nil
	}

	request, err := http.NewRequest(http.MethodGet, listURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "b-ui-panel")

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", githubHTTPError("release list request failed", response)
	}

	var releases []githubRelease
	if err := json.NewDecoder(response.Body).Decode(&releases); err != nil {
		return "", err
	}
	if len(releases) == 0 {
		return "", errors.New("no GitHub release found")
	}
	return canonicalizeReleaseTag(releases[0].TagName), nil
}

func fetchReleaseTag(client *http.Client, requestURL string) (string, error) {
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "b-ui-panel")

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", githubHTTPError("latest release request failed", response)
	}

	release := &githubRelease{}
	if err := json.NewDecoder(response.Body).Decode(release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

func githubHTTPError(prefix string, response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("%s: %s", prefix, response.Status)
	}
	return fmt.Errorf("%s: %s: %s", prefix, response.Status, message)
}

func panelInstallScriptURL() string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/install.sh", panelUpdateRepoOwner, panelUpdateRepoName)
}

func canonicalizeReleaseTag(version string) string {
	normalized := normalizeReleaseVersion(version)
	if normalized == "" {
		return ""
	}
	return "v" + normalized
}

func normalizeReleaseVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")
	return version
}

func compareReleaseTags(currentVersion string, targetVersion string) string {
	normalizedCurrent := normalizeReleaseVersion(currentVersion)
	normalizedTarget := normalizeReleaseVersion(targetVersion)
	if normalizedCurrent == "" || normalizedTarget == "" {
		return "unknown"
	}

	comparison, ok := compareNormalizedVersions(normalizedCurrent, normalizedTarget)
	if !ok {
		return "unknown"
	}
	if comparison < 0 {
		return "older"
	}
	if comparison > 0 {
		return "newer"
	}
	return "same"
}

func compareNormalizedVersions(left string, right string) (int, bool) {
	leftSegments, leftPrerelease, ok := splitVersionParts(left)
	if !ok {
		return 0, false
	}
	rightSegments, rightPrerelease, ok := splitVersionParts(right)
	if !ok {
		return 0, false
	}

	segmentCount := len(leftSegments)
	if len(rightSegments) > segmentCount {
		segmentCount = len(rightSegments)
	}
	for index := 0; index < segmentCount; index++ {
		leftValue := 0
		if index < len(leftSegments) {
			leftValue = leftSegments[index]
		}
		rightValue := 0
		if index < len(rightSegments) {
			rightValue = rightSegments[index]
		}
		if leftValue < rightValue {
			return -1, true
		}
		if leftValue > rightValue {
			return 1, true
		}
	}

	if leftPrerelease == rightPrerelease {
		return 0, true
	}
	if leftPrerelease == "" {
		return 1, true
	}
	if rightPrerelease == "" {
		return -1, true
	}

	if leftPrerelease < rightPrerelease {
		return -1, true
	}
	if leftPrerelease > rightPrerelease {
		return 1, true
	}
	return 0, true
}

func splitVersionParts(version string) ([]int, string, bool) {
	parts := strings.SplitN(version, "-", 2)
	segments := strings.Split(parts[0], ".")
	values := make([]int, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			return nil, "", false
		}
		value, err := strconv.Atoi(segment)
		if err != nil {
			return nil, "", false
		}
		values = append(values, value)
	}
	prerelease := ""
	if len(parts) == 2 {
		prerelease = parts[1]
	}
	return values, prerelease, true
}

func fallbackVersion(version string) string {
	if version == "" {
		return "unknown"
	}
	return version
}
