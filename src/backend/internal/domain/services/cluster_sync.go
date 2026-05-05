package service

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/config"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
)

var errClusterMemberNotFound = errors.New("cluster member not found")
var errClusterDomainNotFound = errors.New("cluster domain not found")

const (
	ClusterDomainUpdatePolicyAuto    = "auto"
	ClusterDomainUpdatePolicyManual  = "manual"
	ClusterDomainDefaultTimeLocation = "Asia/Shanghai"
)

const (
	ClusterPanelUpdateStatusUpdating = "updating"
	ClusterPanelUpdateStatusOnline   = "online"

	clusterPanelUpdateWatchInterval = 30 * time.Second
	clusterPanelUpdateWatchTimeout  = 30 * time.Minute
)

type ClusterEnvelope struct {
	SchemaVersion int    `json:"schemaVersion"`
	MessageType   string `json:"messageType"`
	SourceNodeID  string `json:"sourceNodeId"`
	Domain        string `json:"domain"`
	SentAt        int64  `json:"sentAt"`
	Version       int64  `json:"version"`
	Signature     string `json:"signature"`
}

type ClusterNotifyVersionMessage struct {
	SourceNodeID string
	Domain       string
	SentAt       int64
	Version      int64
}

func SignClusterNotifyVersionEnvelope(local *model.ClusterLocalNode, domain string, version int64, sentAt int64) (*ClusterEnvelope, error) {
	privateKeyRaw, err := base64.StdEncoding.DecodeString(local.PrivateKey)
	if err != nil {
		return nil, err
	}
	envelope := &ClusterEnvelope{
		SchemaVersion: 1,
		MessageType:   "sync.notify_version",
		SourceNodeID:  local.NodeID,
		Domain:        domain,
		SentAt:        sentAt,
		Version:       version,
	}
	payload, err := clusterEnvelopePayload(envelope)
	if err != nil {
		return nil, err
	}
	envelope.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(privateKeyRaw), payload))
	return envelope, nil
}

func VerifyClusterEnvelope(envelope *ClusterEnvelope, publicKey string) (*ClusterNotifyVersionMessage, error) {
	if envelope.SchemaVersion != 1 {
		return nil, errors.New("unsupported cluster message version")
	}
	if envelope.MessageType != "sync.notify_version" {
		return nil, errors.New("unsupported cluster message type")
	}
	publicKeyRaw, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return nil, err
	}
	signatureRaw, err := base64.StdEncoding.DecodeString(envelope.Signature)
	if err != nil {
		return nil, err
	}
	payload, err := clusterEnvelopePayload(envelope)
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKeyRaw), payload, signatureRaw) {
		return nil, errors.New("invalid cluster message signature")
	}
	return &ClusterNotifyVersionMessage{
		SourceNodeID: envelope.SourceNodeID,
		Domain:       envelope.Domain,
		SentAt:       envelope.SentAt,
		Version:      envelope.Version,
	}, nil
}

type clusterSyncStore interface {
	GetMember(domainID uint, nodeID string) (*model.ClusterMember, error)
	GetMembers(domainID uint) ([]model.ClusterMember, error)
	SaveMember(*model.ClusterMember) error
	ListMembers() ([]model.ClusterMember, error)
	GetDomain(id uint) (*model.ClusterDomain, error)
	SaveDomain(*model.ClusterDomain) error
	ListDomains() ([]model.ClusterDomain, error)
}

type clusterBroadcaster interface {
	BroadcastNotifyVersion(context.Context, int64, string) error
	BroadcastUpdateAvailable(context.Context, uint, string, string, string) error
	BroadcastUpdateStatus(context.Context, uint, string, string, string, string, string) error
}

type clusterHubSyncer interface {
	LatestVersion(context.Context, *model.ClusterDomain) (int64, error)
	SyncDomain(context.Context, *model.ClusterDomain, int64) error
}

type clusterPanelUpdater interface {
	GetUpdateInfo() (*PanelUpdateInfo, error)
	StartUpdate(targetVersion string, force bool) (*PanelUpdateStartResult, error)
}

type clusterUpdateHubClient interface {
	ClaimUpdate(context.Context, string, string, string, string, string) (*ClusterHubClaimUpdateResponse, error)
	SetMemberStatus(context.Context, string, string, string, string, string, string, string) (*ClusterHubMemberStatusResponse, error)
}

type ClusterPanelUpdateCheckResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	Comparison      string `json:"comparison"`
	UpdateAvailable bool   `json:"updateAvailable"`
	UpdatePolicy    string `json:"updatePolicy"`
	AutoUpdate      bool   `json:"autoUpdate"`
	UpdateStarted   bool   `json:"updateStarted"`
}

type ClusterPanelMemberUpdateResult struct {
	NodeID         string `json:"nodeId"`
	CurrentVersion string `json:"currentVersion"`
	TargetVersion  string `json:"targetVersion,omitempty"`
	Status         string `json:"status"`
	UpdateStarted  bool   `json:"updateStarted"`
}

type ClusterSyncService struct {
	store          clusterSyncStore
	hubSyncer      clusterHubSyncer
	broadcaster    clusterBroadcaster
	panelService   clusterPanelUpdater
	hubClient      clusterUpdateHubClient
	secretProvider clusterSecretProvider
	localIdentity  clusterLocalIdentityProvider
}

func NewRuntimeClusterSyncService() ClusterSyncService {
	return ClusterSyncService{
		store:        &dbClusterSyncStore{},
		hubSyncer:    &ClusterHubSyncer{localIdentity: &ClusterLocalIdentityService{}, timeLocationSyncer: newRuntimeClusterTimeLocationSyncer()},
		broadcaster:  &ClusterHTTPBroadcaster{},
		panelService: &PanelService{},
		hubClient:    &ClusterHubClient{},
	}
}

func (s *ClusterSyncService) HandleIncomingNotifyVersion(ctx context.Context, domainID uint, nodeID string, version int64) (bool, error) {
	member, err := s.store.GetMember(domainID, nodeID)
	if err != nil {
		return false, err
	}
	if version <= member.LastVersion {
		return false, nil
	}
	member.LastVersion = version
	member.LastNotifiedValue = version
	if err := s.store.SaveMember(member); err != nil {
		return false, err
	}
	if s.hubSyncer != nil && member.DomainID > 0 {
		domain, err := s.store.GetDomain(member.DomainID)
		if err != nil {
			return false, err
		}
		if domain.HubURL != "" {
			if err := s.hubSyncer.SyncDomain(ctx, domain, version); err != nil {
				logger.ClusterError(logger.ClusterCron, "version_poll.sync_domain", map[string]interface{}{"domain": domain.Domain, "version": version, "error": err.Error()})
				return false, err
			}
		}
	}
	return true, nil
}

func (s *ClusterSyncService) SyncNow(ctx context.Context) error {
	return s.pollAndNotifyVersion(ctx, true)
}

func (s *ClusterSyncService) pollAndNotifyVersion(ctx context.Context, forceSnapshot bool) error {
	if s.store == nil || s.hubSyncer == nil {
		return nil
	}
	domains, err := s.store.ListDomains()
	if err != nil {
		return err
	}
	var removedMirrorErr error
	for i := range domains {
		domain := domains[i]
		if domain.HubURL == "" {
			continue
		}
		version, err := s.hubSyncer.LatestVersion(ctx, &domain)
		if err != nil {
			logger.ClusterError(logger.ClusterCron, "version_poll.latest_version", map[string]interface{}{"domain": domain.Domain, "error": err.Error()})
			return err
		}
		if version <= domain.LastVersion {
			if version < domain.LastVersion {
				_, _ = s.CheckAndBroadcastUpdate(ctx, &domain)
				continue
			}
			needsSnapshotRefresh, err := s.domainNeedsSnapshotRefresh(domain.Id)
			if err != nil {
				return err
			}
			if !forceSnapshot && !needsSnapshotRefresh {
				_, _ = s.CheckAndBroadcastUpdate(ctx, &domain)
				continue
			}
		}
		if err := s.hubSyncer.SyncDomain(ctx, &domain, version); err != nil {
			var mirrorErr *clusterDomainMirrorRemovedError
			logger.ClusterError(logger.ClusterCron, "version_poll.sync_domain", map[string]interface{}{"domain": domain.Domain, "version": version, "error": err.Error()})
			if errors.As(err, &mirrorErr) {
				if removedMirrorErr == nil {
					removedMirrorErr = mirrorErr
				}
				continue
			}
			return err
		}

		_, _ = s.CheckAndBroadcastUpdate(ctx, &domain)
	}

	// Reconcile proxy configs with hub during explicit sync.
	s.ReconcileProxyConfigs()

	return removedMirrorErr
}

