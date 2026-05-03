package ping

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type externalEndpointProbe func(context.Context, ExternalEndpoint, []string) (string, float64, error)

type ExternalService struct {
	store         *Store
	meshSvc       *MeshService
	httpClient    *http.Client
	tcpDialer     *net.Dialer
	probeEndpoint externalEndpointProbe
}

func NewExternalService(store *Store) *ExternalService {
	svc := &ExternalService{
		store:      store,
		meshSvc:    NewMeshService(),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		tcpDialer:  &net.Dialer{Timeout: 5 * time.Second},
	}
	svc.probeEndpoint = svc.probeEndpointWithMethods
	return svc
}

func currentPanelEndpoint() ExternalEndpoint {
	return ExternalEndpoint{
		ID:       "current-panel",
		Label:    "Current panel",
		Provider: "panel",
	}
}

func (s *ExternalService) probeEndpointWithMethods(ctx context.Context, endpoint ExternalEndpoint, requestedMethods []string) (string, float64, error) {
	methods := requestedMethods
	if len(methods) == 0 {
		methods = endpoint.Methods
	}
	if len(methods) == 0 {
		methods = []string{MethodTCP}
	}

	var lastErr error
	for _, method := range methods {
		if !methodAllowed(method, endpoint.Methods) {
			continue
		}
		switch method {
		case MethodTCP:
			addr := endpointAddress(endpoint)
			latency, err := measureTCPConnectLatency(s.tcpDialer, addr)
			if err == nil {
				return MethodTCP, latency, nil
			}
			lastErr = err
		case MethodICMP:
			latency, err := s.meshSvc.icmpPing(ctx, endpoint.Host)
			if err == nil {
				return MethodICMP, latency, nil
			}
			lastErr = err
		case MethodHTTP:
			scheme := "https"
			if endpoint.Port == 80 {
				scheme = "http"
			}
			latency, err := s.meshSvc.httpPing(ctx, scheme+"://"+endpoint.Host, "")
			if err == nil {
				return MethodHTTP, latency, nil
			}
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", 0, lastErr
	}
	return "", 0, fmt.Errorf("no supported methods for %s", endpoint.ID)
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

// RunInbound runs inbound tests: each enabled external source pings each cluster member.
func (s *ExternalService) RunInbound(ctx context.Context, sourceIDs []string, members []MeshMember) (*ExternalResultData, error) {
	config := s.store.LoadExternalConfigOrDefault()
	enabledSources := enabledExternalSources(sourceIDs, config, DirectionInbound)

	tasks := make([]func() ExternalTestResult, 0)

	for _, src := range enabledSources {
		targets, ok := inboundTargets[src.ID]
		if !ok {
			src := src
			tasks = append(tasks, func() ExternalTestResult {
				return unimplementedProviderResult(src, DirectionInbound)
			})
			continue
		}
		for _, tgt := range targets {
			for _, member := range members {
				tgt := tgt
				member := member
				src := src
				tasks = append(tasks, func() ExternalTestResult {
					latency, err := s.meshSvc.icmpPing(ctx, member.Address)
					r := ExternalTestResult{
						SourceMemberID: "",
						SourceLabel:    tgt.Label,
						Direction:      DirectionInbound,
						TargetMemberID: member.MemberID,
						TargetName:     member.Name,
						Source:         externalTargetEndpoint(src.ID, tgt),
						Target:         memberEndpoint(member),
					}
					if err != nil {
						r.Success = false
						r.Error = errorPtr(err.Error())
					} else {
						r.Success = true
						r.Method = methodPtr(MethodICMP)
						r.LatencyMs = latencyPtr(latency)
					}
					return r
				})
			}
		}
	}

	results := runExternalProbeTasks(DefaultExternalMaxConcurrent, tasks)
	data := &ExternalResultData{TestedAt: nowUnix(), Results: results}
	if err := s.store.SaveExternalResults(data); err != nil {
		return nil, err
	}
	return data, nil
}

// RunOutbound runs outbound tests from the current panel to external targets.
func (s *ExternalService) RunOutbound(ctx context.Context, sourceIDs []string, members []MeshMember) (*ExternalResultData, error) {
	return s.runOutboundWithMethods(ctx, sourceIDs, nil)
}

func (s *ExternalService) probeOutboundTarget(ctx context.Context, source ExternalEndpoint, target ExternalEndpoint, requestedMethods []string) ExternalTestResult {
	r := ExternalTestResult{
		SourceMemberID: source.ID,
		SourceLabel:    source.Label,
		Direction:      DirectionOutbound,
		TargetMemberID: target.ID,
		TargetName:     target.Host,
		Source:         source,
		Target:         target,
	}
	method, latency, err := s.probeEndpoint(ctx, target, requestedMethods)
	if err != nil {
		r.Success = false
		r.Error = errorPtr(err.Error())
		return r
	}
	r.Success = true
	r.Method = methodPtr(method)
	r.LatencyMs = latencyPtr(latency)
	return r
}

func (s *ExternalService) runOutboundWithMethods(ctx context.Context, sourceIDs []string, methods []string) (*ExternalResultData, error) {
	config := s.store.LoadExternalConfigOrDefault()
	enabledSources := enabledExternalSources(sourceIDs, config, DirectionOutbound)
	source := currentPanelEndpoint()
	tasks := make([]func() ExternalTestResult, 0)
	for _, src := range enabledSources {
		targets := targetsForProvider(src.ID)
		if len(targets) == 0 {
			src := src
			tasks = append(tasks, func() ExternalTestResult {
				return unimplementedProviderResult(src, DirectionOutbound)
			})
			continue
		}
		for _, target := range targets {
			target := target
			tasks = append(tasks, func() ExternalTestResult {
				return s.probeOutboundTarget(ctx, source, target, methods)
			})
		}
	}
	results := runExternalProbeTasks(DefaultExternalMaxConcurrent, tasks)
	data := &ExternalResultData{TestedAt: nowUnix(), Results: results}
	if err := s.store.SaveExternalResults(data); err != nil {
		return nil, err
	}
	return data, nil
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
			SourceMemberID: "",
			SourceLabel:    "RIPE Atlas",
			Direction:      DirectionInbound,
			TargetMemberID: member.MemberID,
			TargetName:     member.Name,
			Source:         providerEndpoint(ExternalSource{ID: "ripe_atlas", Name: "RIPE Atlas"}),
			Target:         memberEndpoint(member),
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
			if src.Direction == DirectionInbound {
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
		inData, err := s.RunInbound(ctx, enabledIn, filterExternalMembers(members, req.TargetNodeIDs))
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, inData.Results...)
	}

	if len(enabledOut) > 0 {
		outData, err := s.runOutboundWithMethods(ctx, enabledOut, req.Methods)
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

func filterExternalMembers(members []MeshMember, targetNodeIDs []string) []MeshMember {
	if len(targetNodeIDs) == 0 {
		return members
	}
	targets := make(map[string]bool, len(targetNodeIDs))
	for _, id := range targetNodeIDs {
		if id != "" {
			targets[id] = true
		}
	}
	if len(targets) == 0 {
		return members
	}
	filtered := make([]MeshMember, 0, len(members))
	for _, member := range members {
		if targets[member.NodeID] || targets[member.MemberID] {
			filtered = append(filtered, member)
		}
	}
	return filtered
}

func runExternalProbeTasks(maxConcurrent int, tasks []func() ExternalTestResult) []ExternalTestResult {
	if len(tasks) == 0 {
		return nil
	}
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	if maxConcurrent > len(tasks) {
		maxConcurrent = len(tasks)
	}

	results := make([]ExternalTestResult, len(tasks))
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		go func(index int, run func() ExternalTestResult) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[index] = run()
		}(i, task)
	}
	wg.Wait()
	return results
}

func enabledExternalSources(requestIDs []string, config *ExternalConfig, direction string) []ExternalSource {
	sources := make([]ExternalSource, 0)
	seen := make(map[string]bool, len(requestIDs))
	for _, sid := range requestIDs {
		if seen[sid] {
			continue
		}
		seen[sid] = true
		for _, src := range config.Sources {
			if src.ID == sid && src.Enabled && src.Direction == direction {
				sources = append(sources, src)
				break
			}
		}
	}
	return sources
}

func providerEndpoint(src ExternalSource) ExternalEndpoint {
	return ExternalEndpoint{
		ID:       src.ID,
		Label:    src.Name,
		Provider: src.ID,
	}
}

func memberEndpoint(member MeshMember) ExternalEndpoint {
	return ExternalEndpoint{
		ID:    member.MemberID,
		Label: member.Name,
		Host:  member.Address,
	}
}

func externalTargetEndpoint(providerID string, tgt externalTarget) ExternalEndpoint {
	return ExternalEndpoint{
		ID:       tgt.Label,
		Label:    tgt.Label,
		Provider: providerID,
		Host:     tgt.IP,
	}
}

func unimplementedProviderResult(src ExternalSource, direction string) ExternalTestResult {
	return ExternalTestResult{
		SourceLabel: src.Name,
		Direction:   direction,
		Source:      providerEndpoint(src),
		Success:     false,
		Error:       errorPtr(fmt.Sprintf("external provider %q is not implemented yet", src.ID)),
	}
}
