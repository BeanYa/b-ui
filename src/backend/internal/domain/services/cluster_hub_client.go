package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type clusterHubClient interface {
	RegisterNode(context.Context, string, ClusterHubRegisterNodeRequest) (*ClusterHubRegisterNodeResponse, error)
	GetLatestVersion(context.Context, string, string) (*ClusterHubVersionResponse, error)
	GetSnapshot(context.Context, string, string) (*ClusterHubSnapshotResponse, error)
}

type ClusterHubRegisterNodeRequest struct {
	NodeID    string `json:"nodeId"`
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	PublicKey string `json:"publicKey"`
	BaseURL   string `json:"baseUrl"`
}

type ClusterHubMemberResponse struct {
	NodeID    string `json:"nodeId"`
	Name      string `json:"name"`
	BaseURL   string `json:"baseUrl"`
	PublicKey string `json:"publicKey"`
}

type ClusterHubRegisterNodeResponse struct {
	Member  ClusterHubMemberResponse `json:"member"`
	Message string                   `json:"message"`
}

type ClusterHubVersionResponse struct {
	Version int64 `json:"version"`
}

type ClusterHubSnapshotResponse struct {
	Version int64                      `json:"version"`
	Members []ClusterHubMemberResponse `json:"members"`
}

type ClusterHubClient struct {
	HTTPClient *http.Client
}

func (c *ClusterHubClient) RegisterNode(ctx context.Context, hubURL string, request ClusterHubRegisterNodeRequest) (*ClusterHubRegisterNodeResponse, error) {
	response := &ClusterHubRegisterNodeResponse{}
	if err := c.postJSON(ctx, strings.TrimRight(hubURL, "/")+"/register", request, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *ClusterHubClient) GetLatestVersion(ctx context.Context, hubURL string, domain string) (*ClusterHubVersionResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(hubURL, "/")+"/version?domain="+domain, nil)
	if err != nil {
		return nil, err
	}
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

func (c *ClusterHubClient) GetSnapshot(ctx context.Context, hubURL string, domain string) (*ClusterHubSnapshotResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(hubURL, "/")+"/snapshot?domain="+domain, nil)
	if err != nil {
		return nil, err
	}
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

func (c *ClusterHubClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}
