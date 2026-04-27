package ping

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

type ExternalService struct {
	store      *Store
	meshSvc    *MeshService
	httpClient *http.Client
	tcpDialer  *net.Dialer
}

func NewExternalService(store *Store) *ExternalService {
	return &ExternalService{
		store:      store,
		meshSvc:    NewMeshService(),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		tcpDialer:  &net.Dialer{Timeout: 5 * time.Second},
	}
}

type externalTarget struct {
	IP    string `json:"ip"`
	Label string `json:"label"`
}

// External node definitions — sourced from public cloud/CDN provider documentation.
var inboundTargets = map[string][]externalTarget{
	"linode_lg": {
		{IP: "speedtest.newark.linode.com", Label: "Linode - Newark"},
		{IP: "speedtest.atlanta.linode.com", Label: "Linode - Atlanta"},
		{IP: "speedtest.dallas.linode.com", Label: "Linode - Dallas"},
		{IP: "speedtest.fremont.linode.com", Label: "Linode - Fremont"},
		{IP: "speedtest.frankfurt.linode.com", Label: "Linode - Frankfurt"},
		{IP: "speedtest.london.linode.com", Label: "Linode - London"},
		{IP: "speedtest.singapore.linode.com", Label: "Linode - Singapore"},
		{IP: "speedtest.tokyo2.linode.com", Label: "Linode - Tokyo"},
		{IP: "speedtest.syd1.linode.com", Label: "Linode - Sydney"},
		{IP: "speedtest.mumbai1.linode.com", Label: "Linode - Mumbai"},
		{IP: "speedtest.toronto1.linode.com", Label: "Linode - Toronto"},
	},
	"he_lg": {
		{IP: "lg.he.net", Label: "HE LG - Fremont"},
	},
	"zstatic_cdn": {
		{IP: "lf3-ips.zstaticcdn.com", Label: "Zstatic CDN"},
	},
}

var outboundTargets = map[string][]externalTarget{
	"cloud_test_ips": {
		// AWS
		{IP: "ec2.ap-northeast-1.amazonaws.com", Label: "AWS - Tokyo"},
		{IP: "ec2.ap-southeast-1.amazonaws.com", Label: "AWS - Singapore"},
		{IP: "ec2.eu-west-1.amazonaws.com", Label: "AWS - Ireland"},
		{IP: "ec2.us-east-1.amazonaws.com", Label: "AWS - N. Virginia"},
		{IP: "ec2.us-west-1.amazonaws.com", Label: "AWS - N. California"},
		// GCP
		{IP: "35.200.0.0", Label: "GCP - Tokyo"},
		{IP: "35.197.0.0", Label: "GCP - London"},
		{IP: "35.185.0.0", Label: "GCP - US Central"},
		// Azure
		{IP: "13.73.0.0", Label: "Azure - SE Asia"},
		{IP: "40.69.0.0", Label: "Azure - Japan"},
		{IP: "13.69.0.0", Label: "Azure - W. Europe"},
		// Alibaba Cloud
		{IP: "47.88.0.0", Label: "AliCloud - Singapore"},
		{IP: "47.91.0.0", Label: "AliCloud - Hong Kong"},
		{IP: "8.209.0.0", Label: "AliCloud - Frankfurt"},
	},
	"public_dns": {
		{IP: "8.8.8.8", Label: "Google DNS"},
		{IP: "1.1.1.1", Label: "Cloudflare DNS"},
		{IP: "114.114.114.114", Label: "114 DNS"},
	},
	"cdn_edges": {
		{IP: "cloudflare.com", Label: "Cloudflare Edge"},
		{IP: "www.fastly.com", Label: "Fastly Edge"},
		{IP: "www.akamai.com", Label: "Akamai Edge"},
	},
}

// RunInbound runs inbound tests: each enabled external source pings each cluster member.
func (s *ExternalService) RunInbound(ctx context.Context, sourceIDs []string, members []MeshMember) (*ExternalResultData, error) {
	config := s.store.LoadExternalConfigOrDefault()
	enabledSet := makeSourceSet(sourceIDs, config, "inbound")

	results := make([]ExternalTestResult, 0)

	for sourceID := range enabledSet {
		targets, ok := inboundTargets[sourceID]
		if !ok {
			continue
		}
		for _, tgt := range targets {
			for _, member := range members {
				latency, err := s.meshSvc.icmpPing(ctx, member.Address)
				r := ExternalTestResult{
					SourceLabel:    tgt.Label,
					Direction:      "inbound",
					TargetMemberID: member.MemberID,
					TargetName:     member.Name,
				}
				if err != nil {
					r.Success = false
					r.Error = errorPtr(err.Error())
				} else {
					r.Success = true
					r.Method = methodPtr("icmp")
					r.LatencyMs = latencyPtr(latency)
				}
				results = append(results, r)
			}
		}
	}

	data := &ExternalResultData{TestedAt: nowUnix(), Results: results}
	if err := s.store.SaveExternalResults(data); err != nil {
		return nil, err
	}
	return data, nil
}

