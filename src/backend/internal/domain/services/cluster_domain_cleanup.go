package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	"gorm.io/gorm"
)

type clusterDomainCleanupBroadcaster interface {
	BroadcastDomainCleanup(context.Context, *model.ClusterDomain, clustertypes.DomainCleanupPayload) error
}

type ClusterDomainCleanupServiceOptions struct {
	DB          *gorm.DB
	Broadcaster clusterDomainCleanupBroadcaster
	Now         func() int64
}

type ClusterDomainCleanupService struct {
	db          *gorm.DB
	broadcaster clusterDomainCleanupBroadcaster
	now         func() int64
}

type DomainCleanupResult struct {
	RequestID       string `json:"request_id"`
	ClientsDeleted  int    `json:"clients_deleted"`
	InboundsDeleted int    `json:"inbounds_deleted"`
	TLSDeleted      int    `json:"tls_deleted"`
	DomainDeleted   bool   `json:"domain_deleted"`
}

func NewClusterDomainCleanupService(opts ClusterDomainCleanupServiceOptions) *ClusterDomainCleanupService {
	s := &ClusterDomainCleanupService{
		db:          opts.DB,
		broadcaster: opts.Broadcaster,
		now:         opts.Now,
	}
	if s.db == nil {
		s.db = database.GetDB()
	}
	if s.now == nil {
		s.now = func() int64 { return time.Now().Unix() }
	}
	return s
}

func (s *ClusterDomainCleanupService) CleanupLocalDomainResources(ctx context.Context, domain *model.ClusterDomain) (*DomainCleanupResult, error) {
	if s == nil {
		return nil, errors.New("cluster domain cleanup service is required")
	}
	payload := clustertypes.DomainCleanupPayload{
		RequestID: fmt.Sprintf("local-cleanup-%s-%d", strings.TrimSpace(domain.Domain), s.now()),
		DomainID:  strings.TrimSpace(domain.Domain),
	}
	if s.db == nil {
		return &DomainCleanupResult{RequestID: payload.RequestID}, nil
	}
	return s.cleanupDomain(ctx, domain, payload, false, false)
}

func (s *ClusterDomainCleanupService) ApplyDomainCleanup(ctx context.Context, domain *model.ClusterDomain, payload clustertypes.DomainCleanupPayload, _ string, broadcast bool) (*DomainCleanupResult, error) {
	return s.cleanupDomain(ctx, domain, payload, broadcast, true)
}

func (s *ClusterDomainCleanupService) HandleDomainCleanup(ctx context.Context, req clustertypes.ActionRequest, payload clustertypes.DomainCleanupPayload) (map[string]interface{}, error) {
	domain, err := (&dbClusterStore{}).GetDomainByName(req.Domain)
	if err != nil {
		return nil, err
	}
	result, err := s.ApplyDomainCleanup(ctx, domain, payload, req.SourceNodeID, true)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"request_id":       result.RequestID,
		"clients_deleted":  result.ClientsDeleted,
		"inbounds_deleted": result.InboundsDeleted,
		"tls_deleted":      result.TLSDeleted,
		"domain_deleted":   result.DomainDeleted,
	}, nil
}

func (s *ClusterDomainCleanupService) cleanupDomain(ctx context.Context, domain *model.ClusterDomain, payload clustertypes.DomainCleanupPayload, broadcast bool, deleteDomainMirror bool) (*DomainCleanupResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, errors.New("cluster domain cleanup service is required")
	}
	if domain == nil || domain.Id == 0 || strings.TrimSpace(domain.Domain) == "" {
		return nil, errors.New("local domain is required")
	}
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	payload.DomainID = strings.TrimSpace(payload.DomainID)
	if payload.RequestID == "" {
		return nil, errors.New("request_id is required")
	}
	if payload.DomainID != "" && payload.DomainID != domain.Domain {
		return nil, fmt.Errorf("payload domain_id %q does not match local domain %q", payload.DomainID, domain.Domain)
	}

	result := &DomainCleanupResult{RequestID: payload.RequestID}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		counts, err := cleanupClusterDomainResources(tx, domain.Id)
		if err != nil {
			return err
		}
		result.ClientsDeleted = counts.ClientsDeleted
		result.InboundsDeleted = counts.InboundsDeleted
		result.TLSDeleted = counts.TLSDeleted
		return nil
	}); err != nil {
		return nil, err
	}

	if broadcast && s.broadcaster != nil {
		if err := s.broadcaster.BroadcastDomainCleanup(ctx, domain, payload); err != nil {
			return nil, err
		}
	}
	if deleteDomainMirror {
		if err := (&dbClusterStore{}).DeleteDomain(domain.Id); err != nil {
			return nil, err
		}
		result.DomainDeleted = true
	}
	LastUpdate = time.Now().Unix()
	return result, nil
}

