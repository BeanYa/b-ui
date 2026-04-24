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
	panelUpdateMaxLogBytes        = 128 * 1024
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
	LogText       string `json:"logText,omitempty"`
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
	if reconciledState, changed := reconcilePanelUpdateStateWithCurrentVersion(state, currentVersion, time.Now()); changed {
		state = reconciledState
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
	if err := initializePanelUpdateLog(logPath, resolvedVersion, force, startedAt); err != nil {
		return nil, err
	}

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

func buildPanelUpdateCommand(targetVersion string, force bool, startedAt int64, logPath string) *exec.Cmd {
	installMode := "--update"
	if force {
		installMode = "--force-update"
	}

	updateScript := `set -eu
mkdir -p "$(dirname "$UPDATE_STATE_FILE")"
mkdir -p "$(dirname "$UPDATE_LOG_FILE")"
touch "$UPDATE_LOG_FILE"
log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >>"$UPDATE_LOG_FILE"
}
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
tmp_script="$(mktemp)"
cleanup() {
  rm -f "$tmp_script"
}
trap cleanup EXIT
log "准备更新面板，目标版本：$TARGET_VERSION"
log "更新模式：$INSTALL_MODE"
write_state running download_install_script
log "下载安装脚本：$INSTALL_SCRIPT_URL"
if ! curl -fsSL "$INSTALL_SCRIPT_URL" -o "$tmp_script" >>"$UPDATE_LOG_FILE" 2>&1; then
  log "下载安装脚本失败"
  write_state failed download_failed
  exit 1
fi
chmod +x "$tmp_script"
write_state running execute_install_script
log "执行安装脚本"
if bash "$tmp_script" "$INSTALL_MODE" "$TARGET_VERSION" >>"$UPDATE_LOG_FILE" 2>&1; then
  log "面板更新完成"
  write_state completed install_completed
else
  exit_code=$?
  log "安装脚本失败，退出码：$exit_code"
  write_state failed install_failed
  exit "$exit_code"
fi`

	cmd := exec.Command(
		"systemd-run",
		"--unit="+panelUpdateUnitName,
		"--collect",
		"--quiet",
		"--setenv=INSTALL_SCRIPT_URL="+panelInstallScriptURL(),
		"--setenv=INSTALL_MODE="+installMode,
		"--setenv=TARGET_VERSION="+targetVersion,
		"--setenv=UPDATE_FORCE_JSON="+strconv.FormatBool(force),
		"--setenv=UPDATE_STARTED_AT="+strconv.FormatInt(startedAt, 10),
		"--setenv=UPDATE_STATE_FILE="+panelUpdateStateFilePath(),
		"--setenv=UPDATE_LOG_FILE="+logPath,
		"/usr/bin/env",
		"bash",
		"-lc",
		updateScript,
	)
	return cmd
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
	hydratePanelUpdateStateLog(state)
	return state, nil
}

func hydratePanelUpdateStateLog(state *PanelUpdateState) {
	if state == nil || state.LogPath == "" {
		return
	}

	logText, err := readPanelUpdateLogTail(state.LogPath, panelUpdateMaxLogBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			state.LogText = "更新日志文件尚未创建：" + state.LogPath
			return
		}
		state.LogText = "读取更新日志失败：" + err.Error()
		return
	}
	state.LogText = logText
}

func readPanelUpdateLogTail(path string, maxBytes int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	offset := int64(0)
	if stat.Size() > maxBytes {
		offset = stat.Size() - maxBytes
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", err
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	if offset > 0 {
		return "...日志较长，仅显示最近内容...\n" + string(content), nil
	}
	return string(content), nil
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

func reconcilePanelUpdateStateWithCurrentVersion(state *PanelUpdateState, currentVersion string, now time.Time) (*PanelUpdateState, bool) {
	if state == nil || state.TargetVersion == "" || currentVersion == "" {
		return state, false
	}

	if compareReleaseTags(currentVersion, state.TargetVersion) == "older" {
		return state, false
	}

	switch state.Phase {
	case "failed":
		return nil, true
	case "running":
		state.Phase = "completed"
		state.Message = "current_version_reached"
		state.UpdatedAt = now.Unix()
		return state, true
	default:
		return state, false
	}
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

func saveOrClearPanelUpdateState(state *PanelUpdateState) error {
	if state == nil {
		err := os.Remove(panelUpdateStateFilePath())
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return savePanelUpdateState(state)
}

func panelUpdateStateFilePath() string {
	return filepath.Join(os.TempDir(), "b-ui-panel-update-state.json")
}

func panelUpdateLogFilePath() string {
	return filepath.Join(os.TempDir(), "b-ui-panel-update.log")
}

func initializePanelUpdateLog(logPath string, targetVersion string, force bool, startedAt int64) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}

	mode := "update"
	if force {
		mode = "force-update"
	}
	content := fmt.Sprintf(
		"[%s] 已确认面板更新\n目标版本：%s\n更新模式：%s\n",
		time.Unix(startedAt, 0).Format("2006-01-02 15:04:05"),
		targetVersion,
		mode,
	)
	return os.WriteFile(logPath, []byte(content), 0o644)
}

func resolvePanelUpdateLatestVersion(state *PanelUpdateState, fetchLatest func() (string, error)) (string, error) {
	if state != nil && state.Phase == "running" && state.TargetVersion != "" {
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