// RunOutbound runs outbound tests: each cluster member pings external targets.
func (s *ExternalService) RunOutbound(ctx context.Context, sourceIDs []string, members []MeshMember) (*ExternalResultData, error) {
	config := s.store.LoadExternalConfigOrDefault()
	enabledSet := makeSourceSet(sourceIDs, config, "outbound")

	results := make([]ExternalTestResult, 0)

	for sourceID := range enabledSet {
		targets, ok := outboundTargets[sourceID]
		if !ok {
			continue
		}
		for _, member := range members {
			for _, tgt := range targets {
				r := probeExternalTarget(ctx, s.meshSvc, s.tcpDialer, sourceID, member, tgt)
				results = append(results, r)
			}
		}
	}

	data := &ExternalResultData{TestedAt: nowUnix(), Results: results}
	if err := s.store.SaveExternalResults(data); err != nil {
		return nil, err
	}
	return data, nil
}

func probeExternalTarget(ctx context.Context, meshSvc *MeshService, dialer *net.Dialer, sourceID string, member MeshMember, tgt externalTarget) ExternalTestResult {
	r := ExternalTestResult{
		SourceLabel:    member.Name,
		Direction:      "outbound",
		TargetMemberID: tgt.Label,
		TargetName:     tgt.IP,
	}

	// For ICMP-only sources, skip TCP/HTTP
	switch sourceID {
	case "public_dns", "cloud_test_ips":
		latency, err := meshSvc.icmpPing(ctx, tgt.IP)
		if err == nil {
			r.Success = true
			r.Method = methodPtr("icmp")
			r.LatencyMs = latencyPtr(latency)
		} else {
			r.Success = false
			r.Error = errorPtr(err.Error())
		}
		return r

	case "cdn_edges":
		// HTTP first (CDN edge), then ICMP fallback
		latency, err := meshSvc.httpPing(ctx, "https://"+tgt.IP, "")
		if err == nil {
			r.Success = true
			r.Method = methodPtr("http")
			r.LatencyMs = latencyPtr(latency)
			return r
		}
		latency, err = meshSvc.icmpPing(ctx, tgt.IP)
		if err == nil {
			r.Success = true
			r.Method = methodPtr("icmp")
			r.LatencyMs = latencyPtr(latency)
			return r
		}
		r.Success = false
		r.Error = errorPtr(err.Error())
		return r
	}

	// Default: ICMP
	latency, err := meshSvc.icmpPing(ctx, tgt.IP)
	if err == nil {
		r.Success = true
		r.Method = methodPtr("icmp")
		r.LatencyMs = latencyPtr(latency)
	} else {
		r.Success = false
		r.Error = errorPtr(err.Error())
	}
	return r
}

// RunRIPEAtlas runs an inbound test using RIPE Atlas API.
func (s *ExternalService) RunRIPEAtlas(ctx context.Context, apiKey string, members []MeshMember) (*ExternalResultData, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("RIPE Atlas API key is required")
	}

	results := make([]ExternalTestResult, 0)
	for _, member := range members {
		if member.Address == "" {
			continue
		}
		r := ExternalTestResult{
			SourceLabel:    "RIPE Atlas",
			Direction:      "inbound",
			TargetMemberID: member.MemberID,
			TargetName:     member.Name,
			Success:        false,
			Error:          errorPtr("RIPE Atlas integration not implemented — requires measurement lifecycle management"),
		}
		results = append(results, r)
	}

	data := &ExternalResultData{TestedAt: nowUnix(), Results: results}
	return data, nil
}

// Run attempts to run all enabled sources in the request.
func (s *ExternalService) Run(ctx context.Context, req ExternalRunRequest, members []MeshMember) (*ExternalResultData, error) {
	config := s.store.LoadExternalConfigOrDefault()
	enabledIn := make([]string, 0)
	enabledOut := make([]string, 0)

	for _, sid := range req.SourceIDs {
		for _, src := range config.Sources {
			if src.ID != sid || !src.Enabled {
				continue
			}
			if src.Direction == "inbound" {
				enabledIn = append(enabledIn, sid)
			} else {
				enabledOut = append(enabledOut, sid)
			}
		}
	}

	if len(enabledIn) == 0 && len(enabledOut) == 0 {
		return nil, fmt.Errorf("no enabled sources in request")
	}

	var allResults []ExternalTestResult

	if len(enabledIn) > 0 {
		inData, err := s.RunInbound(ctx, enabledIn, members)
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, inData.Results...)
	}

	if len(enabledOut) > 0 {
		outData, err := s.RunOutbound(ctx, enabledOut, members)
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, outData.Results...)
	}

	data := &ExternalResultData{TestedAt: nowUnix(), Results: allResults}
	if err := s.store.SaveExternalResults(data); err != nil {
		return nil, err
	}
	return data, nil
}

func makeSourceSet(requestIDs []string, config *ExternalConfig, direction string) map[string]bool {
	set := make(map[string]bool)
	for _, sid := range requestIDs {
		for _, src := range config.Sources {
			if src.ID == sid && src.Enabled && src.Direction == direction {
				set[sid] = true
				break
			}
		}
	}
	return set
}
