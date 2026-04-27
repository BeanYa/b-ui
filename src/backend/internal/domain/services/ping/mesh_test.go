package ping

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
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
		httpClient: &http.Client{},
		tcpDialer:  &net.Dialer{},
		tcpPort:    DefaultTCPPort,
	}

	members := []MeshMember{
		{MemberID: "local-1", NodeID: "local-1", Name: "local", BaseURL: "http://localhost:9999", PeerToken: ""},
		{MemberID: "peer-1", NodeID: "peer-1", Name: "peer", BaseURL: ts.URL, PeerToken: ""},
	}

	results := svc.runMesh(context.Background(), "test.example.com", members, "local-1")

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
	results := svc.runMesh(context.Background(), "test.example.com", members, "n1")
	if len(results) != 0 {
		t.Fatalf("expected 0 results for single-node domain, got %d", len(results))
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
