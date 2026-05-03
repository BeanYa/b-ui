package ping

import (
	"net"
	"strconv"
	"strings"
)

const (
	DirectionInbound  = "inbound"
	DirectionOutbound = "outbound"
	MethodICMP        = "icmp"
	MethodTCP         = "tcp"
	MethodHTTP        = "http"
)

func defaultExternalSources() []ExternalSource {
	return []ExternalSource{
		{ID: "check_host", Name: "Check-Host", Type: "public_probe", Direction: DirectionInbound, Enabled: true},
		{ID: "globalping", Name: "Globalping", Type: "public_probe", Direction: DirectionInbound, Enabled: false},
		{ID: "ripe_atlas", Name: "RIPE Atlas", Type: "rest_api", Direction: DirectionInbound, Enabled: false},
		{ID: "cloudflare_workers", Name: "Cloudflare Workers", Type: "self_hosted", Direction: DirectionInbound, Enabled: false},
		{ID: "zstatic_cdn", Name: "ZStaticCDN", Type: "target_catalog", Direction: DirectionOutbound, Enabled: true},
		{ID: "linode_speedtest", Name: "Linode/Akamai Speed Test", Type: "target_catalog", Direction: DirectionOutbound, Enabled: true},
		{ID: "public_dns", Name: "Public DNS", Type: "target_catalog", Direction: DirectionOutbound, Enabled: true},
		{ID: "cdn_edges", Name: "CDN Edge Nodes", Type: "target_catalog", Direction: DirectionOutbound, Enabled: true},
		{ID: "cloud_test_ips", Name: "Cloud Provider Test IPs", Type: "target_catalog", Direction: DirectionOutbound, Enabled: true},
	}
}

func normalizeExternalConfig(config *ExternalConfig) *ExternalConfig {
	defaults := defaultExternalSources()
	if config == nil {
		return &ExternalConfig{Sources: defaults}
	}
	legacy := make(map[string]ExternalSource, len(config.Sources))
	for _, src := range config.Sources {
		legacy[src.ID] = src
	}
	merged := make([]ExternalSource, 0, len(defaults))
	for _, def := range defaults {
		if old, ok := legacy[def.ID]; ok {
			def.Enabled = old.Enabled
			def.APIKey = old.APIKey
			def.WorkerURL = old.WorkerURL
		}
		merged = append(merged, def)
	}
	return &ExternalConfig{Sources: merged}
}

func endpointAddress(endpoint ExternalEndpoint) string {
	host := strings.TrimSpace(endpoint.Host)
	if host == "" || endpoint.Port <= 0 {
		return host
	}
	return net.JoinHostPort(host, strconv.Itoa(endpoint.Port))
}

func methodAllowed(method string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, item := range allowed {
		if item == method {
			return true
		}
	}
	return false
}
