package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type clusterHubClient interface {
	RegisterNode(context.Context, string, ClusterHubRegisterNodeRequest) (*ClusterHubOperationResponse, error)
	GetLatestVersion(context.Context, string, string, string) (*ClusterHubVersionResponse, error)
	GetSnapshot(context.Context, string, string, string) (*ClusterHubSnapshotResponse, error)
	DeleteMember(context.Context, string, string, string, string) (*ClusterHubOperationResponse, error)
	ClaimUpdate(context.Context, string, string, string, string, string) (*ClusterHubClaimUpdateResponse, error)
	SetMemberStatus(context.Context, string, string, string, string, string, string, string) (*ClusterHubMemberStatusResponse, error)
}

type ClusterHubRegisterNodeRequest struct {
	RequestID   string                   `json:"request_id"`
	DomainID    string                   `json:"domain_id"`
	DomainToken string                   `json:"domain_token"`
	Member      ClusterHubMemberRegister `json:"member"`
}

type ClusterHubMemberRegister struct {
	MemberID     string `json:"member_id"`
	NodeID       string `json:"node_id"`
	Address      string `json:"address"`
	BaseURL      string `json:"base_url"`
	PublicKey    string `json:"public_key"`
	Name         string `json:"name,omitempty"`
	DisplayName  string `json:"display_name,omitempty"`
	PanelVersion string `json:"panel_version,omitempty"`
	Status       string `json:"status,omitempty"`
}

type ClusterHubMemberResponse struct {
	MemberID        string `json:"member_id"`
	NodeID          string `json:"nodeId"`
	NodeIDAlt       string `json:"node_id"`
	Name            string `json:"name"`
	DisplayName     string `json:"displayName"`
	DisplayNameAlt  string `json:"display_name"`
	BaseURL         string `json:"baseUrl"`
	BaseURLAlt      string `json:"base_url"`
	PublicKey       string `json:"publicKey"`
	PublicKeyAlt    string `json:"public_key"`
	PeerToken       string `json:"peerToken"`
	PeerTokenAlt    string `json:"peer_token"`
	Address         string `json:"address"`
	PanelVersion    string `json:"panel_version"`
	PanelVersionAlt string `json:"panelVersion"`
	Status          string `json:"status"`
}

type ClusterHubOperationResponse struct {
	OperationID string `json:"operation_id"`
	RequestID   string `json:"request_id"`
	Status      string `json:"status"`
	DomainID    string `json:"domain_id"`
	Type        string `json:"type"`
}

type ClusterHubVersionResponse struct {
	Version int64 `json:"version"`
}

type ClusterHubClaimUpdateResponse struct {
	Proceed       bool   `json:"proceed"`
	TargetVersion string `json:"target_version,omitempty"`
}

type ClusterHubMemberStatusResponse struct {
	OK bool `json:"ok"`
}

type ClusterHubCommunicationResponse struct {
	EndpointPath    string `json:"endpoint_path"`
	ProtocolVersion string `json:"protocol_version"`
}

type ClusterHubSnapshotResponse struct {
	DomainID            string                          `json:"domain_id"`
	Version             int64                           `json:"version"`
	UpdatePolicy        string                          `json:"update_policy"`
	Communication       ClusterHubCommunicationResponse `json:"communication"`
	Members             []ClusterHubMemberResponse      `json:"members"`
	UpdateTargetVersion string                          `json:"update_target_version,omitempty"`
}

func (s ClusterHubSnapshotResponse) EffectiveUpdatePolicy() string {
	return effectiveClusterDomainUpdatePolicy(s.UpdatePolicy)
}

func (s ClusterHubSnapshotResponse) EffectiveCommunicationEndpointPath() string {
	if s.Communication.EndpointPath != "" {
		return s.Communication.EndpointPath
	}
	return ClusterCommunicationEndpointPath
}

