package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

type ClusterPeerDeliveryService struct {
	HTTPClient     *http.Client
	saveAckAttempt func(messageID string, targetNode string, status string, errorMessage string) error
}

func (s *ClusterPeerDeliveryService) Send(ctx context.Context, message *PeerMessage, member model.ClusterMember, token string) error {
	err := s.sendJSON(ctx, message, member, token)
	if shouldRecordPeerAck(message, member) {
		status := PeerAckStatusSucceeded
		errorMessage := ""
		if err != nil {
			status = PeerAckStatusFailed
			errorMessage = err.Error()
		}
		if ackErr := s.getAckAttemptSaver()(message.MessageID, member.NodeID, status, errorMessage); ackErr != nil && err == nil {
			return ackErr
		}
	}
	return err
}

func (s *ClusterPeerDeliveryService) SendEnvelope(ctx context.Context, envelope *ClusterEnvelope, member model.ClusterMember, token string) error {
	return s.sendJSON(ctx, envelope, member, token)
}

func (s *ClusterPeerDeliveryService) sendJSON(ctx context.Context, payload interface{}, member model.ClusterMember, token string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	messageURL, err := clusterPeerMessageURL(member.BaseURL)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, messageURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Cluster-Token", token)
	response, err := s.httpClient().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "cluster peer notify"); err != nil {
		return err
	}
	return requireClusterPeerSuccess(response)
}

func (s *ClusterPeerDeliveryService) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (s *ClusterPeerDeliveryService) getAckAttemptSaver() func(messageID string, targetNode string, status string, errorMessage string) error {
	if s.saveAckAttempt != nil {
		return s.saveAckAttempt
	}
	return SaveClusterPeerAckAttempt
}

func shouldRecordPeerAck(message *PeerMessage, member model.ClusterMember) bool {
	if message == nil || member.NodeID == "" || message.Route.Delivery == nil {
		return false
	}
	return message.Route.Delivery.Ack != "" && message.Route.Delivery.Ack != DeliveryAckNone
}

func ExpandClusterPeerRoute(route RoutePlan, members []model.ClusterMember, sourceNodeID string) []model.ClusterMember {
	switch route.Mode {
	case RouteModeBroadcast, RouteModeScheduledBroadcast:
		targets := make([]model.ClusterMember, 0, len(members))
		for _, member := range members {
			if isEligibleClusterPeerMember(member, route.Selector, sourceNodeID) {
				targets = append(targets, member)
			}
		}
		return targets
	case RouteModeDirect:
		if len(route.Targets) != 1 {
			return nil
		}
		membersByNodeID := clusterPeerMembersByNodeID(members)
		member, ok := membersByNodeID[route.Targets[0]]
		if !ok || !isEligibleClusterPeerMember(member, route.Selector, sourceNodeID) {
			return nil
		}
		return []model.ClusterMember{member}
	case RouteModeMulticast:
		membersByNodeID := clusterPeerMembersByNodeID(members)
		targets := make([]model.ClusterMember, 0, len(route.Targets))
		for _, nodeID := range route.Targets {
			member, ok := membersByNodeID[nodeID]
			if ok && isEligibleClusterPeerMember(member, route.Selector, sourceNodeID) {
				targets = append(targets, member)
			}
		}
		return targets
	default:
		return nil
	}
}

func clusterPeerMembersByNodeID(members []model.ClusterMember) map[string]model.ClusterMember {
	membersByNodeID := make(map[string]model.ClusterMember, len(members))
	for _, member := range members {
		membersByNodeID[member.NodeID] = member
	}
	return membersByNodeID
}

func isEligibleClusterPeerMember(member model.ClusterMember, selector *TargetSelector, sourceNodeID string) bool {
	if member.NodeID == "" || member.BaseURL == "" || member.NodeID == sourceNodeID {
		return false
	}
	if selector == nil {
		return true
	}
	if len(selector.CapabilityRequired) > 0 {
		return false
	}
	if len(selector.Include) > 0 && !containsClusterNodeID(selector.Include, member.NodeID) {
		return false
	}
	return !containsClusterNodeID(selector.Exclude, member.NodeID)
}

func containsClusterNodeID(nodeIDs []string, nodeID string) bool {
	for _, candidate := range nodeIDs {
		if candidate == nodeID {
			return true
		}
	}
	return false
}
