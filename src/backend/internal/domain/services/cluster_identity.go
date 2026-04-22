package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"

	database "github.com/alireza0/s-ui/src/backend/internal/infra/db"
	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

var errClusterLocalNodeNotFound = errors.New("cluster local node not found")

func EncryptClusterDomainToken(secret []byte, token string) (string, error) {
	block, err := aes.NewCipher(clusterSecretKey(secret))
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(token), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func DecryptClusterDomainToken(secret []byte, encrypted string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(clusterSecretKey(secret))
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < aead.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce := raw[:aead.NonceSize()]
	plaintext, err := aead.Open(nil, nonce, raw[aead.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

type clusterLocalNodeStore interface {
	First() (*model.ClusterLocalNode, error)
	Create(*model.ClusterLocalNode) error
}

type dbClusterLocalNodeStore struct{}

func (s *dbClusterLocalNodeStore) First() (*model.ClusterLocalNode, error) {
	local := &model.ClusterLocalNode{}
	err := database.GetDB().First(local).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errClusterLocalNodeNotFound
	}
	if err != nil {
		return nil, err
	}
	return local, nil
}

func (s *dbClusterLocalNodeStore) Create(local *model.ClusterLocalNode) error {
	return database.GetDB().Create(local).Error
}

type ClusterLocalIdentityService struct {
	store clusterLocalNodeStore
}

func (s *ClusterLocalIdentityService) GetOrCreate() (*model.ClusterLocalNode, error) {
	store := s.store
	if store == nil {
		store = &dbClusterLocalNodeStore{}
	}
	local, err := store.First()
	if err == nil {
		return local, nil
	}
	if !errors.Is(err, errClusterLocalNodeNotFound) {
		return nil, err
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	nodeID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	local = &model.ClusterLocalNode{
		NodeID:     nodeID.String(),
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}
	if err := store.Create(local); err != nil {
		return nil, err
	}
	return local, nil
}

func clusterSecretKey(secret []byte) []byte {
	sum := sha256.Sum256(secret)
	return sum[:]
}