func (s ClusterHubSnapshotResponse) EffectiveCommunicationProtocolVersion() string {
	if s.Communication.ProtocolVersion != "" {
		return s.Communication.ProtocolVersion
	}
	return ClusterCommunicationProtocolVersion
}

func (m ClusterHubMemberResponse) EffectiveNodeID() string {
	if m.NodeID != "" {
		return m.NodeID
	}
	return m.NodeIDAlt
}

func (m ClusterHubMemberResponse) EffectiveBaseURL() string {
	if m.BaseURL != "" {
		return m.BaseURL
	}
	if m.BaseURLAlt != "" {
		return m.BaseURLAlt
	}
	return m.Address
}

func (m ClusterHubMemberResponse) EffectiveDisplayName() string {
	if m.DisplayName != "" {
		return m.DisplayName
	}
	if m.DisplayNameAlt != "" {
		return m.DisplayNameAlt
	}
	return m.Name
}

func (m ClusterHubMemberResponse) EffectivePublicKey() string {
	if m.PublicKey != "" {
		return m.PublicKey
	}
	return m.PublicKeyAlt
}

func (m ClusterHubMemberResponse) EffectivePeerToken() string {
	if m.PeerToken != "" {
		return m.PeerToken
	}
	return m.PeerTokenAlt
}

func (m ClusterHubMemberResponse) EffectivePanelVersion() string {
	if m.PanelVersion != "" {
		return m.PanelVersion
	}
	if m.PanelVersionAlt != "" {
		return m.PanelVersionAlt
	}
	return m.PanelVersion
}

func (m ClusterHubMemberResponse) EffectiveStatus() string {
	if m.Status != "" {
		return m.Status
	}
	return "online"
}

type ClusterHubClient struct {
	HTTPClient    *http.Client
	localIdentity clusterLocalIdentityProvider
}

