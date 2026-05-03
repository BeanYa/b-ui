package ping

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const checkHostMaxNodes = 12

type checkHostStartResponse struct {
	OK        int                 `json:"ok"`
	RequestID string              `json:"request_id"`
	Nodes     map[string][]string `json:"nodes"`
}

func (s *ExternalService) runInboundProviderDefault(ctx context.Context, sourceID string, target ExternalEndpoint) []ExternalTestResult {
	if sourceID == "check_host" {
		return s.runCheckHostTCP(ctx, target)
	}
	return []ExternalTestResult{{
		SourceLabel: sourceID,
		Direction:   DirectionInbound,
		Source: ExternalEndpoint{
			ID:       sourceID,
			Label:    sourceID,
			Provider: sourceID,
		},
		TargetMemberID: target.ID,
		TargetName:     target.Label,
		Target:         target,
		Success:        false,
		Error:          errorPtr(fmt.Sprintf("inbound provider %q is not implemented", sourceID)),
	}}
}

func (s *ExternalService) runCheckHostTCP(ctx context.Context, target ExternalEndpoint) []ExternalTestResult {
	start, err := s.startCheckHostTCP(ctx, target)
	if err != nil {
		return []ExternalTestResult{checkHostErrorResult(target, err)}
	}
	rawResults, err := s.fetchCheckHostTCPResults(ctx, start.RequestID)
	if err != nil {
		return []ExternalTestResult{checkHostErrorResult(target, err)}
	}

	nodeIDs := make([]string, 0, len(rawResults))
	for nodeID := range rawResults {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Strings(nodeIDs)

	results := make([]ExternalTestResult, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		source := checkHostNodeEndpoint(nodeID, start.Nodes[nodeID])
		result := ExternalTestResult{
			SourceMemberID: source.ID,
			SourceLabel:    source.Label,
			Direction:      DirectionInbound,
			TargetMemberID: target.ID,
			TargetName:     targetLabel(target),
			Source:         source,
			Target:         target,
		}
		if latencySeconds, ok := checkHostLatencySeconds(rawResults[nodeID]); ok {
			result.Success = true
			result.Method = methodPtr(MethodTCP)
			result.LatencyMs = latencyPtr(latencySeconds * 1000)
		} else {
			result.Success = false
			result.Error = errorPtr(checkHostResultError(rawResults[nodeID]))
		}
		results = append(results, result)
	}

	if len(results) == 0 {
		return []ExternalTestResult{checkHostErrorResult(target, fmt.Errorf("Check-Host returned no TCP results"))}
	}
	return results
}

func (s *ExternalService) startCheckHostTCP(ctx context.Context, target ExternalEndpoint) (*checkHostStartResponse, error) {
	startURL, err := s.checkHostURL("/check-tcp")
	if err != nil {
		return nil, err
	}
	query := startURL.Query()
	query.Set("host", endpointAddress(target))
	query.Set("max_nodes", strconv.Itoa(checkHostMaxNodes))
	startURL.RawQuery = query.Encode()

	var start checkHostStartResponse
	if err := s.doCheckHostJSON(ctx, startURL.String(), &start); err != nil {
		return nil, err
	}
	if start.OK != 1 {
		return nil, fmt.Errorf("Check-Host start response was not ok")
	}
	if strings.TrimSpace(start.RequestID) == "" {
		return nil, fmt.Errorf("Check-Host start response missing request_id")
	}
	return &start, nil
}

func (s *ExternalService) fetchCheckHostTCPResults(ctx context.Context, requestID string) (map[string]interface{}, error) {
	attempts := s.checkHostPollAttempts
	if attempts < 1 {
		attempts = 1
	}

	var results map[string]interface{}
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := sleepCheckHostPoll(ctx, s.checkHostPollDelay); err != nil {
				return nil, err
			}
		}

		resultURL, urlErr := s.checkHostURL("/check-result/" + url.PathEscape(requestID))
		if urlErr != nil {
			return nil, urlErr
		}
		results = make(map[string]interface{})
		err = s.doCheckHostJSON(ctx, resultURL.String(), &results)
		if err != nil {
			return nil, err
		}
		if checkHostHasAnyResult(results) {
			return results, nil
		}
	}
	return results, err
}