// ReconcileProxyConfigs reports current inbound configs to all hub domains.
func (s *ClusterSyncService) ReconcileProxyConfigs() {
	if s.store == nil {
		return
	}
	provider := &syncMemberProvider{
		store:          s.store,
		secretProvider: s.getSecretProvider(),
		localIdentity:  s.getLocalIdentity(),
	}
	report := NewClusterProxyReportService(&ClusterHubClient{}, nil, provider, s)
	report.ReportForAllDomains()
}

func (s *ClusterSyncService) ReportLocalPanelStatus(ctx context.Context) error {
	if s.store == nil {
		return nil
	}

	domains, err := s.store.ListDomains()
	if err != nil {
		return err
	}
	reportDomains := make([]model.ClusterDomain, 0, len(domains))
	for index := range domains {
		if strings.TrimSpace(domains[index].Domain) == "" {
			continue
		}
		reportDomains = append(reportDomains, domains[index])
	}
	if len(reportDomains) == 0 {
		return nil
	}

	currentVersion := canonicalizeReleaseTag(config.GetVersion())
	if localPanelUpdateStillRunning(currentVersion) {
		return nil
	}

	local, err := s.getLocalIdentity().GetOrCreate()
	if err != nil {
		return err
	}
	if strings.TrimSpace(local.NodeID) == "" {
		return nil
	}

	var failures []string
	for index := range reportDomains {
		domain := reportDomains[index]
		localChanged, localPresent, err := s.localPanelStatusChanged(domain.Id, local.NodeID, currentVersion)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s local: %v", domain.Domain, err))
			continue
		}
		if !localPresent {
			continue
		}
		if err := s.saveLocalMemberStatus(ctx, &domain, local.NodeID, ClusterPanelUpdateStatusOnline, currentVersion); err != nil {
			failures = append(failures, fmt.Sprintf("%s local: %v", domain.Domain, err))
		}
		if err := s.notifyHubMemberStatus(ctx, &domain, local.NodeID, ClusterPanelUpdateStatusOnline, currentVersion); err != nil {
			failures = append(failures, fmt.Sprintf("%s hub: %v", domain.Domain, err))
		}
		if localChanged {
			if err := s.publishPanelUpdateStatus(ctx, &domain, ClusterPanelUpdateStatusOnline, "", currentVersion, local.NodeID); err != nil {
				failures = append(failures, fmt.Sprintf("%s peers: %v", domain.Domain, err))
			}
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func (s *ClusterSyncService) localPanelStatusChanged(domainID uint, localNodeID string, currentVersion string) (bool, bool, error) {
	member, err := s.store.GetMember(domainID, localNodeID)
	if err != nil || member == nil {
		if errors.Is(err, errClusterMemberNotFound) || member == nil {
			return false, false, nil
		}
		return false, false, err
	}
	if member.Status != ClusterPanelUpdateStatusOnline {
		return true, true, nil
	}
	return currentVersion != "" && canonicalizeReleaseTag(member.PanelVersion) != currentVersion, true, nil
}

func (s *ClusterSyncService) GetLocalNodeID() string {
	local, err := s.getLocalIdentity().GetOrCreate()
	if err != nil {
		return ""
	}
	return local.NodeID
}

type syncMemberProvider struct {
	store          clusterSyncStore
	secretProvider clusterSecretProvider
	localIdentity  clusterLocalIdentityProvider
}

func (p *syncMemberProvider) GetAllDomains() ([]clusterDomainInfo, error) {
	domains, err := p.store.ListDomains()
	if err != nil {
		return nil, err
	}
	reportDomains := make([]model.ClusterDomain, 0, len(domains))
	for _, domain := range domains {
		if strings.TrimSpace(domain.HubURL) == "" || strings.TrimSpace(domain.TokenEncrypted) == "" {
			continue
		}
		reportDomains = append(reportDomains, domain)
	}
	if len(reportDomains) == 0 {
		return nil, nil
	}
	localIdentity, err := p.localIdentity.GetOrCreate()
	if err != nil {
		return nil, err
	}
	secret, err := p.secretProvider.GetSecret()
	if err != nil {
		return nil, err
	}
	var result []clusterDomainInfo
	for _, domain := range reportDomains {
		domainToken, err := DecryptClusterDomainToken(secret, domain.TokenEncrypted)
		if err != nil {
			continue
		}
		member, err := p.store.GetMember(domain.Id, localIdentity.NodeID)
		if err != nil || member == nil {
			continue
		}
		result = append(result, clusterDomainInfo{
			ID:          domain.Id,
			Name:        domain.Domain,
			MemberID:    localIdentity.NodeID,
			BaseURL:     member.BaseURL,
			HubURL:      domain.HubURL,
			DomainToken: domainToken,
		})
	}
	return result, nil
}

func localPanelUpdateStillRunning(currentVersion string) bool {
	state, err := loadPanelUpdateState()
	if err != nil || state == nil {
		return false
	}
	reconciledState, changed, _ := reconcilePanelUpdateStateWithCurrentVersion(state, currentVersion, time.Now())
	if changed {
		_ = saveOrClearPanelUpdateState(reconciledState)
		state = reconciledState
	}
	if state == nil {
		return false
	}
	if state.Phase != "running" && state.Phase != "preflight" {
		return false
	}
	targetVersion := canonicalizeReleaseTag(state.TargetVersion)
	if targetVersion == "" {
		return true
	}
	comparison := compareReleaseTags(currentVersion, targetVersion)
	return comparison == "older" || comparison == "unknown"
}

func (s *ClusterSyncService) domainNeedsSnapshotRefresh(domainID uint) (bool, error) {
	members, err := s.store.GetMembers(domainID)
	if err != nil {
		return false, err
	}
	for _, member := range members {
		if strings.TrimSpace(member.DisplayName) == "" && strings.TrimSpace(member.Name) == "" {
			return true, nil
		}
		status := strings.TrimSpace(member.Status)
		if status != "" && status != ClusterPanelUpdateStatusOnline {
			return true, nil
		}
	}
	return false, nil
}

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
	comparison := compareReleaseTags(currentVersion, latestVersion)
	updateAvailable := latestVersion != "" && comparison == "older"
	result := &ClusterPanelUpdateCheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVersion,
		Comparison:      comparison,
		UpdateAvailable: updateAvailable,
		UpdatePolicy:    effectiveClusterDomainUpdatePolicy(domain.UpdatePolicy),
	}
	if err := s.saveDomainPanelUpdateState(domain, latestVersion, updateAvailable); err != nil {
		return nil, err
	}
	if !updateAvailable {
		return result, nil
	}
	if claimProceed, claimedVersion, err := s.claimDomainPanelUpdate(ctx, domain, latestVersion); err != nil {
		return nil, err
	} else if !claimProceed {
		return result, nil
	} else if claimedVersion != "" {
		latestVersion = claimedVersion
		result.LatestVersion = latestVersion
	}

	autoUpdate, err := s.shouldAutoUpdate(domain)
	if err != nil {
		return nil, err
	}
	result.AutoUpdate = autoUpdate

	localNodeID := ""
	if s.broadcaster != nil || autoUpdate {
		local, err := s.getLocalIdentity().GetOrCreate()
		if err != nil {
			return nil, err
		}
		localNodeID = local.NodeID
	}
	if s.broadcaster != nil {
		_ = s.broadcaster.BroadcastUpdateAvailable(ctx, domain.Id, domain.Domain, latestVersion, localNodeID)
	}
	if !autoUpdate {
		return result, nil
	}
	_ = s.markLocalMemberUpdating(ctx, domain, localNodeID, currentVersion)
	_ = s.notifyHubMemberStatus(ctx, domain, localNodeID, "offline", currentVersion)
	_ = s.publishPanelUpdateStatus(ctx, domain, ClusterPanelUpdateStatusUpdating, latestVersion, currentVersion, localNodeID)
	if _, err := s.getPanelUpdater().StartUpdate(latestVersion, true); err != nil {
		_ = s.markLocalMemberOnline(ctx, domain, localNodeID, currentVersion)
		_ = s.publishPanelUpdateStatus(ctx, domain, ClusterPanelUpdateStatusOnline, latestVersion, currentVersion, localNodeID)
		return result, err
	}
	result.UpdateStarted = true
	s.startPanelUpdateCompletionWatch(domain, localNodeID, latestVersion)
	return result, nil
}

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
	comparison := compareReleaseTags(currentVersion, latestVersion)
	updateAvailable := latestVersion != "" && comparison == "older"
	result := &ClusterPanelUpdateCheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVersion,
		Comparison:      comparison,
		UpdateAvailable: updateAvailable,
		UpdatePolicy:    effectiveClusterDomainUpdatePolicy(domain.UpdatePolicy),
	}
	if err := s.saveDomainPanelUpdateState(domain, latestVersion, updateAvailable); err != nil {
		return nil, err
	}
	if !updateAvailable {
		return result, nil
	}
	autoUpdate, err := s.shouldAutoUpdate(domain)
	if err != nil {
		return nil, err
	}
	result.AutoUpdate = autoUpdate
	if !autoUpdate {
		return result, nil
	}
	local, err := s.getLocalIdentity().GetOrCreate()
	if err != nil {
		return nil, err
	}
	_ = s.markLocalMemberUpdating(ctx, domain, local.NodeID, currentVersion)
	_ = s.notifyHubMemberStatus(ctx, domain, local.NodeID, "offline", currentVersion)
	_ = s.publishPanelUpdateStatus(ctx, domain, ClusterPanelUpdateStatusUpdating, latestVersion, currentVersion, local.NodeID)
	if _, err := s.getPanelUpdater().StartUpdate(latestVersion, true); err != nil {
		_ = s.markLocalMemberOnline(ctx, domain, local.NodeID, currentVersion)
		_ = s.publishPanelUpdateStatus(ctx, domain, ClusterPanelUpdateStatusOnline, latestVersion, currentVersion, local.NodeID)
		return result, err
	}
	result.UpdateStarted = true
	s.startPanelUpdateCompletionWatch(domain, local.NodeID, latestVersion)
	return result, nil
}

