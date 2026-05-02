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
	var inbounds []model.Inbound
	if err := db.Model(model.Inbound{}).Preload("Tls").Find(&inbounds).Error; err != nil {
		return fmt.Errorf("get all inbounds: %w", err)
	}

	var configs []ClusterHubProxyConfigItem
	for _, inb := range inbounds {
		var listenPort int
		var options json.RawMessage
		var tlsConfig json.RawMessage

		if inb.Options != nil {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(inb.Options, &fields); err == nil {
				if lp, ok := fields["listen_port"]; ok {
					var p float64
					if err := json.Unmarshal(lp, &p); err == nil {
						listenPort = int(p)
					}
				}
				// Build options from protocol-specific fields (exclude listen and listen_port)
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

		if inb.Tls != nil {
			tlsConfig = inb.Tls.Server
		}

		configs = append(configs, ClusterHubProxyConfigItem{
			InboundID:  inb.Id,
			Type:       inb.Type,
			Tag:        inb.Tag,
			ListenPort: listenPort,
			Address:    address,
			Options:    options,
			TLSConfig:  tlsConfig,
		})
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
