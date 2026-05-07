package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func TestPeerMessagePayloadHashIsStable(t *testing.T) {
	payloadA := map[string]any{"version": float64(7), "domain": "edge.example.com"}
	payloadB := map[string]any{"domain": "edge.example.com", "version": float64(7)}

	hashA, err := ClusterPeerPayloadHash(payloadA)
	if err != nil {
		t.Fatalf("hash A: %v", err)
	}
	hashB, err := ClusterPeerPayloadHash(payloadB)
	if err != nil {
		t.Fatalf("hash B: %v", err)
	}
	if hashA != hashB {
		t.Fatalf("expected stable payload hash, got %q and %q", hashA, hashB)
	}
}

func TestSignAndVerifyPeerMessage(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	local := &model.ClusterLocalNode{
		NodeID:     "node-a",
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}
	message, err := NewClusterPeerMessage("edge.example.com", 5, "node-a", 1, "event", "domain.cluster.changed", map[string]any{"version": float64(5)})
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	message.Route = RoutePlan{Mode: RouteModeBroadcast}

	if err := SignClusterPeerMessage(local, message); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := VerifyClusterPeerMessage(message, local.PublicKey, time.Now().Unix()); err != nil {
		t.Fatalf("verify: %v", err)
	}
	message.Payload["version"] = float64(9)
	if err := VerifyClusterPeerMessage(message, local.PublicKey, time.Now().Unix()); err == nil {
		t.Fatal("expected tampered payload to fail verification")
	}
}

func TestVerifyPeerMessageSurvivesJSONRoundTripWithTypedPayloadValues(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	local := &model.ClusterLocalNode{
		NodeID:     "node-a",
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}
	message, err := NewClusterPeerMessage("edge.example.com", 7, "node-a", 7, PeerCategoryCommand, PeerActionDomainInboundCreate, map[string]any{
		"operation_id":        "domain-inbound-op-1",
		"request_id":          "domain-inbound-op-1",
		"domain_id":           "edge.example.com",
		"resource_kind":       ClusterDomainResourceInbound,
		"resource_id":         "group-1",
		"revision":            int64(7),
		"coordinator_node_id": "node-a",
		"target_members": []struct {
			MemberID        string `json:"member_id"`
			NodeID          string `json:"node_id"`
			DisplayName     string `json:"display_name"`
			RemoteInboundID uint   `json:"remote_inbound_id,omitempty"`
		}{{
			MemberID:    "member-b",
			NodeID:      "node-b",
			DisplayName: "Node B",
		}},
		"payload": map[string]any{
			"request_id": "domain-inbound-op-1",
			"domain_id":  "edge.example.com",
			"group_id":   "group-1",
			"inbound": map[string]any{
				"type": "vless",
				"tag":  "main",
				"port": float64(443),
			},
		},
	})
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	message.Route = RoutePlan{Mode: RouteModeDirect, Targets: []string{"node-b"}}
	if err := SignClusterPeerMessage(local, message); err != nil {
		t.Fatalf("sign: %v", err)
	}

	body, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}
	var received PeerMessage
	if err := json.Unmarshal(body, &received); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}
	if err := VerifyClusterPeerMessage(&received, local.PublicKey, time.Now().Unix()); err != nil {
		t.Fatalf("verify after JSON round trip: %v", err)
	}
}

func TestVerifyPeerMessageRejectsExpiredMessage(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	local := &model.ClusterLocalNode{
		NodeID:     "node-a",
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}
	message, err := NewClusterPeerMessage("edge.example.com", 5, "node-a", 1, "event", "domain.cluster.changed", map[string]any{"version": float64(5)})
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	message.ExpiresAt = 100
	if err := SignClusterPeerMessage(local, message); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := VerifyClusterPeerMessage(message, local.PublicKey, 101); err == nil || err.Error() != "message_expired" {
		t.Fatalf("expected message_expired, got %v", err)
	}
}

func TestVerifyPeerMessageRejectsUnsupportedSchemaVersion(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	local := &model.ClusterLocalNode{
		NodeID:     "node-a",
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}
	message, err := NewClusterPeerMessage("edge.example.com", 5, "node-a", 1, "event", "domain.cluster.changed", map[string]any{"version": float64(5)})
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	if err := SignClusterPeerMessage(local, message); err != nil {
		t.Fatalf("sign: %v", err)
	}
	message.SchemaVersion = 2
	if err := VerifyClusterPeerMessage(message, local.PublicKey, time.Now().Unix()); err == nil || err.Error() != "unsupported_schema_version" {
		t.Fatalf("expected unsupported_schema_version, got %v", err)
	}
}

func TestPeerMessageSigningOmitsUnsetRoutePolicies(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	local := &model.ClusterLocalNode{
		NodeID:     "node-a",
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}
	message, err := NewClusterPeerMessage("edge.example.com", 5, "node-a", 1, "event", "domain.cluster.changed", map[string]any{"version": float64(5)})
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	message.Route = RoutePlan{Mode: RouteModeBroadcast}
	if err := SignClusterPeerMessage(local, message); err != nil {
		t.Fatalf("sign: %v", err)
	}

	signingPayload, err := clusterPeerSigningPayload(message)
	if err != nil {
		t.Fatalf("signing payload: %v", err)
	}
	payload := string(signingPayload)
	for _, field := range []string{`"delivery"`, `"retry"`, `"schedule"`} {
		if strings.Contains(payload, field) {
			t.Fatalf("expected signing payload to omit %s when unset: %s", field, payload)
		}
	}
}

func TestDeliveryPolicyAckSerializesAsExtensibleString(t *testing.T) {
	body, err := json.Marshal(DeliveryPolicy{Ack: DeliveryAckNode})
	if err != nil {
		t.Fatalf("marshal delivery policy: %v", err)
	}
	if string(body) != `{"ack":"node"}` {
		t.Fatalf("expected string ack JSON, got %s", body)
	}
}