func (s *ClusterSyncService) HandlePanelUpdateRequest(ctx context.Context, domain *model.ClusterDomain, targetVersion string) (*ClusterPanelUpdateCheckResult, error) {
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
	latestVersion := canonicalizeReleaseTag(targetVersion)
	if latestVersion == "" {
		latestVersion = canonicalizeReleaseTag(info.LatestVersion)
	}
	comparison := compareReleaseTags(currentVersion, latestVersion)
	updateAvailable := latestVersion != "" && comparison == "older"
	result := &ClusterPanelUpdateCheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVersion,
		Comparison:      comparison,
		UpdateAvailable: updateAvailable,
		UpdatePolicy:    effectiveClusterDomainUpdatePolicy(domain.UpdatePolicy),
	}
	if err := s.saveDomainPanelUpdateState(domain, latestVersion, updateAvailable); err != nil {
		return nil, err
	}

	local, err := s.getLocalIdentity().GetOrCreate()
	if err != nil {
		return nil, err
	}
	if !updateAvailable {
		_ = s.markLocalMemberOnline(ctx, domain, local.NodeID, currentVersion)
		_ = s.publishPanelUpdateStatus(ctx, domain, ClusterPanelUpdateStatusOnline, latestVersion, currentVersion, local.NodeID)
		return result, nil
	}

	if err := s.markLocalMemberUpdating(ctx, domain, local.NodeID, currentVersion); err != nil {
		return nil, err
	}
	_ = s.notifyHubMemberStatus(ctx, domain, local.NodeID, "offline", currentVersion)
	_ = s.publishPanelUpdateStatus(ctx, domain, ClusterPanelUpdateStatusUpdating, latestVersion, currentVersion, local.NodeID)
	if _, err := s.getPanelUpdater().StartUpdate(latestVersion, false); err != nil {
		_ = s.markLocalMemberOnline(ctx, domain, local.NodeID, currentVersion)
		_ = s.publishPanelUpdateStatus(ctx, domain, ClusterPanelUpdateStatusOnline, latestVersion, currentVersion, local.NodeID)
		return result, err
	}
	result.UpdateStarted = true
	s.startPanelUpdateCompletionWatch(domain, local.NodeID, latestVersion)
	return result, nil
}

