package service

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type DomainHintItem struct {
	Domain      string `json:"domain"`
	Status      string `json:"status"`
	TLSVersion  string `json:"tlsVersion,omitempty"`
	ALPN        string `json:"alpn,omitempty"`
	Redirect    bool   `json:"redirect"`
	LatencyMs   int64  `json:"latencyMs,omitempty"`
	Certificate string `json:"certificate,omitempty"`
	Error       string `json:"error,omitempty"`
}

type DomainHintCatalog struct {
	UpdatedAt  int64            `json:"updatedAt"`
	ExpiresAt  int64            `json:"expiresAt"`
	Refreshing bool             `json:"refreshing"`
	Notes      string           `json:"notes"`
	Candidates []string         `json:"candidates,omitempty"`
	Signature  string           `json:"signature,omitempty"`
	Items      []DomainHintItem `json:"items"`
}

type DomainHintService struct {
	SettingService
}

const domainHintTTL = 6 * time.Hour

var domainHintState = struct {
	mu         sync.RWMutex
	refreshing bool
	catalog    DomainHintCatalog
}{}

func (s *DomainHintService) GetCatalog(force bool) DomainHintCatalog {
	now := time.Now()
	candidates := s.loadCandidates()
	signature := strings.Join(candidates, "\n")

	domainHintState.mu.RLock()
	current := domainHintState.catalog
	refreshing := domainHintState.refreshing
	domainHintState.mu.RUnlock()

	if force {
		return s.refreshAndStore(candidates, signature)
	}

	if current.UpdatedAt == 0 || current.Signature != signature {
		if !refreshing {
			go s.refreshAndStore(candidates, signature)
		}
		return s.seedCatalog(now, refreshing, candidates, signature)
	}

	if current.ExpiresAt <= now.Unix() && !refreshing {
		go s.refreshAndStore(candidates, signature)
	}

	current.Refreshing = refreshing
	return current
}

func (s *DomainHintService) Refresh() {
	candidates := s.loadCandidates()
	s.refreshAndStore(candidates, strings.Join(candidates, "\n"))
}

func (s *DomainHintService) refreshAndStore(candidates []string, signature string) DomainHintCatalog {
	domainHintState.mu.Lock()
	if domainHintState.refreshing {
		current := domainHintState.catalog
		refreshing := domainHintState.refreshing
		domainHintState.mu.Unlock()
		if current.UpdatedAt == 0 {
			return s.seedCatalog(time.Now(), refreshing, candidates, signature)
		}
		current.Refreshing = refreshing
		return current
	}
	domainHintState.refreshing = true
	domainHintState.mu.Unlock()

	catalog := s.buildCatalog(candidates, signature)

	domainHintState.mu.Lock()
	domainHintState.catalog = catalog
	domainHintState.refreshing = false
	domainHintState.catalog.Refreshing = false
	current := domainHintState.catalog
	domainHintState.mu.Unlock()

	return current
}

func (s *DomainHintService) seedCatalog(now time.Time, refreshing bool, candidates []string, signature string) DomainHintCatalog {
	items := make([]DomainHintItem, 0, len(candidates))
	for _, domain := range candidates {
		items = append(items, DomainHintItem{
			Domain: domain,
			Status: "builtin",
		})
	}

	return DomainHintCatalog{
		UpdatedAt:  0,
		ExpiresAt:  now.Add(domainHintTTL).Unix(),
		Refreshing: refreshing,
		Notes:      "Basic check covers TLS1.3, ALPN, redirect and latency. Custom domains are still allowed.",
		Candidates: candidates,
		Signature:  signature,
		Items:      items,
	}
}

