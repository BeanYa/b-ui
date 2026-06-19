package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

const defaultGeoIPEndpoint = "http://ip-api.com/json?fields=status,countryCode,country"

type GeoIPService struct {
	client *http.Client
	url    string
}

func NewGeoIPService(client *http.Client) *GeoIPService {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &GeoIPService{client: client, url: defaultGeoIPEndpoint}
}

// NewGeoIPServiceWithURL is a test helper to override the endpoint.
func NewGeoIPServiceWithURL(client *http.Client, url string) *GeoIPService {
	svc := NewGeoIPService(client)
	svc.url = url
	return svc
}

type geoIPResponse struct {
	Status      string `json:"status"`
	CountryCode string `json:"countryCode"`
	Country     string `json:"country"`
}

// ResolveSelf queries ip-api for the caller's own public IP region.
func (s *GeoIPService) ResolveSelf(ctx context.Context) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	var body geoIPResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", err
	}
	if body.Status != "success" || body.CountryCode == "" {
		return "", "", errors.New("geoip: resolve failed")
	}
	return body.CountryCode, body.Country, nil
}