func (s *ClusterSyncService) HandlePanelUpdateStatus(_ context.Context, domain *model.ClusterDomain, nodeID string, status string, targetVersion string, panelVersion string) error {
	if domain == nil {
		return errClusterDomainNotFound
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return errors.New("invalid_payload_status")
	}
	if nodeID == "" {
		return errClusterMemberNotFound
	}
	if s.store == nil {
		return nil
	}
	member, err := s.store.GetMember(domain.Id, nodeID)
	if err != nil {
		return err
	}
	member.Status = status
	if version := canonicalizeReleaseTag(panelVersion); version != "" {
		member.PanelVersion = version
	}
	if target := canonicalizeReleaseTag(targetVersion); status == ClusterPanelUpdateStatusOnline && target != "" {
		if member.PanelVersion == "" || compareReleaseTags(member.PanelVersion, target) == "older" {
			member.PanelVersion = target
		}
	}
	return s.store.SaveMember(member)
}

func (s *ClusterSyncService) claimDomainPanelUpdate(ctx context.Context, domain *model.ClusterDomain, targetVersion string) (bool, string, error) {
	if domain.HubURL == "" || domain.TokenEncrypted == "" {
		return true, targetVersion, nil
	}
	secret, err := s.getSecretProvider().GetSecret()
	if err != nil {
		return false, "", err
	}
	domainToken, err := DecryptClusterDomainToken(secret, domain.TokenEncrypted)
	if err != nil {
		return false, "", err
	}
	requestID := fmt.Sprintf("update-%d", time.Now().UnixNano())
	claimResp, err := s.getUpdateHubClient().ClaimUpdate(ctx, domain.HubURL, domain.Domain, domainToken, requestID, targetVersion)
	if err != nil {
		return false, "", err
	}
	if claimResp == nil {
		return true, targetVersion, nil
	}
	claimedVersion := canonicalizeReleaseTag(claimResp.TargetVersion)
	if claimedVersion == "" {
		claimedVersion = targetVersion
	}
	return claimResp.Proceed, claimedVersion, nil
}