func (s *ExternalService) checkHostURL(path string) (*url.URL, error) {
	base := strings.TrimRight(strings.TrimSpace(s.checkHostBaseURL), "/")
	if base == "" {
		base = "https://check-host.net"
	}
	parsed, err := url.Parse(base + path)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func (s *ExternalService) doCheckHostJSON(ctx context.Context, targetURL string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Check-Host returned HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func sleepCheckHostPoll(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func checkHostHasAnyResult(results map[string]interface{}) bool {
	for _, value := range results {
		if value != nil {
			return true
		}
	}
	return false
}

func checkHostErrorResult(target ExternalEndpoint, err error) ExternalTestResult {
	source := ExternalEndpoint{ID: "check_host", Label: "Check-Host", Provider: "check_host"}
	return ExternalTestResult{
		SourceMemberID: source.ID,
		SourceLabel:    source.Label,
		Direction:      DirectionInbound,
		TargetMemberID: target.ID,
		TargetName:     targetLabel(target),
		Source:         source,
		Target:         target,
		Success:        false,
		Error:          errorPtr(err.Error()),
	}
}

func checkHostNodeEndpoint(nodeID string, metadata []string) ExternalEndpoint {
	endpoint := ExternalEndpoint{
		ID:       nodeID,
		Label:    nodeID,
		Provider: "check_host",
		Host:     nodeID,
	}
	if len(metadata) > 1 {
		endpoint.Country = metadata[1]
	}
	if len(metadata) > 2 {
		endpoint.City = metadata[2]
	}
	if len(metadata) > 3 && strings.TrimSpace(metadata[3]) != "" {
		endpoint.Host = metadata[3]
	}
	if len(metadata) > 4 {
		endpoint.Network = metadata[4]
	}
	if endpoint.City != "" && endpoint.Country != "" {
		endpoint.Label = endpoint.City + ", " + endpoint.Country
	} else if endpoint.City != "" {
		endpoint.Label = endpoint.City
	} else if endpoint.Country != "" {
		endpoint.Label = endpoint.Country
	}
	return endpoint
}

func checkHostLatencySeconds(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, false
	case float64:
		return nonNegativeLatency(typed)
	case string:
		return parseLatencyString(typed)
	case []interface{}:
		if len(typed) >= 2 {
			if status, ok := checkHostStatusIndicator(typed[0]); ok {
				if status == 1 {
					if latency, parsed := checkHostLatencySeconds(typed[1]); parsed {
						return latency, true
					}
				}
				return 0, false
			}
		}
		for _, item := range typed {
			if latency, ok := checkHostLatencySeconds(item); ok {
				return latency, true
			}
		}
	case map[string]interface{}:
		for _, key := range []string{"time", "latency", "rtt"} {
			if item, ok := typed[key]; ok {
				if latency, parsed := checkHostLatencySeconds(item); parsed {
					return latency, true
				}
			}
		}
		for key, item := range typed {
			if key == "error" || key == "address" || key == "ip" || key == "status" {
				continue
			}
			if latency, ok := checkHostLatencySeconds(item); ok {
				return latency, true
			}
		}
	}
	return 0, false
}

func nonNegativeLatency(value float64) (float64, bool) {
	if value < 0 {
		return 0, false
	}
	return value, true
}

func parseLatencyString(value string) (float64, bool) {
	latency, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, false
	}
	return nonNegativeLatency(latency)
}

func checkHostStatusIndicator(value interface{}) (int, bool) {
	switch typed := value.(type) {
	case bool:
		if typed {
			return 1, true
		}
		return 0, true
	case float64:
		if typed == 0 || typed == 1 {
			return int(typed), true
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "0" || strings.EqualFold(trimmed, "false") || strings.EqualFold(trimmed, "fail") || strings.EqualFold(trimmed, "failed") {
			return 0, true
		}
		if trimmed == "1" || strings.EqualFold(trimmed, "ok") || strings.EqualFold(trimmed, "true") {
			return 1, true
		}
	}
	return 0, false
}

func checkHostResultError(value interface{}) string {
	if value == nil {
		return "Check-Host TCP result is still pending"
	}
	if message, ok := findCheckHostError(value); ok {
		return message
	}
	return "Check-Host TCP result did not contain a latency"
}

func findCheckHostError(value interface{}) (string, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed != "" {
			return trimmed, true
		}
	case []interface{}:
		for _, item := range typed {
			if message, ok := findCheckHostError(item); ok {
				return message, true
			}
		}
	case map[string]interface{}:
		if item, ok := typed["error"]; ok {
			if message, found := findCheckHostError(item); found {
				return message, true
			}
		}
		for key, item := range typed {
			if key == "time" || key == "latency" || key == "rtt" || key == "address" || key == "ip" {
				continue
			}
			if message, ok := findCheckHostError(item); ok {
				return message, true
			}
		}
	}
	return "", false
}

func targetLabel(target ExternalEndpoint) string {
	if target.Label != "" {
		return target.Label
	}
	return target.Host
}
