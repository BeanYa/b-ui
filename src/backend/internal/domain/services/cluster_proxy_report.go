package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	"github.com/google/uuid"
)

type ClusterProxyReportService struct {
	hubClient   clusterHubClient
	inboundSvc  *InboundService
	memberSvc   clusterMemberProvider
	identitySvc clusterIdentityProvider
	mu          sync.Mutex
}

// clusterMemberProvider abstracts getting domain member info
type clusterMemberProvider interface {
	GetAllDomains() ([]clusterDomainInfo, error)
}

type clusterDomainInfo struct {
	ID          uint
	Name        string
	MemberID    string
	BaseURL     string
	HubURL      string
	DomainToken string
}

// clusterIdentityProvider abstracts getting local node identity
type clusterIdentityProvider interface {
	GetLocalNodeID() string
}

func NewClusterProxyReportService(
	hubClient clusterHubClient,
	inboundSvc *InboundService,
	memberSvc clusterMemberProvider,
	identitySvc clusterIdentityProvider,
) *ClusterProxyReportService {
	return &ClusterProxyReportService{
		hubClient:   hubClient,
		inboundSvc:  inboundSvc,
		memberSvc:   memberSvc,
		identitySvc: identitySvc,
	}
}

// ReportForAllDomains reports proxy configs for all domains this node belongs to.
// Called after inbound CRUD operations and manual sync.
func (s *ClusterProxyReportService) ReportForAllDomains() {
	go func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		domains, err := s.memberSvc.GetAllDomains()
		if err != nil {
			log.Printf("ClusterProxyReport: failed to get domains: %v", err)
			return
		}
		for _, domain := range domains {
			if err := s.reportProxyConfigs(domain); err != nil {
				log.Printf("ClusterProxyReport: failed for domain %s: %v", domain.Name, err)
			}
		}
	}()
}

// ReportProxyConfigs reports proxy configs for a specific domain.
func (s *ClusterProxyReportService) ReportProxyConfigs(domainID uint) {
	go func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		domains, err := s.memberSvc.GetAllDomains()
		if err != nil {
			log.Printf("ClusterProxyReport: failed to get domains: %v", err)
			return
		}
		for _, domain := range domains {
			if domain.ID == domainID {
				if err := s.reportProxyConfigs(domain); err != nil {
					log.Printf("ClusterProxyReport: failed for domain %s: %v", domain.Name, err)
				}
				return
			}
		}
	}()
}

func (s *ClusterProxyReportService) reportProxyConfigs(domain clusterDomainInfo) error {
	nodeID := s.identitySvc.GetLocalNodeID()

	address := extractHostFromURL(domain.BaseURL)
	if address == "" {
		return fmt.Errorf("could not determine address for domain %s", domain.Name)
	}

	db := database.GetDB()
	var wrappers []model.ClusterInbound
	if err := db.Model(model.ClusterInbound{}).
		Preload("Inbound.Tls").
		Where("domain_id = ?", domain.ID).
		Find(&wrappers).Error; err != nil {
		return fmt.Errorf("get cluster domain inbounds: %w", err)
	}

	configs := make([]ClusterHubProxyConfigItem, 0)
	wrappedInboundIDs := make(map[uint]struct{}, len(wrappers))
	for _, wrapper := range wrappers {
		if wrapper.Inbound == nil {
			continue
		}
		wrappedInboundIDs[wrapper.InboundID] = struct{}{}
	}

	var inbounds []model.Inbound
	if err := db.Model(model.Inbound{}).Preload("Tls").Order("id").Find(&inbounds).Error; err != nil {
		return fmt.Errorf("get inbounds: %w", err)
	}
	for _, inb := range inbounds {
		if _, wrapped := wrappedInboundIDs[inb.Id]; wrapped {
			continue
		}
		configs = append(configs, buildClusterProxyReportConfig(inb, address, "node", ""))
	}

	for _, wrapper := range wrappers {
		if wrapper.Inbound == nil {
			continue
		}
		configs = append(configs, buildClusterProxyReportConfig(*wrapper.Inbound, address, "domain", wrapper.RequestID))
	}

	body := ClusterHubReportProxyConfigsRequest{
		RequestID:   uuid.New().String(),
		NodeID:      nodeID,
		MemberID:    domain.MemberID,
		DomainToken: domain.DomainToken,
		Signature:   "", // Signature generation deferred
		Configs:     configs,
	}

	return s.hubClient.ReportProxyConfigs(context.Background(), domain.HubURL, domain.Name, body)
}

func buildClusterProxyReportConfig(inb model.Inbound, address string, scope string, requestID string) ClusterHubProxyConfigItem {
	var listenPort int
	var options json.RawMessage

	if inb.Options != nil {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(inb.Options, &fields); err == nil {
			if lp, ok := fields["listen_port"]; ok {
				var p float64
				if err := json.Unmarshal(lp, &p); err == nil {
					listenPort = int(p)
				}
			}
			optMap := make(map[string]json.RawMessage)
			for k, v := range fields {
				if k != "listen" && k != "listen_port" && k != "sniff" && k != "sniff_override_destination" && k != "domain_strategy" && k != "udp_timeout" && k != "proxy_protocol" && k != "proxy_protocol_accept_no_header" {
					optMap[k] = v
				}
			}
			if len(optMap) > 0 {
				options, _ = json.Marshal(optMap)
			}
		}
	}

	return ClusterHubProxyConfigItem{
		InboundID:              inb.Id,
		Type:                   inb.Type,
		Tag:                    inb.Tag,
		ListenPort:             listenPort,
		Address:                address,
		Options:                options,
		TLSConfig:              buildClusterProxyReportTLSConfig(inb.Tls),
		Scope:                  scope,
		DomainInboundRequestID: requestID,
	}
}

func buildClusterProxyReportTLSConfig(tls *model.Tls) json.RawMessage {
	if tls == nil || len(tls.Server) == 0 {
		return nil
	}

	var server map[string]interface{}
	if err := json.Unmarshal(tls.Server, &server); err != nil {
		return tls.Server
	}

	var client map[string]interface{}
	if len(tls.Client) > 0 {
		_ = json.Unmarshal(tls.Client, &client)
	}
	if clientReality, ok := client["reality"].(map[string]interface{}); ok {
		serverReality, _ := server["reality"].(map[string]interface{})
		if serverReality == nil {
			serverReality = map[string]interface{}{"enabled": true}
			server["reality"] = serverReality
		}
		if publicKey, ok := clientReality["public_key"].(string); ok && publicKey != "" {
			serverReality["public_key"] = publicKey
		}
		if shortID, ok := clientReality["short_id"].(string); ok && shortID != "" {
			serverReality["short_id"] = shortID
		}
		delete(serverReality, "private_key")
	}

	out, err := json.Marshal(server)
	if err != nil {
		return tls.Server
	}
	return out
}

func extractHostFromURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	url := rawURL
	if len(url) > 8 && url[:8] == "https://" {
		url = url[8:]
	} else if len(url) > 7 && url[:7] == "http://" {
		url = url[7:]
	}
	for i, c := range url {
		if c == ':' || c == '/' {
			return url[:i]
		}
	}
	return url
}
