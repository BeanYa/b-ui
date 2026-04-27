package ping

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Store struct {
	dataDir string
}

func NewStore() *Store {
	return &Store{dataDir: DataDir}
}

func (s *Store) meshDir() string {
	return filepath.Join(s.dataDir, MeshSubDir)
}

func (s *Store) externalDir() string {
	return filepath.Join(s.dataDir, ExternalSubDir)
}

func (s *Store) meshPath(domainID string) string {
	return filepath.Join(s.meshDir(), sanitizeFileName(domainID)+".json")
}

func (s *Store) configPath() string {
	return filepath.Join(s.externalDir(), ConfigFile)
}

func (s *Store) resultsPath() string {
	return filepath.Join(s.externalDir(), ResultsFile)
}

func (s *Store) SaveMeshResult(result *MeshResult) error {
	if err := os.MkdirAll(s.meshDir(), 0755); err != nil {
		return err
	}
	return writeJSON(s.meshPath(result.DomainID), result)
}

func (s *Store) LoadMeshResult(domainID string) (*MeshResult, error) {
	var result MeshResult
	if err := readJSON(s.meshPath(domainID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Store) SaveExternalConfig(config *ExternalConfig) error {
	if err := os.MkdirAll(s.externalDir(), 0755); err != nil {
		return err
	}
	return writeJSON(s.configPath(), config)
}

func (s *Store) LoadExternalConfig() (*ExternalConfig, error) {
	var config ExternalConfig
	if err := readJSON(s.configPath(), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (s *Store) LoadExternalConfigOrDefault() *ExternalConfig {
	config, err := s.LoadExternalConfig()
	if err != nil {
		return defaultExternalConfig()
	}
	return config
}

func (s *Store) SaveExternalResults(data *ExternalResultData) error {
	if err := os.MkdirAll(s.externalDir(), 0755); err != nil {
		return err
	}
	return writeJSON(s.resultsPath(), data)
}

func (s *Store) LoadExternalResults() (*ExternalResultData, error) {
	var data ExternalResultData
	if err := readJSON(s.resultsPath(), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func readJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func sanitizeFileName(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}

func defaultExternalConfig() *ExternalConfig {
	return &ExternalConfig{
		Sources: []ExternalSource{
			{ID: "ripe_atlas", Name: "RIPE Atlas", Type: "rest_api", Direction: "inbound", Enabled: true},
			{ID: "cloudflare_workers", Name: "Cloudflare Workers", Type: "self_hosted", Direction: "inbound", Enabled: false},
			{ID: "linode_lg", Name: "Linode Looking Glass", Type: "web_scrape", Direction: "inbound", Enabled: true},
			{ID: "he_lg", Name: "Hurricane Electric LG", Type: "web_scrape", Direction: "inbound", Enabled: true},
			{ID: "zstatic_cdn", Name: "Zstatic CDN", Type: "cdn_ping", Direction: "inbound", Enabled: true},
			{ID: "cloud_test_ips", Name: "Cloud Provider Test IPs", Type: "icmp_tcp", Direction: "outbound", Enabled: true},
			{ID: "speedtest_net", Name: "Speedtest.net Servers", Type: "rest_api", Direction: "outbound", Enabled: false},
			{ID: "public_dns", Name: "Public DNS", Type: "icmp", Direction: "outbound", Enabled: true},
			{ID: "cdn_edges", Name: "CDN Edge Nodes", Type: "http_icmp", Direction: "outbound", Enabled: true},
			{ID: "ix_isp_lg", Name: "IX/ISP Looking Glass", Type: "icmp_mtr", Direction: "outbound", Enabled: true},
		},
	}
}
