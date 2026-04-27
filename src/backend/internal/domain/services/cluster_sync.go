package service

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alireza0/b-ui/src/backend/internal/domain/config"
	"github.com/alireza0/b-ui/src/backend/internal/infra/db/model"
)

var errClusterMemberNotFound = errors.New("cluster member not found")
var errClusterDomainNotFound = errors.New("cluster domain not found")

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

type ClusterSyncService struct {
	store        clusterSyncStore
	hubSyncer    clusterHubSyncer
	broadcaster  clusterBroadcaster
	panelService *PanelService
}

func NewRuntimeClusterSyncService() ClusterSyncService {
	return ClusterSyncService{
		store:        &dbClusterSyncStore{},
		hubSyncer:    &ClusterHubSyncer{},
		panelService: &PanelService{},
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
			continue
		}
		if err := s.hubSyncer.SyncDomain(ctx, &domain, version); err != nil {
			return err
		}

		_ = s.CheckAndBroadcastUpdate(ctx, &domain)
	}
	return nil
}

func (s *ClusterSyncService) CheckAndBroadcastUpdate(ctx context.Context, domain *model.ClusterDomain) error {
	members, err := s.store.GetMembers(domain.Id)
	if err != nil {
		return err
	}

	currentVersion := canonicalizeReleaseTag(config.GetVersion())
	maxVersion := currentVersion
	for _, member := range members {
		mv := canonicalizeReleaseTag(member.PanelVersion)
		if compareReleaseTags(mv, maxVersion) == "newer" {
			maxVersion = mv
		}
	}

	if compareReleaseTags(currentVersion, maxVersion) != "older" {
		return nil
	}

	hubClient := &ClusterHubClient{}
	secret, err := (&SettingService{}).GetSecret()
	if err != nil {
		return err
	}
	domainToken, err := DecryptClusterDomainToken(secret, domain.TokenEncrypted)
	if err != nil {
		return err
	}
	requestID := fmt.Sprintf("update-%d", time.Now().UnixNano())
	claimResp, err := hubClient.ClaimUpdate(ctx, domain.HubURL, domain.Domain, domainToken, requestID, maxVersion)
	if err != nil {
		return err
	}
	if !claimResp.Proceed {
		return nil
	}

	local, err := (&ClusterLocalIdentityService{}).GetOrCreate()
	if err != nil {
		return err
	}
	_, _ = hubClient.SetMemberStatus(ctx, domain.HubURL, domain.Domain, domainToken, requestID+"-status", local.NodeID, "offline", "")

	if s.broadcaster != nil {
		_ = s.broadcaster.BroadcastUpdateAvailable(ctx, domain.Id, domain.Domain, maxVersion, local.NodeID)
	}

	_, err = s.panelService.StartUpdate(maxVersion, true)
	return err
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
