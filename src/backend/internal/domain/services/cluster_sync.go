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
)

var errClusterMemberNotFound = errors.New("cluster member not found")
var errClusterDomainNotFound = errors.New("cluster domain not found")

const (
	ClusterDomainUpdatePolicyAuto   = "auto"
	ClusterDomainUpdatePolicyManual = "manual"
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
		hubSyncer:    &ClusterHubSyncer{localIdentity: &ClusterLocalIdentityService{}},
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
				return false, err
			}
		}
	}
	return true, nil
}

func (s *ClusterSyncService) PollAndNotifyVersion(ctx context.Context) error {
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
			return err
		}
		if version <= domain.LastVersion {
			if version < domain.LastVersion {
				_, _ = s.CheckAndBroadcastUpdate(ctx, &domain)
				continue
			}
			needsDisplayNameBackfill, err := s.domainNeedsDisplayNameBackfill(domain.Id)
			if err != nil {
				return err
			}
			if !needsDisplayNameBackfill {
				_, _ = s.CheckAndBroadcastUpdate(ctx, &domain)
				continue
			}
		}
		if err := s.hubSyncer.SyncDomain(ctx, &domain, version); err != nil {
			var mirrorErr *clusterDomainMirrorRemovedError
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
	return removedMirrorErr
}

func (s *ClusterSyncService) domainNeedsDisplayNameBackfill(domainID uint) (bool, error) {
	members, err := s.store.GetMembers(domainID)
	if err != nil {
		return false, err
	}
	for _, member := range members {
		if strings.TrimSpace(member.DisplayName) == "" && strings.TrimSpace(member.Name) == "" {
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
	_ = s.markLocalMemberOffline(ctx, domain, localNodeID, currentVersion)
	if _, err := s.getPanelUpdater().StartUpdate(latestVersion, true); err != nil {
		return result, err
	}
	result.UpdateStarted = true
	return result, nil
}

func (s *ClusterSyncService) HandlePanelUpdateAvailable(ctx context.Context, domain *model.ClusterDomain, targetVersion string) (*ClusterPanelUpdateCheckResult, error) {
	if domain == nil {
		return nil, errClusterDomainNotFound
	}
	currentVersion := canonicalizeReleaseTag(config.GetVersion())
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
	if _, err := s.getPanelUpdater().StartUpdate(latestVersion, true); err != nil {
		return result, err
	}
	result.UpdateStarted = true
	return result, nil
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

func (s *ClusterSyncService) markLocalMemberOffline(ctx context.Context, domain *model.ClusterDomain, localNodeID string, currentVersion string) error {
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
	_, err = s.getUpdateHubClient().SetMemberStatus(ctx, domain.HubURL, domain.Domain, domainToken, requestID, localNodeID, "offline", currentVersion)
	return err
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