type clusterDomainCleanupCounts struct {
	ClientsDeleted  int
	InboundsDeleted int
	TLSDeleted      int
}

func cleanupClusterDomainResources(tx *gorm.DB, domainID uint) (clusterDomainCleanupCounts, error) {
	counts := clusterDomainCleanupCounts{}
	var clientWrappers []model.ClusterClient
	if err := tx.Where("domain_id = ?", domainID).Find(&clientWrappers).Error; err != nil {
		return counts, err
	}
	restartInboundIDs := make([]uint, 0)
	for _, wrapper := range clientWrappers {
		var client model.Client
		err := tx.Where("id = ?", wrapper.ClientID).First(&client).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return counts, err
		}
		if err == nil {
			var inboundIDs []uint
			_ = json.Unmarshal(client.Inbounds, &inboundIDs)
			restartInboundIDs = appendUniqueUintSlice(restartInboundIDs, inboundIDs...)
			if err := tx.Delete(&client).Error; err != nil {
				return counts, err
			}
			counts.ClientsDeleted++
		}
		if err := tx.Delete(&wrapper).Error; err != nil {
			return counts, err
		}
	}
	if len(restartInboundIDs) > 0 {
		if err := (&InboundService{}).RestartInbounds(tx, restartInboundIDs); err != nil {
			return counts, err
		}
	}

	var inboundWrappers []model.ClusterInbound
	if err := tx.Where("domain_id = ?", domainID).Find(&inboundWrappers).Error; err != nil {
		return counts, err
	}
	for _, wrapper := range inboundWrappers {
		tlsDeleted, inboundDeleted, err := deleteClusterManagedInbound(tx, wrapper.InboundID)
		if err != nil {
			return counts, err
		}
		if inboundDeleted {
			counts.InboundsDeleted++
		}
		if tlsDeleted {
			counts.TLSDeleted++
		}
		if err := tx.Delete(&wrapper).Error; err != nil {
			return counts, err
		}
	}
	return counts, nil
}

func deleteClusterManagedInbound(tx *gorm.DB, inboundID uint) (bool, bool, error) {
	var inbound model.Inbound
	err := tx.Where("id = ?", inboundID).First(&inbound).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	if corePtr != nil && corePtr.IsRunning() {
		if err := corePtr.RemoveInbound(inbound.Tag); err != nil && err != os.ErrInvalid {
			return false, false, err
		}
	}
	if err := (&ClientService{}).UpdateClientsOnInboundDelete(tx, inbound.Id, inbound.Tag); err != nil {
		return false, false, err
	}
	tlsID := inbound.TlsId
	if err := tx.Delete(&inbound).Error; err != nil {
		return false, false, err
	}
	tlsDeleted, err := deleteUnusedClusterTLS(tx, tlsID)
	if err != nil {
		return false, false, err
	}
	return tlsDeleted, true, nil
}

func deleteUnusedClusterTLS(tx *gorm.DB, tlsID uint) (bool, error) {
	if tlsID == 0 {
		return false, nil
	}
	var inboundCount int64
	if err := tx.Model(model.Inbound{}).Where("tls_id = ?", tlsID).Count(&inboundCount).Error; err != nil {
		return false, err
	}
	var serviceCount int64
	if err := tx.Model(model.Service{}).Where("tls_id = ?", tlsID).Count(&serviceCount).Error; err != nil {
		return false, err
	}
	if inboundCount > 0 || serviceCount > 0 {
		return false, nil
	}
	if err := tx.Delete(&model.Tls{}, tlsID).Error; err != nil {
		return false, err
	}
	return true, nil
}

func appendUniqueUintSlice(base []uint, values ...uint) []uint {
	for _, value := range values {
		base = appendUniqueUint(base, value)
	}
	return base
}
