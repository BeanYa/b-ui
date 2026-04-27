package ping

import "time"

const (
	DataDir           = "data/ping"
	MeshSubDir        = "mesh"
	ExternalSubDir    = "external"
	ConfigFile        = "config.json"
	ResultsFile       = "results.json"
	DefaultTCPPort    = 80
	DefaultPingCount  = 5
	DefaultPingTimeout = 2 // seconds
)

type MeshResult struct {
	DomainID  string           `json:"domain_id"`
	TestedAt  int64            `json:"tested_at"`
	Results   []MeshPairResult `json:"results"`
}

type MeshPairResult struct {
	SourceMemberID string  `json:"source_member_id"`
	SourceName     string  `json:"source_name"`
	TargetMemberID string  `json:"target_member_id"`
	TargetName     string  `json:"target_name"`
	Method         *string `json:"method"`
	LatencyMs      *float64 `json:"latency_ms"`
	Success        bool    `json:"success"`
	Error          *string `json:"error"`
}

type ExternalConfig struct {
	Sources []ExternalSource `json:"sources"`
}

type ExternalSource struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Direction string `json:"direction"`
	Enabled   bool   `json:"enabled"`
	APIKey    string `json:"api_key"`
	WorkerURL string `json:"worker_url,omitempty"`
}

type ExternalResultData struct {
	TestedAt int64                `json:"tested_at"`
	Results  []ExternalTestResult `json:"results"`
}

type ExternalTestResult struct {
	SourceLabel    string  `json:"source_label"`
	Direction      string  `json:"direction"`
	TargetMemberID string  `json:"target_member_id"`
	TargetName     string  `json:"target_name"`
	Method         *string `json:"method"`
	LatencyMs      *float64 `json:"latency_ms"`
	Success        bool    `json:"success"`
	Error          *string  `json:"error"`
}

type ExternalRunRequest struct {
	SourceIDs []string `json:"source_ids"`
}

// TCP ping result from a single dial attempt.
type tcpResult struct {
	addr      string
	latencyMs float64
	err       error
}

// ICMP ping result parsed from system ping output.
type icmpResult struct {
	addr      string
	latencyMs float64
	err       error
}

func methodPtr(m string) *string { return &m }
func latencyPtr(v float64) *float64 { return &v }
func errorPtr(e string) *string { return &e }

func ptrToLatency(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func nowUnix() int64 { return time.Now().Unix() }