func (c *ClusterHubClient) RegisterNode(ctx context.Context, hubURL string, request ClusterHubRegisterNodeRequest) (*ClusterHubOperationResponse, error) {
	if err := validateClusterHubURL(hubURL); err != nil {
		return nil, err
	}
	response := &ClusterHubOperationResponse{}
	if err := c.postJSON(ctx, strings.TrimRight(hubURL, "/")+"/v1/domains/register", request, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *ClusterHubClient) GetLatestVersion(ctx context.Context, hubURL string, domain string, domainToken string) (*ClusterHubVersionResponse, error) {
	if err := validateClusterHubURL(hubURL); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(hubURL, "/")+"/v1/domains/"+url.PathEscape(domain)+"/version", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Domain-Token", domainToken)
	if err := c.attachReadIdentity(request); err != nil {
		return nil, err
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return decodeClusterHubReadResponse[ClusterHubVersionResponse](response, "hub latest version")
}

func (c *ClusterHubClient) GetSnapshot(ctx context.Context, hubURL string, domain string, domainToken string) (*ClusterHubSnapshotResponse, error) {
	if err := validateClusterHubURL(hubURL); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(hubURL, "/")+"/v1/domains/"+url.PathEscape(domain)+"/snapshot", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Domain-Token", domainToken)
	if err := c.attachReadIdentity(request); err != nil {
		return nil, err
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return decodeClusterHubReadResponse[ClusterHubSnapshotResponse](response, "hub snapshot")
}

func (c *ClusterHubClient) DeleteMember(ctx context.Context, hubURL string, domain string, domainToken string, memberID string) (*ClusterHubOperationResponse, error) {
	if err := validateClusterHubURL(hubURL); err != nil {
		return nil, err
	}
	payload := map[string]string{
		"request_id":   fmt.Sprintf("delete-%d", time.Now().UnixNano()),
		"domain_token": domainToken,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, strings.TrimRight(hubURL, "/")+"/v1/domains/"+url.PathEscape(domain)+"/members/"+url.PathEscape(memberID), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "hub delete member"); err != nil {
		return nil, err
	}
	decoded := &ClusterHubOperationResponse{}
	if err := json.NewDecoder(response.Body).Decode(decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func (c *ClusterHubClient) ClaimUpdate(ctx context.Context, hubURL string, domain string, domainToken string, requestID string, targetVersion string) (*ClusterHubClaimUpdateResponse, error) {
	if err := validateClusterHubURL(hubURL); err != nil {
		return nil, err
	}
	payload := map[string]string{
		"request_id":     requestID,
		"domain_token":   domainToken,
		"target_version": targetVersion,
	}
	response := &ClusterHubClaimUpdateResponse{}
	if err := c.postJSON(ctx, strings.TrimRight(hubURL, "/")+"/v1/domains/"+url.PathEscape(domain)+"/claim-update", payload, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *ClusterHubClient) SetMemberStatus(ctx context.Context, hubURL string, domain string, domainToken string, requestID string, memberID string, status string, panelVersion string) (*ClusterHubMemberStatusResponse, error) {
	if err := validateClusterHubURL(hubURL); err != nil {
		return nil, err
	}
	payload := map[string]string{
		"request_id":   requestID,
		"domain_token": domainToken,
		"member_id":    memberID,
		"status":       status,
	}
	if panelVersion != "" {
		payload["panel_version"] = panelVersion
	}
	response := &ClusterHubMemberStatusResponse{}
	if err := c.postJSON(ctx, strings.TrimRight(hubURL, "/")+"/v1/domains/"+url.PathEscape(domain)+"/member-status", payload, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *ClusterHubClient) postJSON(ctx context.Context, url string, requestBody interface{}, target interface{}) error {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "hub request"); err != nil {
		return err
	}
	return json.NewDecoder(response.Body).Decode(target)
}

type clusterHubReadRejectedError struct {
	Operation string
	Status    string
	Code      string
	Message   string
}

func (e *clusterHubReadRejectedError) Error() string {
	if e == nil {
		return "cluster hub read was rejected"
	}
	detail := strings.TrimSpace(e.Message)
	if detail == "" {
		detail = strings.TrimSpace(e.Code)
	}
	if detail == "" {
		detail = "hub rejected the read request"
	}
	if op := strings.TrimSpace(e.Operation); op != "" {
		return fmt.Sprintf("%s rejected: %s", op, detail)
	}
	return detail
}

type clusterHubProtocolStatusEnvelope struct {
	Status  string `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func decodeClusterHubReadResponse[T any](response *http.Response, operation string) (*T, error) {
	if err := requireHTTPSuccess(response, operation); err != nil {
		return nil, err
	}
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	envelope := &clusterHubProtocolStatusEnvelope{}
	if err := json.Unmarshal(payload, envelope); err == nil {
		switch envelope.Status {
		case "rejected", "failed", "duplicate", "pending":
			return nil, &clusterHubReadRejectedError{
				Operation: operation,
				Status:    envelope.Status,
				Code:      envelope.Code,
				Message:   envelope.Message,
			}
		}
	}
	decoded := new(T)
	if err := json.Unmarshal(payload, decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func requireHTTPSuccess(response *http.Response, operation string) error {
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	return fmt.Errorf("%s failed: %s", operation, response.Status)
}

func (c *ClusterHubClient) attachReadIdentity(request *http.Request) error {
	if c.localIdentity == nil {
		return nil
	}
	local, err := c.localIdentity.GetOrCreate()
	if err != nil {
		return err
	}
	if local == nil || strings.TrimSpace(local.NodeID) == "" {
		return nil
	}
	request.Header.Set("X-Cluster-Node-Id", local.NodeID)
	return nil
}

func validateClusterHubURL(hubURL string) error {
	parsed, err := url.Parse(hubURL)
	if err != nil {
		return err
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme != "http" {
		return errors.New("cluster hub URL must use http or https")
	}
	host := parsed.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return errors.New("cluster hub URL must use https for non-local addresses")
}

func (c *ClusterHubClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}
