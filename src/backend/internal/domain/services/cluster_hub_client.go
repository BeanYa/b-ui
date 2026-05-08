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

	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
)

type clusterHubClient interface {
	RegisterNode(context.Context, string, ClusterHubRegisterNodeRequest) (*ClusterHubOperationResponse, error)
	GetLatestVersion(context.Context, string, string, string) (*ClusterHubVersionResponse, error)
	GetSnapshot(context.Context, string, string, string) (*ClusterHubSnapshotResponse, error)
	DeleteMember(context.Context, string, string, string, string, bool) (*ClusterHubOperationResponse, error)
	ClaimUpdate(context.Context, string, string, string, string, string) (*ClusterHubClaimUpdateResponse, error)
	SetMemberStatus(context.Context, string, string, string, string, string, string, string) (*ClusterHubMemberStatusResponse, error)
	ReportProxyConfigs(ctx context.Context, hubURL string, domain string, body ClusterHubReportProxyConfigsRequest) error
	ReportDomainReport(ctx context.Context, hubURL string, domain string, report ClusterHubReportRequest) error
	ReportDomainResourceState(ctx context.Context, hubURL string, domain string, token string, body ClusterHubResourceStateReportRequest) error
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

type ClusterHubReportProxyConfigsRequest struct {
	RequestID   string                      `json:"request_id"`
	NodeID      string                      `json:"node_id"`
	MemberID    string                      `json:"member_id"`
	DomainToken string                      `json:"domain_token"`
	Signature   string                      `json:"signature"`
	Configs     []ClusterHubProxyConfigItem `json:"configs"`
}

type ClusterHubProxyConfigItem struct {
	InboundID              uint            `json:"inbound_id"`
	Type                   string          `json:"type"`
	Tag                    string          `json:"tag"`
	ListenPort             int             `json:"listen_port"`
	Address                string          `json:"address"`
	Options                json.RawMessage `json:"options"`
	TLSConfig              json.RawMessage `json:"tls_config,omitempty"`
	Scope                  string          `json:"scope,omitempty"`
	DomainInboundRequestID string          `json:"domain_inbound_request_id,omitempty"`
	DomainInboundGroupID   string          `json:"domain_inbound_group_id,omitempty"`
}

type ClusterHubReportRequest struct {
	RequestID   string `json:"request_id"`
	DomainToken string `json:"domain_token"`
	ReportType  string `json:"report_type"`
	GeneratedAt string `json:"generated_at"`
	Data        any    `json:"data"`
}

type ClusterHubResourceStateReportRequest struct {
	ReportID         string                         `json:"report_id"`
	OperationID      string                         `json:"operation_id"`
	ReportedByNodeID string                         `json:"reported_by_node_id"`
	Resources        ClusterHubDomainResources      `json:"resources"`
	OperationSummary ClusterHubDomainOperationState `json:"operation_summary"`
}

type ClusterHubCommunicationResponse struct {
	EndpointPath    string `json:"endpoint_path"`
	ProtocolVersion string `json:"protocol_version"`
}

type ClusterHubSnapshotResponse struct {
	DomainID            string                          `json:"domain_id"`
	Version             int64                           `json:"version"`
	UpdatePolicy        string                          `json:"update_policy"`
	TimeLocation        string                          `json:"time_location"`
	Communication       ClusterHubCommunicationResponse `json:"communication"`
	Members             []ClusterHubMemberResponse      `json:"members"`
	UpdateTargetVersion string                          `json:"update_target_version,omitempty"`
}

func (s ClusterHubSnapshotResponse) EffectiveUpdatePolicy() string {
	return effectiveClusterDomainUpdatePolicy(s.UpdatePolicy)
}

func (s ClusterHubSnapshotResponse) EffectiveTimeLocation() string {
	timeLocation := strings.TrimSpace(s.TimeLocation)
	if timeLocation != "" {
		return timeLocation
	}
	return ClusterDomainDefaultTimeLocation
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

func (m ClusterHubMemberResponse) EffectiveAddress() string {
	if address := normalizeClusterNodeAddress(m.Address); address != "" {
		return address
	}
	return normalizeClusterNodeAddress(m.EffectiveBaseURL())
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
	start := time.Now()
	if err := validateClusterHubURL(hubURL); err != nil {
		c.logHubCall("register_node", request.DomainID, start, err)
		return nil, err
	}
	response := &ClusterHubOperationResponse{}
	if err := c.postJSON(ctx, strings.TrimRight(hubURL, "/")+"/v1/domains/register", request, response); err != nil {
		c.logHubCall("register_node", request.DomainID, start, err)
		return nil, err
	}
	c.logHubCall("register_node", request.DomainID, start, nil)
	return response, nil
}

func (c *ClusterHubClient) GetLatestVersion(ctx context.Context, hubURL string, domain string, domainToken string) (*ClusterHubVersionResponse, error) {
	start := time.Now()
	if err := validateClusterHubURL(hubURL); err != nil {
		c.logHubCall("latest_version", domain, start, err)
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(hubURL, "/")+"/v1/domains/"+url.PathEscape(domain)+"/version", nil)
	if err != nil {
		c.logHubCall("latest_version", domain, start, err)
		return nil, err
	}
	request.Header.Set("X-Domain-Token", domainToken)
	if err := c.attachReadIdentity(request); err != nil {
		c.logHubCall("latest_version", domain, start, err)
		return nil, err
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		c.logHubCall("latest_version", domain, start, err)
		return nil, err
	}
	defer response.Body.Close()
	result, err := decodeClusterHubReadResponse[ClusterHubVersionResponse](response, "hub latest version")
	if err != nil {
		c.logHubCall("latest_version", domain, start, err)
		return nil, err
	}
	c.logHubCall("latest_version", domain, start, nil)
	return result, nil
}

func (c *ClusterHubClient) GetSnapshot(ctx context.Context, hubURL string, domain string, domainToken string) (*ClusterHubSnapshotResponse, error) {
	start := time.Now()
	if err := validateClusterHubURL(hubURL); err != nil {
		c.logHubCall("snapshot", domain, start, err)
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(hubURL, "/")+"/v1/domains/"+url.PathEscape(domain)+"/snapshot", nil)
	if err != nil {
		c.logHubCall("snapshot", domain, start, err)
		return nil, err
	}
	request.Header.Set("X-Domain-Token", domainToken)
	if err := c.attachReadIdentity(request); err != nil {
		c.logHubCall("snapshot", domain, start, err)
		return nil, err
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		c.logHubCall("snapshot", domain, start, err)
		return nil, err
	}
	defer response.Body.Close()
	result, err := decodeClusterHubReadResponse[ClusterHubSnapshotResponse](response, "hub snapshot")
	if err != nil {
		c.logHubCall("snapshot", domain, start, err)
		return nil, err
	}
	c.logHubCall("snapshot", domain, start, nil)
	return result, nil
}

func (c *ClusterHubClient) DeleteMember(ctx context.Context, hubURL string, domain string, domainToken string, memberID string, force bool) (*ClusterHubOperationResponse, error) {
	start := time.Now()
	if err := validateClusterHubURL(hubURL); err != nil {
		c.logHubCall("delete_member", domain, start, err)
		return nil, err
	}
	payload := map[string]any{
		"request_id":   fmt.Sprintf("delete-%d", time.Now().UnixNano()),
		"domain_token": domainToken,
	}
	if force {
		payload["force"] = true
	}
	body, err := json.Marshal(payload)
	if err != nil {
		c.logHubCall("delete_member", domain, start, err)
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, strings.TrimRight(hubURL, "/")+"/v1/domains/"+url.PathEscape(domain)+"/members/"+url.PathEscape(memberID), bytes.NewReader(body))
	if err != nil {
		c.logHubCall("delete_member", domain, start, err)
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient().Do(request)
	if err != nil {
		c.logHubCall("delete_member", domain, start, err)
		return nil, err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "hub delete member"); err != nil {
		c.logHubCall("delete_member", domain, start, err)
		return nil, err
	}
	decoded := &ClusterHubOperationResponse{}
	if err := json.NewDecoder(response.Body).Decode(decoded); err != nil {
		c.logHubCall("delete_member", domain, start, err)
		return nil, err
	}
	c.logHubCall("delete_member", domain, start, nil)
	return decoded, nil
}

func (c *ClusterHubClient) ClaimUpdate(ctx context.Context, hubURL string, domain string, domainToken string, requestID string, targetVersion string) (*ClusterHubClaimUpdateResponse, error) {
	start := time.Now()
	if err := validateClusterHubURL(hubURL); err != nil {
		c.logHubCall("claim_update", domain, start, err)
		return nil, err
	}
	payload := map[string]string{
		"request_id":     requestID,
		"domain_token":   domainToken,
		"target_version": targetVersion,
	}
	response := &ClusterHubClaimUpdateResponse{}
	if err := c.postJSON(ctx, strings.TrimRight(hubURL, "/")+"/v1/domains/"+url.PathEscape(domain)+"/claim-update", payload, response); err != nil {
		c.logHubCall("claim_update", domain, start, err)
		return nil, err
	}
	c.logHubCall("claim_update", domain, start, nil)
	return response, nil
}

func (c *ClusterHubClient) SetMemberStatus(ctx context.Context, hubURL string, domain string, domainToken string, requestID string, memberID string, status string, panelVersion string) (*ClusterHubMemberStatusResponse, error) {
	start := time.Now()
	if err := validateClusterHubURL(hubURL); err != nil {
		c.logHubCall("set_member_status", domain, start, err)
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
		c.logHubCall("set_member_status", domain, start, err)
		return nil, err
	}
	c.logHubCall("set_member_status", domain, start, nil)
	return response, nil
}

func (c *ClusterHubClient) ReportProxyConfigs(ctx context.Context, hubURL string, domain string, body ClusterHubReportProxyConfigsRequest) error {
	start := time.Now()
	if err := validateClusterHubURL(hubURL); err != nil {
		c.logHubCall("report_proxy_configs", domain, start, err)
		return err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		c.logHubCall("report_proxy_configs", domain, start, err)
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, strings.TrimRight(hubURL, "/")+"/v1/domains/"+url.PathEscape(domain)+"/proxy-configs", bytes.NewReader(payload))
	if err != nil {
		c.logHubCall("report_proxy_configs", domain, start, err)
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Domain-Token", body.DomainToken)
	response, err := c.httpClient().Do(request)
	if err != nil {
		c.logHubCall("report_proxy_configs", domain, start, err)
		return err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "hub report proxy configs"); err != nil {
		c.logHubCall("report_proxy_configs", domain, start, err)
		return err
	}
	c.logHubCall("report_proxy_configs", domain, start, nil)
	return nil
}

func (c *ClusterHubClient) ReportDomainReport(ctx context.Context, hubURL string, domain string, report ClusterHubReportRequest) error {
	start := time.Now()
	if err := validateClusterHubURL(hubURL); err != nil {
		c.logHubCall("report_domain", domain, start, err)
		return err
	}
	payload, err := json.Marshal(report)
	if err != nil {
		c.logHubCall("report_domain", domain, start, err)
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut,
		strings.TrimRight(hubURL, "/")+"/v1/domains/"+url.PathEscape(domain)+"/reports/"+url.PathEscape(report.ReportType),
		bytes.NewReader(payload))
	if err != nil {
		c.logHubCall("report_domain", domain, start, err)
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Domain-Token", report.DomainToken)
	response, err := c.httpClient().Do(request)
	if err != nil {
		c.logHubCall("report_domain", domain, start, err)
		return err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "hub report domain"); err != nil {
		c.logHubCall("report_domain", domain, start, err)
		return err
	}
	c.logHubCall("report_domain", domain, start, nil)
	return nil
}

func (c *ClusterHubClient) ReportDomainResourceState(ctx context.Context, hubURL string, domain string, token string, body ClusterHubResourceStateReportRequest) error {
	start := time.Now()
	if err := validateClusterHubURL(hubURL); err != nil {
		c.logHubCall("report_domain_resource_state", domain, start, err)
		return err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		c.logHubCall("report_domain_resource_state", domain, start, err)
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, strings.TrimRight(hubURL, "/")+"/v1/domains/"+url.PathEscape(domain)+"/resource-state", bytes.NewReader(payload))
	if err != nil {
		c.logHubCall("report_domain_resource_state", domain, start, err)
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Domain-Token", token)
	response, err := c.httpClient().Do(request)
	if err != nil {
		c.logHubCall("report_domain_resource_state", domain, start, err)
		return err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "hub report domain resource state"); err != nil {
		c.logHubCall("report_domain_resource_state", domain, start, err)
		return err
	}
	c.logHubCall("report_domain_resource_state", domain, start, nil)
	return nil
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

func (c *ClusterHubClient) logHubCall(operation, domain string, start time.Time, err error) {
	latency := logger.ClusterLatency(start)
	fields := map[string]interface{}{
		"operation": operation,
		"domain":    domain,
		"latency":   latency,
	}
	if err != nil {
		fields["status"] = "failed"
		fields["error"] = err.Error()
		logger.ClusterError(logger.ClusterHub, operation, fields)
	} else {
		fields["status"] = "ok"
		logger.ClusterInfo(logger.ClusterHub, operation, fields)
	}
}