func (s *ClusterSyncService) markLocalMemberUpdating(ctx context.Context, domain *model.ClusterDomain, localNodeID string, currentVersion string) error {
	return s.saveLocalMemberStatus(ctx, domain, localNodeID, ClusterPanelUpdateStatusUpdating, currentVersion)
}

func (s *ClusterSyncService) markLocalMemberOnline(ctx context.Context, domain *model.ClusterDomain, localNodeID string, panelVersion string) error {
	if err := s.saveLocalMemberStatus(ctx, domain, localNodeID, ClusterPanelUpdateStatusOnline, panelVersion); err != nil {
		return err
	}
	return s.notifyHubMemberStatus(ctx, domain, localNodeID, ClusterPanelUpdateStatusOnline, panelVersion)
}

func (s *ClusterSyncService) saveLocalMemberStatus(_ context.Context, domain *model.ClusterDomain, localNodeID string, status string, panelVersion string) error {
	if s.store == nil || domain == nil || localNodeID == "" {
		return nil
	}
	member, err := s.store.GetMember(domain.Id, localNodeID)
	if err != nil {
		if errors.Is(err, errClusterMemberNotFound) {
			return nil
		}
		return err
	}
	member.Status = status
	if version := canonicalizeReleaseTag(panelVersion); version != "" {
		member.PanelVersion = version
	}
	return s.store.SaveMember(member)
}

func (s *ClusterSyncService) notifyHubMemberStatus(ctx context.Context, domain *model.ClusterDomain, localNodeID string, status string, panelVersion string) error {
	if domain.HubURL == "" || domain.TokenEncrypted == "" || localNodeID == "" {
		return nil
	}
	secret, err := s.getSecretProvider().GetSecret()
	if err != nil {
		return err
	}
	domainToken, err := DecryptClusterDomainToken(secret, domain.TokenEncrypted)
	if err != nil {
		return err
	}
	requestID := fmt.Sprintf("update-status-%d", time.Now().UnixNano())
	_, err = s.getUpdateHubClient().SetMemberStatus(ctx, domain.HubURL, domain.Domain, domainToken, requestID, localNodeID, status, panelVersion)
	return err
}

func (s *ClusterSyncService) publishPanelUpdateStatus(ctx context.Context, domain *model.ClusterDomain, status string, targetVersion string, panelVersion string, excludeNodeID string) error {
	if s.broadcaster == nil || domain == nil {
		return nil
	}
	return s.broadcaster.BroadcastUpdateStatus(ctx, domain.Id, domain.Domain, status, targetVersion, panelVersion, excludeNodeID)
}