func (s *DomainHintService) buildCatalog(candidates []string, signature string) DomainHintCatalog {
	items := make([]DomainHintItem, 0, len(candidates))
	resultCh := make(chan DomainHintItem, len(candidates))
	workerCh := make(chan string, len(candidates))

	workerCount := 4
	if len(candidates) < workerCount {
		workerCount = len(candidates)
	}
	if workerCount == 0 {
		workerCount = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i += 1 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for domain := range workerCh {
				resultCh <- s.checkDomain(domain)
			}
		}()
	}

	for _, domain := range candidates {
		workerCh <- domain
	}
	close(workerCh)

	wg.Wait()
	close(resultCh)

	for item := range resultCh {
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		left := scoreDomainHint(items[i])
		right := scoreDomainHint(items[j])
		if left == right {
			if items[i].LatencyMs == items[j].LatencyMs {
				return items[i].Domain < items[j].Domain
			}
			if items[i].LatencyMs == 0 {
				return false
			}
			if items[j].LatencyMs == 0 {
				return true
			}
			return items[i].LatencyMs < items[j].LatencyMs
		}
		return left > right
	})

	now := time.Now()
	return DomainHintCatalog{
		UpdatedAt:  now.Unix(),
		ExpiresAt:  now.Add(domainHintTTL).Unix(),
		Refreshing: false,
		Notes:      "Basic check covers TLS1.3, ALPN, redirect and latency. Custom domains are still allowed.",
		Candidates: candidates,
		Signature:  signature,
		Items:      items,
	}
}

func (s *DomainHintService) loadCandidates() []string {
	candidates, err := s.SettingService.GetTLSDomainHints()
	if err == nil && len(candidates) > 0 {
		return candidates
	}
	return []string{
		"www.youtube.com",
		"www.cloudflare.com",
		"www.apple.com",
		"www.microsoft.com",
		"www.amazon.com",
		"www.github.com",
		"www.nvidia.com",
		"www.adobe.com",
		"www.bing.com",
		"www.dropbox.com",
	}
}

func (s *DomainHintService) checkDomain(domain string) DomainHintItem {
	item := DomainHintItem{
		Domain: domain,
		Status: "builtin",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	start := time.Now()
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(domain, "443"), &tls.Config{
		ServerName:         domain,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		NextProtos:         []string{"h2", "http/1.1"},
		InsecureSkipVerify: true,
	})
	if err != nil {
		item.Status = "failed"
		item.Error = err.Error()
		return item
	}

	state := conn.ConnectionState()
	_ = conn.Close()

	item.LatencyMs = time.Since(start).Milliseconds()
	item.TLSVersion = tlsVersionLabel(state.Version)
	item.ALPN = state.NegotiatedProtocol
	if len(state.PeerCertificates) > 0 {
		item.Certificate = state.PeerCertificates[0].Subject.CommonName
		if item.Certificate == "" && len(state.PeerCertificates[0].DNSNames) > 0 {
			item.Certificate = state.PeerCertificates[0].DNSNames[0]
		}
	}

	redirect, err := s.checkRedirect(ctx, domain)
	if err != nil {
		item.Error = err.Error()
	}
	item.Redirect = redirect

	item.Status = classifyDomainHint(item)
	return item
}

func (s *DomainHintService) checkRedirect(ctx context.Context, domain string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+domain, nil)
	if err != nil {
		return false, err
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location == "" {
			return true, nil
		}
		target, parseErr := url.Parse(location)
		if parseErr != nil {
			return true, nil
		}
		targetHost := strings.TrimPrefix(target.Hostname(), "www.")
		sourceHost := strings.TrimPrefix(domain, "www.")
		return targetHost != "" && targetHost != sourceHost, nil
	}

	return false, nil
}

func classifyDomainHint(item DomainHintItem) string {
	if item.Error != "" {
		return "failed"
	}
	if item.TLSVersion == "TLS 1.3" && item.ALPN == "h2" && !item.Redirect {
		return "recommended"
	}
	if item.TLSVersion == "TLS 1.3" && item.ALPN != "" {
		return "available"
	}
	return "limited"
}

func scoreDomainHint(item DomainHintItem) int {
	switch item.Status {
	case "recommended":
		return 4
	case "available":
		return 3
	case "limited":
		return 2
	case "builtin":
		return 1
	default:
		return 0
	}
}

func tlsVersionLabel(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	default:
		return ""
	}
}
