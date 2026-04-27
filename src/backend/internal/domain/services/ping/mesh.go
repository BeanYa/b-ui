package ping

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var ErrHubUnreachable = fmt.Errorf("hub unreachable")

type MeshMember struct {
	MemberID  string
	NodeID    string
	Name      string
	Address   string
	BaseURL   string
	PeerToken string
}

type MeshService struct {
	httpClient *http.Client
	tcpDialer  *net.Dialer
	tcpPort    int
}

func NewMeshService() *MeshService {
	return &MeshService{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		tcpDialer:  &net.Dialer{Timeout: 3 * time.Second},
		tcpPort:    DefaultTCPPort,
	}
}

func (s *MeshService) Run(ctx context.Context, domainID string, members []MeshMember, localNodeID string) (*MeshResult, error) {
	result := &MeshResult{
		DomainID: domainID,
		TestedAt: nowUnix(),
		Results:  s.runMesh(ctx, domainID, members, localNodeID),
	}
	return result, nil
}

func (s *MeshService) runMesh(ctx context.Context, domainID string, members []MeshMember, localNodeID string) []MeshPairResult {
	if len(members) <= 1 {
		return nil
	}

	results := make([]MeshPairResult, 0, len(members)*len(members))

	for _, src := range members {
		if src.MemberID != localNodeID {
			continue
		}
		for _, tgt := range members {
			if tgt.MemberID == localNodeID {
				continue
			}
			pairResult := s.probePair(ctx, src, tgt)
			results = append(results, pairResult)
		}
	}

	return results
}

func (s *MeshService) probePair(ctx context.Context, src, tgt MeshMember) MeshPairResult {
	r := MeshPairResult{
		SourceMemberID: src.MemberID,
		SourceName:     src.Name,
		TargetMemberID: tgt.MemberID,
		TargetName:     tgt.Name,
	}

	// 1. ICMP ping (preferred — lightest, no auth required)
	target := tgt.Address
	if target == "" && tgt.BaseURL != "" {
		target = extractHostFromBaseURL(tgt.BaseURL)
	}
	if target != "" {
		latency, err := s.icmpPing(ctx, target)
		if err == nil {
			r.Method = methodPtr("icmp")
			r.LatencyMs = latencyPtr(latency)
			r.Success = true
			return r
		}
	}

	// 2. TCP connect fallback
	if tgt.Address != "" {
		addr := net.JoinHostPort(tgt.Address, strconv.Itoa(s.tcpPort))
		latency, err := measureTCPConnectLatency(s.tcpDialer, addr)
		if err == nil {
			r.Method = methodPtr("tcp")
			r.LatencyMs = latencyPtr(latency)
			r.Success = true
			return r
		}
	}

	// 3. HTTP ping via cluster protocol
	if tgt.BaseURL != "" {
		latency, err := s.httpPing(ctx, tgt.BaseURL, tgt.PeerToken)
		if err == nil {
			r.Method = methodPtr("http")
			r.LatencyMs = latencyPtr(latency)
			r.Success = true
			return r
		}
	}

	// All failed
	r.Success = false
	r.Error = errorPtr("all methods failed: icmp, tcp, http")
	return r
}

func (s *MeshService) httpPing(ctx context.Context, baseURL string, peerToken string) (float64, error) {
	pingURL := strings.TrimRight(baseURL, "/") + "/_cluster/v1/ping"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
	if err != nil {
		return 0, err
	}
	if peerToken != "" {
		req.Header.Set("X-Cluster-Token", peerToken)
	}

	start := time.Now()
	resp, err := s.httpClientOrDefault().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	latency := float64(time.Since(start).Microseconds()) / 1000.0

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("ping returned %d", resp.StatusCode)
	}
	var body struct {
		Status string `json:"status"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	if body.Status != "processed" && body.Code != "ok" {
		return 0, fmt.Errorf("ping rejected: %s/%s", body.Status, body.Code)
	}
	return latency, nil
}

func (s *MeshService) icmpPing(ctx context.Context, target string) (float64, error) {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "ping", "-n", strconv.Itoa(DefaultPingCount), "-w", strconv.Itoa(DefaultPingTimeout*1000), target)
	} else {
		cmd = exec.CommandContext(ctx, "ping", "-c", strconv.Itoa(DefaultPingCount), "-W", strconv.Itoa(DefaultPingTimeout), target)
	}

	// Force C locale for consistent output parsing
	cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")

	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) == 0 {
			return 0, err
		}
	}

	result, parseErr := parsePingOutput(output, target)
	if parseErr != nil {
		if err != nil {
			return 0, err
		}
		return 0, parseErr
	}
	if result.err != nil {
		return 0, result.err
	}
	return result.latencyMs, nil
}

func (s *MeshService) httpClientOrDefault() *http.Client {
	if s.httpClient != nil {
		return s.httpClient
	}
	return &http.Client{Timeout: 5 * time.Second}
}

// Windows: Average = Xms
var winAvgPattern = regexp.MustCompile(`Average\s*[=:]\s*(\d+)\s*ms`)

// Linux: rtt min/avg/max/mdev = a/b/c/d ms
var linuxAvgPattern = regexp.MustCompile(`rtt min/avg/max/mdev\s*=\s*[\d.]+/([\d.]+)/[\d.]+/[\d.]+\s*ms`)

// macOS: round-trip min/avg/max/stddev = a/b/c/d ms
var macAvgPattern = regexp.MustCompile(`round-trip min/avg/max/stddev\s*=\s*[\d.]+/([\d.]+)/[\d.]+/[\d.]+\s*ms`)

func parsePingOutput(output []byte, target string) (pingResult, error) {
	text := string(output)

	// Check for 100% loss
	if strings.Contains(text, "100% loss") || strings.Contains(text, "100% packet loss") {
		return pingResult{addr: target, err: fmt.Errorf("100%% packet loss")}, nil
	}

	// Try Windows pattern
	if m := winAvgPattern.FindStringSubmatch(text); m != nil {
		avg, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return pingResult{err: err}, nil
		}
		return pingResult{addr: target, latencyMs: avg}, nil
	}

	// Try Linux pattern
	if m := linuxAvgPattern.FindStringSubmatch(text); m != nil {
		avg, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return pingResult{err: err}, nil
		}
		return pingResult{addr: target, latencyMs: avg}, nil
	}

	// Try macOS pattern
	if m := macAvgPattern.FindStringSubmatch(text); m != nil {
		avg, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return pingResult{err: err}, nil
		}
		return pingResult{addr: target, latencyMs: avg}, nil
	}

	return pingResult{err: fmt.Errorf("could not parse ping output")}, nil
}

func measureTCPConnectLatency(dialer *net.Dialer, addr string) (float64, error) {
	start := time.Now()
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return float64(time.Since(start).Microseconds()) / 1000.0, nil
}

func extractHostFromBaseURL(baseURL string) string {
	s := strings.TrimSpace(baseURL)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.Index(s, ":"); idx >= 0 {
		s = s[:idx]
	}
	return s
}