func (s *ClusterSyncService) startPanelUpdateCompletionWatch(domain *model.ClusterDomain, localNodeID string, targetVersion string) {
	if domain == nil || localNodeID == "" || targetVersion == "" {
		return
	}
	domainCopy := *domain
	go func() {
		ticker := time.NewTicker(clusterPanelUpdateWatchInterval)
		defer ticker.Stop()
		timeout := time.After(clusterPanelUpdateWatchTimeout)
		for {
			select {
			case <-ticker.C:
				info, err := s.getPanelUpdater().GetUpdateInfo()
				if err != nil {
					continue
				}
				currentVersion := canonicalizeReleaseTag(info.CurrentVersion)
				if currentVersion == "" {
					currentVersion = canonicalizeReleaseTag(config.GetVersion())
				}
				finished := currentVersion != "" && compareReleaseTags(currentVersion, targetVersion) != "older"
				failed := info.UpdateState != nil && info.UpdateState.Phase == "failed"
				if !finished && !failed {
					continue
				}
				_ = s.markLocalMemberOnline(context.Background(), &domainCopy, localNodeID, currentVersion)
				_ = s.publishPanelUpdateStatus(context.Background(), &domainCopy, ClusterPanelUpdateStatusOnline, targetVersion, currentVersion, localNodeID)
				return
			case <-timeout:
				return
			}
		}
	}()
}

func (s *ClusterSyncService) saveDomainPanelUpdateState(domain *model.ClusterDomain, latestVersion string, updateAvailable bool) error {
	domain.UpdatePolicy = effectiveClusterDomainUpdatePolicy(domain.UpdatePolicy)
	if latestVersion != "" {
		domain.LatestPanelVersion = latestVersion
	}
	domain.PanelUpdateAvailable = updateAvailable
	if s.store == nil {
		return nil
	}
	return s.store.SaveDomain(domain)
}

func (s *ClusterSyncService) shouldAutoUpdate(domain *model.ClusterDomain) (bool, error) {
	if s.store == nil {
		return effectiveClusterDomainUpdatePolicy(domain.UpdatePolicy) == ClusterDomainUpdatePolicyAuto, nil
	}
	domains, err := s.store.ListDomains()
	if err != nil {
		return false, err
	}
	if len(domains) == 0 {
		return effectiveClusterDomainUpdatePolicy(domain.UpdatePolicy) == ClusterDomainUpdatePolicyAuto, nil
	}
	for _, item := range domains {
		if effectiveClusterDomainUpdatePolicy(item.UpdatePolicy) == ClusterDomainUpdatePolicyAuto {
			return true, nil
		}
	}
	return false, nil
}

func effectiveClusterDomainUpdatePolicy(value string) string {
	if strings.TrimSpace(value) == ClusterDomainUpdatePolicyManual {
		return ClusterDomainUpdatePolicyManual
	}
	return ClusterDomainUpdatePolicyAuto
}

func effectiveClusterDomainTimeLocation(value string) string {
	timeLocation := strings.TrimSpace(value)
	if timeLocation == "" {
		return ClusterDomainDefaultTimeLocation
	}
	return timeLocation
}

func (s *ClusterSyncService) getPanelUpdater() clusterPanelUpdater {
	if s.panelService != nil {
		return s.panelService
	}
	s.panelService = &PanelService{}
	return s.panelService
}

func (s *ClusterSyncService) getUpdateHubClient() clusterUpdateHubClient {
	if s.hubClient != nil {
		return s.hubClient
	}
	s.hubClient = &ClusterHubClient{}
	return s.hubClient
}

func (s *ClusterSyncService) getSecretProvider() clusterSecretProvider {
	if s.secretProvider != nil {
		return s.secretProvider
	}
	return &SettingService{}
}

func (s *ClusterSyncService) getLocalIdentity() clusterLocalIdentityProvider {
	if s.localIdentity != nil {
		return s.localIdentity
	}
	s.localIdentity = &ClusterLocalIdentityService{}
	return s.localIdentity
}

func clusterEnvelopePayload(envelope *ClusterEnvelope) ([]byte, error) {
	unsigned := struct {
		SchemaVersion int    `json:"schemaVersion"`
		MessageType   string `json:"messageType"`
		SourceNodeID  string `json:"sourceNodeId"`
		Domain        string `json:"domain"`
		SentAt        int64  `json:"sentAt"`
		Version       int64  `json:"version"`
	}{
		SchemaVersion: envelope.SchemaVersion,
		MessageType:   envelope.MessageType,
		SourceNodeID:  envelope.SourceNodeID,
		Domain:        envelope.Domain,
		SentAt:        envelope.SentAt,
		Version:       envelope.Version,
	}
	return json.Marshal(unsigned)
}
