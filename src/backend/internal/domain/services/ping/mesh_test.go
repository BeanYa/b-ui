package ping

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestMeshServiceHTTPPing(t *testing.T) {
	// Start a mock peer server that responds to /_cluster/v1/ping
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_cluster/v1/ping" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"processed","code":"ok","nodeId":"peer-1"}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer ts.Close()

	svc := &MeshService{
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					var dialer net.Dialer
					return dialer.DialContext(ctx, network, ts.Listener.Addr().String())
				},
			},
		},
		tcpDialer: &net.Dialer{},
		tcpPort:   DefaultTCPPort,
		icmpPinger: func(context.Context, string) (float64, error) {
			return 0, errors.New("icmp disabled for http fallback test")
		},
	}

	members := []MeshMember{
		{MemberID: "local-1", NodeID: "local-1", Name: "local", BaseURL: "http://localhost:9999", PeerToken: ""},
		{MemberID: "peer-1", NodeID: "peer-1", Name: "peer", BaseURL: "http://mesh-http.invalid", PeerToken: ""},
	}

	results := svc.runMesh(context.Background(), "test.example.com", members, "local-1", 1)

	if len(results) != 1 {
		t.Fatalf("expected 1 pair (local->peer), got %d", len(results))
	}
	// local->peer should succeed via HTTP
	for _, r := range results {
		if r.SourceMemberID == "local-1" && r.TargetMemberID == "peer-1" {
			if !r.Success {
				t.Fatalf("expected success for local->peer, got error: %v", r.Error)
			}
			if *r.Method != "http" {
				t.Fatalf("expected http method, got %v", *r.Method)
			}
		}
	}
}

func TestMeshServiceSingleNode(t *testing.T) {
	svc := &MeshService{httpClient: &http.Client{}}
	members := []MeshMember{
		{MemberID: "n1", NodeID: "n1", Name: "only", BaseURL: "http://localhost:9999", PeerToken: ""},
	}
	results := svc.runMesh(context.Background(), "test.example.com", members, "n1", 1)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for single-node domain, got %d", len(results))
	}
}

func TestMeshServiceRunWithProgressEmitsEachPairResult(t *testing.T) {
	svc := &MeshService{
		icmpPinger: func(ctx context.Context, target string) (float64, error) {
			switch target {
			case "10.0.0.2":
				return 21, nil
			case "10.0.0.3":
				return 34, nil
			default:
				return 0, errors.New("unexpected target")
			}
		},
	}
	members := []MeshMember{
		{MemberID: "local-1", NodeID: "local-1", Name: "local", Address: "10.0.0.1"},
		{MemberID: "peer-1", NodeID: "peer-1", Name: "peer one", Address: "10.0.0.2"},
		{MemberID: "peer-2", NodeID: "peer-2", Name: "peer two", Address: "10.0.0.3"},
	}
	var progress []MeshPairResult

	result, err := svc.RunWithProgress(context.Background(), "test.example.com", members, "local-1", func(r MeshPairResult) {
		progress = append(progress, r)
	}, 1)
	if err != nil {
		t.Fatalf("RunWithProgress: %v", err)
	}

	if len(progress) != 2 {
		t.Fatalf("expected 2 progress callbacks, got %d", len(progress))
	}
	if len(result.Results) != len(progress) {
		t.Fatalf("expected final results to match progress callbacks, got %d and %d", len(result.Results), len(progress))
	}
	for i := range progress {
		if progress[i].TargetMemberID != result.Results[i].TargetMemberID {
			t.Fatalf("progress result %d target = %s, final target = %s", i, progress[i].TargetMemberID, result.Results[i].TargetMemberID)
		}
	}
}

func TestParsePingOutput(t *testing.T) {
	// Windows ping output
	winOut := []byte(`
Pinging 1.1.1.1 with 32 bytes of data:
Reply from 1.1.1.1: bytes=32 time=5ms TTL=58
Reply from 1.1.1.1: bytes=32 time=6ms TTL=58
Reply from 1.1.1.1: bytes=32 time=4ms TTL=58
Reply from 1.1.1.1: bytes=32 time=5ms TTL=58
Reply from 1.1.1.1: bytes=32 time=7ms TTL=58

Ping statistics for 1.1.1.1:
    Packets: Sent = 5, Received = 5, Lost = 0 (0% loss),
Approximate round trip times in milli-seconds:
    Minimum = 4ms, Maximum = 7ms, Average = 5ms
`)
	r, err := parsePingOutput(winOut, "1.1.1.1")
	if err != nil {
		t.Fatalf("parsePingOutput: %v", err)
	}
	if r.latencyMs != 5.0 {
		t.Fatalf("expected 5ms avg, got %f", r.latencyMs)
	}

	// Linux ping output
	linuxOut := []byte(`PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.
64 bytes from 8.8.8.8: icmp_seq=1 ttl=117 time=12.3 ms
64 bytes from 8.8.8.8: icmp_seq=2 ttl=117 time=11.8 ms

--- 8.8.8.8 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1001ms
rtt min/avg/max/mdev = 11.800/12.050/12.300/0.250 ms
`)
	r2, err := parsePingOutput(linuxOut, "8.8.8.8")
	if err != nil {
		t.Fatalf("parsePingOutput linux: %v", err)
	}
	if r2.latencyMs != 12.05 {
		t.Fatalf("expected 12.05ms avg, got %f", r2.latencyMs)
	}
}

