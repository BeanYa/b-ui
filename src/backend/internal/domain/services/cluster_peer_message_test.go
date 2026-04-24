package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
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
	message := NewClusterPeerMessage("edge.example.com", 5, "node-a", 1, "event", "domain.cluster.changed", map[string]any{"version": float64(5)})
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
	message := NewClusterPeerMessage("edge.example.com", 5, "node-a", 1, "event", "domain.cluster.changed", map[string]any{"version": float64(5)})
	message.ExpiresAt = 100
	if err := SignClusterPeerMessage(local, message); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := VerifyClusterPeerMessage(message, local.PublicKey, 101); err == nil || err.Error() != "message_expired" {
		t.Fatalf("expected message_expired, got %v", err)
	}
}
