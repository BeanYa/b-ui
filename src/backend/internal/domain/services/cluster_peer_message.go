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
	"github.com/gofrs/uuid/v5"
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
	MessageID         string                 `json:"messageId"`
	WorkflowID        string                 `json:"workflowId,omitempty"`
	StepID            string                 `json:"stepId,omitempty"`
	DomainID          string                 `json:"domainId"`
	MembershipVersion int64                  `json:"membershipVersion"`
	SourceNodeID      string                 `json:"sourceNodeId"`
	SourceSeq         int64                  `json:"sourceSeq"`
	Category          string                 `json:"category"`
	Action            string                 `json:"action"`
	ProtocolVersion   string                 `json:"protocolVersion"`
	SchemaVersion     int                    `json:"schemaVersion"`
	Route             RoutePlan              `json:"route"`
	IdempotencyKey    string                 `json:"idempotencyKey,omitempty"`
	CausationID       string                 `json:"causationId,omitempty"`
	CorrelationID     string                 `json:"correlationId,omitempty"`
	CreatedAt         int64                  `json:"createdAt"`
	ExpiresAt         int64                  `json:"expiresAt,omitempty"`
	PayloadHash       string                 `json:"payloadHash"`
	Payload           map[string]interface{} `json:"payload"`
	Signature         string                 `json:"signature"`
}

type RoutePlan struct {
	Mode     string          `json:"mode"`
	Targets  []string        `json:"targets,omitempty"`
	Selector *TargetSelector `json:"selector,omitempty"`
	Chain    []RouteStep     `json:"chain,omitempty"`
	Delivery *DeliveryPolicy `json:"delivery,omitempty"`
	Schedule *SchedulePolicy `json:"schedule,omitempty"`
}

type TargetSelector struct {
	Include            []string `json:"include,omitempty"`
	Exclude            []string `json:"exclude,omitempty"`
	CapabilityRequired []string `json:"capabilityRequired,omitempty"`
}

type RouteStep struct {
	StepID            string                 `json:"stepId"`
	NodeID            string                 `json:"nodeId"`
	Action            string                 `json:"action,omitempty"`
	PayloadOverride   map[string]interface{} `json:"payloadOverride,omitempty"`
	ContinueOnFailure bool                   `json:"continueOnFailure,omitempty"`
}

type DeliveryPolicy struct {
	Ack       bool         `json:"ack,omitempty"`
	TimeoutMs int64        `json:"timeoutMs,omitempty"`
	Retry     *RetryPolicy `json:"retry,omitempty"`
	MaxHops   int          `json:"maxHops,omitempty"`
}

type RetryPolicy struct {
	MaxAttempts int   `json:"maxAttempts,omitempty"`
	BackoffMs   int64 `json:"backoffMs,omitempty"`
}

type SchedulePolicy struct {
	Kind       string `json:"kind,omitempty"`
	RunAt      int64  `json:"runAt,omitempty"`
	IntervalMs int64  `json:"intervalMs,omitempty"`
	Cron       string `json:"cron,omitempty"`
	MaxRuns    int    `json:"maxRuns,omitempty"`
	ExpiresAt  int64  `json:"expiresAt,omitempty"`
}

func NewClusterPeerMessage(domain string, membershipVersion int64, sourceNodeID string, sourceSeq int64, category string, action string, payload map[string]interface{}) (*PeerMessage, error) {
	messageID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	return &PeerMessage{
		MessageID:         messageID.String(),
		DomainID:          domain,
		ProtocolVersion:   ClusterPeerProtocolVersion,
		SchemaVersion:     1,
		MembershipVersion: membershipVersion,
		SourceNodeID:      sourceNodeID,
		SourceSeq:         sourceSeq,
		Category:          category,
		Action:            action,
		Payload:           payload,
		CreatedAt:         time.Now().Unix(),
	}, nil
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
