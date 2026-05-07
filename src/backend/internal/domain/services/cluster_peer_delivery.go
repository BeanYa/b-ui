package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
)

type ClusterPeerDeliveryService struct {
	HTTPClient     *http.Client
	saveAckAttempt func(messageID string, targetNode string, status string, errorMessage string) error
}

func (s *ClusterPeerDeliveryService) Send(ctx context.Context, message *PeerMessage, member model.ClusterMember, token string) error {
	_, err := s.sendPeerMessage(ctx, message, member, token)
	return err
}

func (s *ClusterPeerDeliveryService) SendWithResult(ctx context.Context, message *PeerMessage, member model.ClusterMember, token string) (*clustertypes.DomainResourceCommandResult, error) {
	body, err := s.sendPeerMessage(ctx, message, member, token)
	if err != nil {
		return nil, err
	}
	return parseClusterPeerCommandResult(body)
}

func (s *ClusterPeerDeliveryService) sendPeerMessage(ctx context.Context, message *PeerMessage, member model.ClusterMember, token string) ([]byte, error) {
	start := time.Now()
	body, err := s.sendJSON(ctx, message, member, token)
	latency := logger.ClusterLatency(start)
	domainName := ""
	if member.Domain != nil {
		domainName = member.Domain.Domain
	}
	fields := map[string]interface{}{
		"targetNode": member.NodeID,
		"domain":     domainName,
		"action":     message.Action,
		"route":      message.Route.Mode,
		"latency":    latency,
	}
	if err != nil {
		fields["status"] = "failed"
		fields["error"] = err.Error()
		logger.ClusterError(logger.ClusterOutbound, message.Action, fields)
	} else {
		fields["status"] = "ok"
		logger.ClusterInfo(logger.ClusterOutbound, message.Action, fields)
	}
	if shouldRecordPeerAck(message, member) {
		status := PeerAckStatusSucceeded
		errorMessage := ""
		if err != nil {
			status = PeerAckStatusFailed
			errorMessage = err.Error()
		}
		if ackErr := s.getAckAttemptSaver()(message.MessageID, member.NodeID, status, errorMessage); ackErr != nil && err == nil {
			return nil, ackErr
		}
	}
	return body, err
}

func (s *ClusterPeerDeliveryService) SendEnvelope(ctx context.Context, envelope *ClusterEnvelope, member model.ClusterMember, token string) error {
	start := time.Now()
	_, err := s.sendJSON(ctx, envelope, member, token)
	latency := logger.ClusterLatency(start)
	domainName := ""
	if member.Domain != nil {
		domainName = member.Domain.Domain
	}
	fields := map[string]interface{}{
		"targetNode": member.NodeID,
		"domain":     domainName,
		"latency":    latency,
	}
	if err != nil {
		fields["status"] = "failed"
		fields["error"] = err.Error()
		logger.ClusterError(logger.ClusterOutbound, "envelope.send", fields)
	} else {
		fields["status"] = "ok"
		logger.ClusterInfo(logger.ClusterOutbound, "envelope.send", fields)
	}
	return err
}

func (s *ClusterPeerDeliveryService) sendJSON(ctx context.Context, payload interface{}, member model.ClusterMember, token string) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	messageURL, err := clusterPeerMessageURL(member.BaseURL)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, messageURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Cluster-Token", token)
	response, err := s.httpClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "cluster peer notify"); err != nil {
		return nil, clusterPeerNotifyError(response, err)
	}
	return readValidClusterPeerResponseBody(response)
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

func readValidClusterPeerResponseBody(response *http.Response) ([]byte, error) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if _, err := parseClusterPeerCommandResult(body); err != nil {
		return nil, err
	}
	return body, nil
}

func clusterPeerNotifyError(response *http.Response, statusErr error) error {
	if response == nil || response.Body == nil {
		return statusErr
	}
	body, err := io.ReadAll(response.Body)
	if err != nil || len(bytes.TrimSpace(body)) == 0 {
		return statusErr
	}
	var payload struct {
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || strings.TrimSpace(payload.Msg) == "" {
		return statusErr
	}
	return errors.New(statusErr.Error() + ": " + strings.TrimSpace(payload.Msg))
}

func parseClusterPeerCommandResult(body []byte) (*clustertypes.DomainResourceCommandResult, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil
	}
	var payload struct {
		Success bool                                      `json:"success"`
		Msg     string                                    `json:"msg"`
		Result  *clustertypes.DomainResourceCommandResult `json:"result"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("invalid cluster peer response")
	}
	if !payload.Success {
		if payload.Msg == "" {
			return nil, errors.New("cluster peer notify failed")
		}
		return nil, errors.New(payload.Msg)
	}
	return payload.Result, nil
}