func TestTCPConnectLatency(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, _ := ln.Accept()
			if conn != nil {
				conn.Close()
			}
		}
	}()

	lat, err := measureTCPConnectLatency(&net.Dialer{Timeout: 2 * time.Second}, ln.Addr().String())
	if err != nil {
		t.Fatalf("expected TCP success, got: %v", err)
	}
	if lat < 0 {
		t.Fatalf("expected non-negative latency, got %f", lat)
	}
}

func TestMeshServiceConcurrentProbing(t *testing.T) {
	var mu sync.Mutex
	var started []string

	svc := &MeshService{
		icmpPinger: func(ctx context.Context, target string) (float64, error) {
			mu.Lock()
			started = append(started, target)
			mu.Unlock()
			// simulate work so goroutines overlap
			time.Sleep(50 * time.Millisecond)
			switch target {
			case "10.0.0.2":
				return 10, nil
			case "10.0.0.3":
				return 20, nil
			case "10.0.0.4":
				return 30, nil
			default:
				return 0, errors.New("unexpected target")
			}
		},
	}
	members := []MeshMember{
		{MemberID: "local-1", NodeID: "local-1", Name: "local", Address: "10.0.0.1"},
		{MemberID: "peer-1", NodeID: "peer-1", Name: "peer one", Address: "10.0.0.2"},
		{MemberID: "peer-2", NodeID: "peer-2", Name: "peer two", Address: "10.0.0.3"},
		{MemberID: "peer-3", NodeID: "peer-3", Name: "peer three", Address: "10.0.0.4"},
	}

	start := time.Now()
	results := svc.runMeshWithProgress(context.Background(), "test.example.com", members, "local-1", nil, 3)
	elapsed := time.Since(start)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Success {
			t.Errorf("expected success for %s->%s, got error: %v", r.SourceName, r.TargetName, r.Error)
		}
	}

	// With 3 targets and maxConcurrent=3, all probes run in parallel
	// so total time should be ~50ms, not ~150ms
	if elapsed > 200*time.Millisecond {
		t.Fatalf("concurrent probes took too long (%v), likely not running in parallel", elapsed)
	}

	// Verify all targets were probed
	mu.Lock()
	defer mu.Unlock()
	if len(started) != 3 {
		t.Fatalf("expected 3 probes started, got %d", len(started))
	}
}

func TestMeshServiceConcurrencySemaphore(t *testing.T) {
	var mu sync.Mutex
	activeCount := 0
	maxActive := 0

	svc := &MeshService{
		icmpPinger: func(ctx context.Context, target string) (float64, error) {
			mu.Lock()
			activeCount++
			if activeCount > maxActive {
				maxActive = activeCount
			}
			mu.Unlock()

			time.Sleep(50 * time.Millisecond)

			mu.Lock()
			activeCount--
			mu.Unlock()
			return 10, nil
		},
	}
	members := []MeshMember{
		{MemberID: "local-1", NodeID: "local-1", Name: "local", Address: "10.0.0.1"},
		{MemberID: "peer-1", NodeID: "peer-1", Name: "peer one", Address: "10.0.0.2"},
		{MemberID: "peer-2", NodeID: "peer-2", Name: "peer two", Address: "10.0.0.3"},
		{MemberID: "peer-3", NodeID: "peer-3", Name: "peer three", Address: "10.0.0.4"},
		{MemberID: "peer-4", NodeID: "peer-4", Name: "peer four", Address: "10.0.0.5"},
	}

	results := svc.runMeshWithProgress(context.Background(), "test.example.com", members, "local-1", nil, 2)

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	mu.Lock()
	defer mu.Unlock()
	if maxActive > 2 {
		t.Fatalf("max concurrent probes was %d, expected at most 2", maxActive)
	}
}
