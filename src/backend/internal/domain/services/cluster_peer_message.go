package service

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

const ClusterPeerProtocolVersion = "v1"

const (
	RouteModeDirect             = "direct"
	RouteModeMulticast          = "multicast"
	RouteModeBroadcast          = "broadcast"
	RouteModeChain              = "chain"
	RouteModeScheduledBroadcast = "scheduled_broadcast"
)

type PeerMessage struct {
	ProtocolVersion   string                 `json:"protocolVersion"`
	Domain            string                 `json:"domain"`
	MembershipVersion int64                  `json:"membershipVersion"`
	SourceNodeID      string                 `json:"sourceNodeId"`
	SourceSeq         int64                  `json:"sourceSeq"`
	Category          string                 `json:"category"`
	Action            string                 `json:"action"`
	Payload           map[string]interface{} `json:"payload"`
	PayloadHash       string                 `json:"payloadHash"`
	Route             RoutePlan              `json:"route"`
	CreatedAt         int64                  `json:"createdAt"`
	ExpiresAt         int64                  `json:"expiresAt,omitempty"`
	Signature         string                 `json:"signature,omitempty"`
}

type RoutePlan struct {
	Mode     string           `json:"mode"`
	Targets  []TargetSelector `json:"targets,omitempty"`
	Steps    []RouteStep      `json:"steps,omitempty"`
	Delivery DeliveryPolicy   `json:"delivery,omitempty"`
	Schedule SchedulePolicy   `json:"schedule,omitempty"`
}

type TargetSelector struct {
	NodeID string `json:"nodeId,omitempty"`
	Role   string `json:"role,omitempty"`
	Domain string `json:"domain,omitempty"`
}

type RouteStep struct {
	NodeID string `json:"nodeId,omitempty"`
	Action string `json:"action,omitempty"`
}

type DeliveryPolicy struct {
	RequireAck bool        `json:"requireAck,omitempty"`
	Retry      RetryPolicy `json:"retry,omitempty"`
}

type RetryPolicy struct {
	MaxAttempts int   `json:"maxAttempts,omitempty"`
	BackoffMs   int64 `json:"backoffMs,omitempty"`
}

type SchedulePolicy struct {
	NotBefore int64 `json:"notBefore,omitempty"`
	Deadline  int64 `json:"deadline,omitempty"`
}

func NewClusterPeerMessage(domain string, membershipVersion int64, sourceNodeID string, sourceSeq int64, category string, action string, payload map[string]interface{}) *PeerMessage {
	return &PeerMessage{
		ProtocolVersion:   ClusterPeerProtocolVersion,
		Domain:            domain,
		MembershipVersion: membershipVersion,
		SourceNodeID:      sourceNodeID,
		SourceSeq:         sourceSeq,
		Category:          category,
		Action:            action,
		Payload:           payload,
		CreatedAt:         time.Now().Unix(),
	}
}

func ClusterPeerPayloadHash(payload map[string]interface{}) (string, error) {
	canonical, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func SignClusterPeerMessage(local *model.ClusterLocalNode, message *PeerMessage) error {
	payloadHash, err := ClusterPeerPayloadHash(message.Payload)
	if err != nil {
		return err
	}
	privateKey, err := base64.StdEncoding.DecodeString(local.PrivateKey)
	if err != nil {
		return err
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return errors.New("invalid_private_key")
	}

	message.PayloadHash = payloadHash
	signingPayload, err := clusterPeerSigningPayload(message)
	if err != nil {
		return err
	}
	signature := ed25519.Sign(ed25519.PrivateKey(privateKey), signingPayload)
	message.Signature = base64.StdEncoding.EncodeToString(signature)
	return nil
}

func VerifyClusterPeerMessage(message *PeerMessage, publicKey string, now int64) error {
	if message.ProtocolVersion != ClusterPeerProtocolVersion {
		return errors.New("unsupported_protocol_version")
	}
	if message.ExpiresAt > 0 && now > message.ExpiresAt {
		return errors.New("message_expired")
	}

	payloadHash, err := ClusterPeerPayloadHash(message.Payload)
	if err != nil {
		return err
	}
	if payloadHash != message.PayloadHash {
		return errors.New("payload_hash_mismatch")
	}

	decodedPublicKey, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil || len(decodedPublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid_signature")
	}
	signature, err := base64.StdEncoding.DecodeString(message.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("invalid_signature")
	}
	signingPayload, err := clusterPeerSigningPayload(message)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(decodedPublicKey), signingPayload, signature) {
		return errors.New("invalid_signature")
	}
	return nil
}

func clusterPeerSigningPayload(message *PeerMessage) ([]byte, error) {
	unsigned := *message
	unsigned.Signature = ""
	return json.Marshal(unsigned)
}
