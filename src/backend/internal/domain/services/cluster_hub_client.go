package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
}

type ClusterHubRegisterNodeRequest struct {
	RequestID   string                   `json:"request_id"`
	DomainID    string                   `json:"domain_id"`
	DomainToken string                   `json:"domain_token"`
	Member      ClusterHubMemberRegister `json:"member"`
}

type ClusterHubMemberRegister struct {
	MemberID  string `json:"member_id"`
	NodeID    string `json:"node_id"`
	Address   string `json:"address"`
	BaseURL   string `json:"base_url"`
	PublicKey string `json:"public_key"`
	Name      string `json:"name,omitempty"`
}

type ClusterHubMemberResponse struct {
	MemberID     string `json:"member_id"`
	NodeID       string `json:"nodeId"`
	NodeIDAlt    string `json:"node_id"`
	Name         string `json:"name"`
	BaseURL      string `json:"baseUrl"`
	BaseURLAlt   string `json:"base_url"`
	PublicKey    string `json:"publicKey"`
	PublicKeyAlt string `json:"public_key"`
	PeerToken    string `json:"peerToken"`
	PeerTokenAlt string `json:"peer_token"`
	Address      string `json:"address"`
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

type ClusterHubCommunicationResponse struct {
	EndpointPath    string `json:"endpoint_path"`
	ProtocolVersion string `json:"protocol_version"`
}

type ClusterHubSnapshotResponse struct {
	DomainID      string                          `json:"domain_id"`
	Version       int64                           `json:"version"`
	Communication ClusterHubCommunicationResponse `json:"communication"`
	Members       []ClusterHubMemberResponse      `json:"members"`
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

type ClusterHubClient struct {
	HTTPClient *http.Client
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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(hubURL, "/")+"/v1/domains/"+domain+"/version", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Domain-Token", domainToken)
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "hub latest version"); err != nil {
		return nil, err
	}
	decoded := &ClusterHubVersionResponse{}
	if err := json.NewDecoder(response.Body).Decode(decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func (c *ClusterHubClient) GetSnapshot(ctx context.Context, hubURL string, domain string, domainToken string) (*ClusterHubSnapshotResponse, error) {
	if err := validateClusterHubURL(hubURL); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(hubURL, "/")+"/v1/domains/"+domain+"/snapshot", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Domain-Token", domainToken)
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "hub snapshot"); err != nil {
		return nil, err
	}
	decoded := &ClusterHubSnapshotResponse{}
	if err := json.NewDecoder(response.Body).Decode(decoded); err != nil {
		return nil, err
	}
	return decoded, nil
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
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, strings.TrimRight(hubURL, "/")+"/v1/domains/"+domain+"/members/"+memberID, bytes.NewReader(body))
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

func requireHTTPSuccess(response *http.Response, operation string) error {
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	return fmt.Errorf("%s failed: %s", operation, response.Status)
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
