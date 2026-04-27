package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func TestClusterDomainTokenCodecRoundTrip(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	token := "domain-token-value"

	encrypted, err := EncryptClusterDomainToken(secret, token)
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}
	if encrypted == token {
		t.Fatal("expected encrypted token to differ from plaintext")
	}

	decrypted, err := DecryptClusterDomainToken(secret, encrypted)
	if err != nil {
		t.Fatalf("decrypt token: %v", err)
	}
	if decrypted != token {
		t.Fatalf("expected decrypted token %q, got %q", token, decrypted)
	}
}

func TestClusterDomainTokenCodecRejectsTampering(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")

	encrypted, err := EncryptClusterDomainToken(secret, "domain-token-value")
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}

	tampered := encrypted[:len(encrypted)-1] + "A"
	if _, err := DecryptClusterDomainToken(secret, tampered); err == nil {
		t.Fatal("expected tampered ciphertext to be rejected")
	}
}

func TestClusterLocalIdentityStoreCreatesAndReusesKeypair(t *testing.T) {
	repo := &stubClusterLocalNodeStore{}
	service := &ClusterLocalIdentityService{store: repo}
	first, err := service.GetOrCreate()
	if err != nil {
		t.Fatalf("create local identity: %v", err)
	}
	second, err := service.GetOrCreate()
	if err != nil {
		t.Fatalf("reload local identity: %v", err)
	}

	if first.Id == 0 {
		t.Fatal("expected persisted local node id")
	}
	if first.NodeID == "" {
		t.Fatal("expected node identifier")
	}
	if first.NodeID != second.NodeID {
		t.Fatalf("expected stable node id, got %q then %q", first.NodeID, second.NodeID)
	}
	if first.PublicKey != second.PublicKey {
		t.Fatal("expected stored public key to be reused")
	}
	if first.PrivateKey != second.PrivateKey {
		t.Fatal("expected stored private key to be reused")
	}

	publicKey, err := base64.StdEncoding.DecodeString(first.PublicKey)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	if len(publicKey) != ed25519.PublicKeySize {
		t.Fatalf("expected ed25519 public key size %d, got %d", ed25519.PublicKeySize, len(publicKey))
	}

	privateKey, err := base64.StdEncoding.DecodeString(first.PrivateKey)
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		t.Fatalf("expected ed25519 private key size %d, got %d", ed25519.PrivateKeySize, len(privateKey))
	}
	if got := ed25519.PrivateKey(privateKey).Public().(ed25519.PublicKey); string(got) != string(publicKey) {
		t.Fatal("expected stored public key to match private key")
	}
}

type stubClusterLocalNodeStore struct {
	node   *model.ClusterLocalNode
	nextID uint
}

func (s *stubClusterLocalNodeStore) First() (*model.ClusterLocalNode, error) {
	if s.node == nil {
		return nil, errClusterLocalNodeNotFound
	}
	copy := *s.node
	return &copy, nil
}

func (s *stubClusterLocalNodeStore) Create(node *model.ClusterLocalNode) error {
	s.nextID++
	copy := *node
	copy.Id = s.nextID
	s.node = &copy
	node.Id = copy.Id
	return nil
}
