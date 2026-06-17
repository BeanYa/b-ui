# External Ping Target Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a refreshable outbound ping target catalog and change Multi-Location Ping so admins select concrete target nodes across providers before running outbound tests.

**Architecture:** Move outbound targets into a catalog loaded from runtime data with an embedded seed fallback. Provider refreshers update the catalog through both an admin API and a CLI command, while outbound tests filter concrete endpoints using `target_node_ids`. The frontend loads the catalog, displays provider and region/city groups, defaults to no selected targets, and sends selected target IDs with the outbound request.

**Tech Stack:** Go backend with Gin APIs, Go tests, embedded JSON seed data, shell wrapper script, Vue 3 + Pinia + Vuetify frontend, Vitest source-level frontend tests.

---

## File Structure

- Create `src/backend/internal/domain/services/ping/catalog_types.go`: target catalog structs and helper methods.
- Create `src/backend/internal/domain/services/ping/catalog_seed.go`: embedded seed catalog loader.
- Create `src/backend/internal/domain/services/ping/catalogs/external_targets.seed.json`: committed seed catalog for static providers and initial dynamic providers.
- Create `src/backend/internal/domain/services/ping/catalog_refresh.go`: catalog service, refresh orchestration, provider preservation, target filtering helpers.
- Create `src/backend/internal/domain/services/ping/catalog_zstatic.go`: ZStaticCDN entry-page discovery, page metadata parser, and node data parser.
- Create `src/backend/internal/domain/services/ping/catalog_linode.go`: Linode/Akamai speedtest target refresher.
- Create `src/backend/internal/domain/services/ping/catalog_test.go`: storage, filtering, and refresh tests.
- Create `src/backend/internal/domain/services/ping/testdata/zstatic_nodes_data.js`: small fixture matching ZStaticCDN script shape.
- Modify `src/backend/internal/domain/services/ping/types.go`: add catalog metadata fields to `ExternalEndpoint` and keep `ExternalRunRequest.TargetNodeIDs`.
- Modify `src/backend/internal/domain/services/ping/storage.go`: add `targets.json` path and save/load methods.
- Modify `src/backend/internal/domain/services/ping/target_providers.go`: replace hard-coded provider functions with catalog-backed seed helpers or delete provider-specific functions after callers move.
- Modify `src/backend/internal/domain/services/ping/external.go`: enforce selected outbound targets in `Run` and filter targets in `runOutboundWithMethods`.
- Modify `src/backend/internal/http/api/ping.go`: add target catalog routes and handler methods.
- Modify `src/backend/internal/http/api/ping_test.go`: add refresh and target list API tests; assert selected target IDs pass through unchanged.
- Modify `src/backend/internal/cli/cmd.go`: add `external-targets` subcommand.
- Create `src/backend/internal/cli/external_targets.go`: CLI command implementation using the ping catalog service.
- Create `scripts/dev/refresh-external-targets.sh`: wrapper command for local catalog refresh.
- Modify `src/frontend/src/types/ping.ts`: add catalog types and metadata fields.
- Modify `src/frontend/src/store/modules/ping.ts`: add catalog load/refresh state and actions.
- Modify `src/frontend/src/store/modules/ping.test.ts`: add API request tests for catalog load/refresh.
- Modify `src/frontend/src/views/MultiLocationPing.vue`: replace outbound provider table with grouped target picker and refresh button.
- Modify `src/frontend/src/views/MultiLocationPing.test.ts`: update source-level tests for `target_node_ids`, disabled empty selection, and refresh button wiring.

## Task 1: Backend Catalog Types, Seed, and Storage

**Files:**
- Create: `src/backend/internal/domain/services/ping/catalog_types.go`
- Create: `src/backend/internal/domain/services/ping/catalog_seed.go`
- Create: `src/backend/internal/domain/services/ping/catalogs/external_targets.seed.json`
- Modify: `src/backend/internal/domain/services/ping/types.go`
- Modify: `src/backend/internal/domain/services/ping/storage.go`
- Modify: `src/backend/internal/domain/services/ping/storage_test.go`

- [ ] **Step 1: Write failing storage and seed tests**

Add these tests to `src/backend/internal/domain/services/ping/storage_test.go`:

```go
func TestSaveAndLoadExternalTargetCatalog(t *testing.T) {
	dir := t.TempDir()
	store := newStoreWithDir(dir)
	catalog := &ExternalTargetCatalog{
		UpdatedAt: 1710000000,
		Providers: []ExternalTargetProviderCatalog{{
			ProviderID:   "public_dns",
			ProviderName: "Public DNS",
			Static:       true,
			Targets: []ExternalEndpoint{{
				ID: "public_dns:cloudflare-dns", Label: "Cloudflare DNS", Provider: "public_dns",
				Group: "Global", Country: "Global", Network: "Cloudflare", Host: "1.1.1.1", Port: 53,
				Methods: []string{MethodTCP, MethodICMP},
			}},
		}},
	}

	if err := store.SaveExternalTargetCatalog(catalog); err != nil {
		t.Fatalf("SaveExternalTargetCatalog: %v", err)
	}
	loaded, err := store.LoadExternalTargetCatalog()
	if err != nil {
		t.Fatalf("LoadExternalTargetCatalog: %v", err)
	}
	if loaded.UpdatedAt != catalog.UpdatedAt {
		t.Fatalf("expected updated_at %d, got %d", catalog.UpdatedAt, loaded.UpdatedAt)
	}
	if len(loaded.Providers) != 1 || len(loaded.Providers[0].Targets) != 1 {
		t.Fatalf("expected one provider with one target, got %#v", loaded)
	}
	if loaded.Providers[0].Targets[0].Group != "Global" {
		t.Fatalf("expected target group Global, got %#v", loaded.Providers[0].Targets[0])
	}
}

func TestLoadSeedExternalTargetCatalog(t *testing.T) {
	catalog, err := loadSeedExternalTargetCatalog()
	if err != nil {
		t.Fatalf("loadSeedExternalTargetCatalog: %v", err)
	}
	if len(catalog.Providers) < 5 {
		t.Fatalf("expected seed providers, got %#v", catalog.Providers)
	}
	if len(catalog.TargetsForProvider("public_dns")) == 0 {
		t.Fatal("expected public_dns seed targets")
	}
}
```

- [ ] **Step 2: Run tests and verify red**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui go test ./src/backend/internal/domain/services/ping -run 'TestSaveAndLoadExternalTargetCatalog|TestLoadSeedExternalTargetCatalog' -count=1
```

Expected: FAIL because `ExternalTargetCatalog`, `SaveExternalTargetCatalog`, `LoadExternalTargetCatalog`, and `loadSeedExternalTargetCatalog` do not exist.

- [ ] **Step 3: Add catalog structs and endpoint metadata**

Create `src/backend/internal/domain/services/ping/catalog_types.go`:

```go
package ping

type ExternalTargetCatalog struct {
	UpdatedAt int64                           `json:"updated_at"`
	Providers []ExternalTargetProviderCatalog `json:"providers"`
}

type ExternalTargetProviderCatalog struct {
	ProviderID   string             `json:"provider_id"`
	ProviderName string             `json:"provider_name"`
	Static       bool               `json:"static,omitempty"`
	UpdatedAt    int64              `json:"updated_at,omitempty"`
	Targets      []ExternalEndpoint `json:"targets"`
}

func (c *ExternalTargetCatalog) TargetsForProvider(providerID string) []ExternalEndpoint {
	if c == nil {
		return nil
	}
	for _, provider := range c.Providers {
		if provider.ProviderID == providerID {
			return provider.Targets
		}
	}
	return nil
}

func (c *ExternalTargetCatalog) TargetByID(id string) (ExternalEndpoint, bool) {
	if c == nil {
		return ExternalEndpoint{}, false
	}
	for _, provider := range c.Providers {
		for _, target := range provider.Targets {
			if target.ID == id {
				return target, true
			}
		}
	}
	return ExternalEndpoint{}, false
}
```

Modify `ExternalEndpoint` in `src/backend/internal/domain/services/ping/types.go`:

```go
type ExternalEndpoint struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Provider string   `json:"provider"`
	Region   string   `json:"region,omitempty"`
	Country  string   `json:"country,omitempty"`
	City     string   `json:"city,omitempty"`
	Network  string   `json:"network,omitempty"`
	Group    string   `json:"group,omitempty"`
	Level    string   `json:"level,omitempty"`
	Host     string   `json:"host,omitempty"`
	Port     int      `json:"port,omitempty"`
	Methods  []string `json:"methods,omitempty"`
}
```

- [ ] **Step 4: Add storage methods**

Modify constants in `src/backend/internal/domain/services/ping/types.go`:

```go
TargetCatalogFile = "targets.json"
```

Add to `src/backend/internal/domain/services/ping/storage.go`:

```go
func NewStoreWithDataDir(dataDir string) *Store {
	return &Store{dataDir: dataDir}
}

func (s *Store) targetCatalogPath() string {
	return filepath.Join(s.externalDir(), TargetCatalogFile)
}

func (s *Store) SaveExternalTargetCatalog(catalog *ExternalTargetCatalog) error {
	if err := os.MkdirAll(s.externalDir(), 0755); err != nil {
		return err
	}
	return writeJSON(s.targetCatalogPath(), catalog)
}

func (s *Store) LoadExternalTargetCatalog() (*ExternalTargetCatalog, error) {
	var catalog ExternalTargetCatalog
	if err := readJSON(s.targetCatalogPath(), &catalog); err != nil {
		return nil, err
	}
	return &catalog, nil
}
```

- [ ] **Step 5: Update tests to use exported temp-store helper**

Change `newStoreWithDir` in `src/backend/internal/domain/services/ping/storage_test.go`:

```go
func newStoreWithDir(dir string) *Store {
	return NewStoreWithDataDir(filepath.Join(dir, DataDir))
}
```

- [ ] **Step 6: Add seed catalog**

Create `src/backend/internal/domain/services/ping/catalogs/external_targets.seed.json` with the existing static targets and enough dynamic seed targets to keep current behavior usable:

```json
{
  "updated_at": 0,
  "providers": [
    {
      "provider_id": "zstatic_cdn",
      "provider_name": "ZStaticCDN",
      "targets": [
        { "id": "zstatic_cdn:he-cm-v4", "label": "河北移动", "provider": "zstatic_cdn", "region": "河北", "country": "CN", "network": "China Mobile", "group": "河北", "level": "province", "host": "he-cm-v4.ip.zstaticcdn.com", "port": 80, "methods": ["tcp"] },
        { "id": "zstatic_cdn:he-cu-v4", "label": "河北联通", "provider": "zstatic_cdn", "region": "河北", "country": "CN", "network": "China Unicom", "group": "河北", "level": "province", "host": "he-cu-v4.ip.zstaticcdn.com", "port": 80, "methods": ["tcp"] },
        { "id": "zstatic_cdn:he-ct-v4", "label": "河北电信", "provider": "zstatic_cdn", "region": "河北", "country": "CN", "network": "China Telecom", "group": "河北", "level": "province", "host": "he-ct-v4.ip.zstaticcdn.com", "port": 80, "methods": ["tcp"] }
      ]
    },
    {
      "provider_id": "linode_speedtest",
      "provider_name": "Linode/Akamai Speed Test",
      "targets": [
        { "id": "linode_speedtest:newark", "label": "Linode Newark", "provider": "linode_speedtest", "region": "Newark", "country": "US", "group": "United States", "host": "speedtest.newark.linode.com", "port": 80, "methods": ["tcp", "http", "icmp"] },
        { "id": "linode_speedtest:fremont", "label": "Linode Fremont", "provider": "linode_speedtest", "region": "Fremont", "country": "US", "group": "United States", "host": "speedtest.fremont.linode.com", "port": 80, "methods": ["tcp", "http", "icmp"] },
        { "id": "linode_speedtest:frankfurt", "label": "Linode Frankfurt", "provider": "linode_speedtest", "region": "Frankfurt", "country": "DE", "group": "Europe", "host": "speedtest.frankfurt.linode.com", "port": 80, "methods": ["tcp", "http", "icmp"] },
        { "id": "linode_speedtest:singapore", "label": "Linode Singapore", "provider": "linode_speedtest", "region": "Singapore", "country": "SG", "group": "Asia Pacific", "host": "speedtest.singapore.linode.com", "port": 80, "methods": ["tcp", "http", "icmp"] }
      ]
    },
    {
      "provider_id": "public_dns",
      "provider_name": "Public DNS",
      "static": true,
      "targets": [
        { "id": "public_dns:cloudflare-dns", "label": "Cloudflare DNS", "provider": "public_dns", "country": "Global", "network": "Cloudflare", "group": "Global", "host": "1.1.1.1", "port": 53, "methods": ["tcp", "icmp"] },
        { "id": "public_dns:google-dns", "label": "Google DNS", "provider": "public_dns", "country": "Global", "network": "Google", "group": "Global", "host": "8.8.8.8", "port": 53, "methods": ["tcp", "icmp"] },
        { "id": "public_dns:quad9-dns", "label": "Quad9 DNS", "provider": "public_dns", "country": "Global", "network": "Quad9", "group": "Global", "host": "9.9.9.9", "port": 53, "methods": ["tcp", "icmp"] },
        { "id": "public_dns:114-dns", "label": "114 DNS", "provider": "public_dns", "country": "CN", "network": "114DNS", "group": "China", "host": "114.114.114.114", "port": 53, "methods": ["tcp", "icmp"] }
      ]
    },
    {
      "provider_id": "cdn_edges",
      "provider_name": "CDN Edge Nodes",
      "static": true,
      "targets": [
        { "id": "cdn_edges:cloudflare-edge", "label": "Cloudflare Edge", "provider": "cdn_edges", "country": "Global", "network": "Cloudflare", "group": "Global", "host": "cloudflare.com", "port": 443, "methods": ["http", "tcp"] },
        { "id": "cdn_edges:fastly-edge", "label": "Fastly Edge", "provider": "cdn_edges", "country": "Global", "network": "Fastly", "group": "Global", "host": "www.fastly.com", "port": 443, "methods": ["http", "tcp"] },
        { "id": "cdn_edges:akamai-edge", "label": "Akamai Edge", "provider": "cdn_edges", "country": "Global", "network": "Akamai", "group": "Global", "host": "www.akamai.com", "port": 443, "methods": ["http", "tcp"] }
      ]
    },
    {
      "provider_id": "cloud_test_ips",
      "provider_name": "Cloud Provider Test IPs",
      "static": true,
      "targets": [
        { "id": "cloud_test_ips:aws-tokyo", "label": "AWS Tokyo", "provider": "cloud_test_ips", "region": "Tokyo", "country": "JP", "network": "AWS", "group": "Asia Pacific", "host": "ec2.ap-northeast-1.amazonaws.com", "port": 443, "methods": ["tcp", "http"] },
        { "id": "cloud_test_ips:aws-singapore", "label": "AWS Singapore", "provider": "cloud_test_ips", "region": "Singapore", "country": "SG", "network": "AWS", "group": "Asia Pacific", "host": "ec2.ap-southeast-1.amazonaws.com", "port": 443, "methods": ["tcp", "http"] },
        { "id": "cloud_test_ips:aws-virginia", "label": "AWS N. Virginia", "provider": "cloud_test_ips", "region": "Virginia", "country": "US", "network": "AWS", "group": "United States", "host": "ec2.us-east-1.amazonaws.com", "port": 443, "methods": ["tcp", "http"] }
      ]
    }
  ]
}
```

Create `src/backend/internal/domain/services/ping/catalog_seed.go`:

```go
package ping

import (
	"embed"
	"encoding/json"
)

//go:embed catalogs/external_targets.seed.json
var externalTargetCatalogSeedFS embed.FS

func loadSeedExternalTargetCatalog() (*ExternalTargetCatalog, error) {
	data, err := externalTargetCatalogSeedFS.ReadFile("catalogs/external_targets.seed.json")
	if err != nil {
		return nil, err
	}
	var catalog ExternalTargetCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}
	return &catalog, nil
}
```

- [ ] **Step 7: Run tests and commit**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui go test ./src/backend/internal/domain/services/ping -run 'TestSaveAndLoadExternalTargetCatalog|TestLoadSeedExternalTargetCatalog' -count=1
```

Expected: PASS.

Commit:

```bash
git -c safe.directory=/home/bean/workspace/bproject/b-ui add src/backend/internal/domain/services/ping
git -c safe.directory=/home/bean/workspace/bproject/b-ui commit -m "feat: add external target catalog storage"
```

## Task 2: ZStaticCDN and Linode Refreshers

**Files:**
- Create: `src/backend/internal/domain/services/ping/catalog_zstatic.go`
- Create: `src/backend/internal/domain/services/ping/catalog_linode.go`
- Create: `src/backend/internal/domain/services/ping/catalog_refresh.go`
- Create: `src/backend/internal/domain/services/ping/catalog_test.go`
- Create: `src/backend/internal/domain/services/ping/testdata/zstatic_nodes_data.js`

- [ ] **Step 1: Add ZStatic fixture**

Create `src/backend/internal/domain/services/ping/testdata/zstatic_nodes_data.js`:

```javascript
window.nodeData = {
  provinceBaseData: [
    {
      province: "河北",
      carriers: {
        mobile: "he-cm-v4.ip.zstaticcdn.com:80",
        unicom: "he-cu-v4.ip.zstaticcdn.com:80",
        telecom: "he-ct-v4.ip.zstaticcdn.com:80"
      }
    }
  ],
  cityKeyList: ["hb-sjz-cm-v4", "hb-sjz-cu-v4", "hb-sjz-ct-v4"],
  extraCityNodeMeta: {
    "hk-hkg-cm-v4": { province: "香港", city: "香港", carrier: "mobile" }
  }
};
```

- [ ] **Step 2: Write failing parser and discovery tests**

Add to `src/backend/internal/domain/services/ping/catalog_test.go`:

```go
package ping

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseZStaticNodeDataImportsProvinceAndCityTargets(t *testing.T) {
	data, err := os.ReadFile("testdata/zstatic_nodes_data.js")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	meta := zstaticPageMetadata{
		provinceNames: map[string]string{"hb": "河北", "hk": "香港"},
		cityNames:     map[string]string{"sjz": "石家庄", "hkg": "香港"},
	}
	targets, err := parseZStaticNodeData(string(data), meta)
	if err != nil {
		t.Fatalf("parseZStaticNodeData: %v", err)
	}
	if len(targets) != 7 {
		t.Fatalf("expected 7 targets, got %d: %#v", len(targets), targets)
	}
	byID := map[string]ExternalEndpoint{}
	for _, target := range targets {
		byID[target.ID] = target
		if target.Provider != "zstatic_cdn" {
			t.Fatalf("expected zstatic provider, got %#v", target)
		}
		if len(target.Methods) != 1 || target.Methods[0] != MethodTCP {
			t.Fatalf("expected TCP-only target, got %#v", target)
		}
	}
	if byID["zstatic_cdn:he-cm-v4"].Port != 80 || byID["zstatic_cdn:he-cm-v4"].Group != "河北" {
		t.Fatalf("expected province target metadata, got %#v", byID["zstatic_cdn:he-cm-v4"])
	}
	if byID["zstatic_cdn:hb-sjz-cm-v4"].Port != 443 || byID["zstatic_cdn:hb-sjz-cm-v4"].City != "石家庄" {
		t.Fatalf("expected generated city metadata, got %#v", byID["zstatic_cdn:hb-sjz-cm-v4"])
	}
	if byID["zstatic_cdn:hk-hkg-cm-v4"].Region != "香港" || byID["zstatic_cdn:hk-hkg-cm-v4"].City != "香港" {
		t.Fatalf("expected extra city metadata, got %#v", byID["zstatic_cdn:hk-hkg-cm-v4"])
	}
}

func TestRefreshZStaticFollowsEntryRedirectAndResolvesScript(t *testing.T) {
	data, err := os.ReadFile("testdata/zstatic_nodes_data.js")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var sawEntry bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			sawEntry = true
			http.Redirect(w, r, "/landing", http.StatusFound)
		case "/landing":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><body><script>
const provinceNameMap = { hb: "河北", hk: "香港" };
const cityNameMap = { sjz: "石家庄", hkg: "香港" };
</script><script src="assets/nodes_data.js"></script></body></html>`))
		case "/assets/nodes_data.js":
			w.Header().Set("Content-Type", "application/javascript")
			w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider, err := refreshZStaticCatalog(context.Background(), server.Client(), server.URL, time.Now)
	if err != nil {
		t.Fatalf("refreshZStaticCatalog: %v", err)
	}
	if !sawEntry {
		t.Fatal("expected refresher to start from entry URL")
	}
	if provider.ProviderID != "zstatic_cdn" || len(provider.Targets) != 7 {
		t.Fatalf("expected refreshed zstatic provider, got %#v", provider)
	}
}
```

- [ ] **Step 3: Run parser tests and verify red**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui go test ./src/backend/internal/domain/services/ping -run 'TestParseZStaticNodeDataImportsProvinceAndCityTargets|TestRefreshZStaticFollowsEntryRedirectAndResolvesScript' -count=1
```

Expected: FAIL because parser and refresher functions do not exist.

- [ ] **Step 4: Implement ZStatic parser and entry discovery**

Create `src/backend/internal/domain/services/ping/catalog_zstatic.go` with:

```go
package ping

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const zstaticEntryURL = "https://zstaticcdn.com/"

var zstaticCarrierNames = map[string]string{
	"cm":      "China Mobile",
	"mobile":  "China Mobile",
	"cu":      "China Unicom",
	"unicom":  "China Unicom",
	"ct":      "China Telecom",
	"telecom": "China Telecom",
}

var zstaticCarrierLabels = map[string]string{
	"cm":      "移动",
	"mobile":  "移动",
	"cu":      "联通",
	"unicom":  "联通",
	"ct":      "电信",
	"telecom": "电信",
}

var zstaticProvinceCodeNames = map[string]string{
	"hb": "河北", "he": "河北", "sx": "山西", "ln": "辽宁", "jl": "吉林", "hl": "黑龙江",
	"js": "江苏", "zj": "浙江", "ah": "安徽", "fj": "福建", "jx": "江西", "sd": "山东",
	"ha": "河南", "hn": "湖南", "gd": "广东", "hi": "海南", "sc": "四川", "gz": "贵州",
	"yn": "云南", "sn": "陕西", "gs": "甘肃", "qh": "青海", "nm": "内蒙古", "gx": "广西",
	"xz": "西藏", "nx": "宁夏", "xj": "新疆", "bj": "北京", "sh": "上海", "tj": "天津",
	"cq": "重庆", "hk": "香港", "mo": "澳门",
}

var zstaticCityCodeNames = map[string]string{
	"sjz": "石家庄", "hkg": "香港",
}

type zstaticPageMetadata struct {
	provinceNames map[string]string
	cityNames     map[string]string
}

type zstaticNodeData struct {
	ProvinceBaseData []struct {
		Province string            `json:"province"`
		Carriers map[string]string `json:"carriers"`
	} `json:"provinceBaseData"`
	CityKeyList       []string                         `json:"cityKeyList"`
	ExtraCityNodeMeta map[string]zstaticExtraCityMeta  `json:"extraCityNodeMeta"`
}

type zstaticExtraCityMeta struct {
	Province string `json:"province"`
	City     string `json:"city"`
	Carrier  string `json:"carrier"`
}

func refreshZStaticCatalog(ctx context.Context, client *http.Client, entryURL string, now func() time.Time) (ExternalTargetProviderCatalog, error) {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if entryURL == "" {
		entryURL = zstaticEntryURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, entryURL, nil)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	html := string(body)
	meta := parseZStaticPageMetadata(html)
	scriptURL, err := resolveZStaticNodeScriptURL(resp.Request.URL, html)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, scriptURL, nil)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	resp, err = client.Do(req)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	defer resp.Body.Close()
	script, err := io.ReadAll(resp.Body)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	targets, err := parseZStaticNodeData(string(script), meta)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	return ExternalTargetProviderCatalog{
		ProviderID: "zstatic_cdn", ProviderName: "ZStaticCDN", UpdatedAt: now().Unix(), Targets: targets,
	}, nil
}

func resolveZStaticNodeScriptURL(base *url.URL, html string) (string, error) {
	re := regexp.MustCompile(`<script[^>]+src=["']([^"']*nodes_data\.js[^"']*)["']`)
	match := re.FindStringSubmatch(html)
	if len(match) != 2 {
		return "", fmt.Errorf("zstatic nodes_data.js script not found")
	}
	ref, err := url.Parse(match[1])
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

func parseZStaticPageMetadata(html string) zstaticPageMetadata {
	return zstaticPageMetadata{
		provinceNames: parseZStaticStringMap(html, "provinceNameMap"),
		cityNames:     parseZStaticStringMap(html, "cityNameMap"),
	}
}

func parseZStaticStringMap(html string, name string) map[string]string {
	re := regexp.MustCompile(`(?s)const\s+` + regexp.QuoteMeta(name) + `\s*=\s*\{(.*?)\};`)
	match := re.FindStringSubmatch(html)
	if len(match) != 2 {
		return map[string]string{}
	}
	itemRe := regexp.MustCompile(`([a-z0-9]+)\s*:\s*"([^"]+)"`)
	out := map[string]string{}
	for _, item := range itemRe.FindAllStringSubmatch(match[1], -1) {
		out[item[1]] = item[2]
	}
	return out
}

func parseZStaticNodeData(script string, meta zstaticPageMetadata) ([]ExternalEndpoint, error) {
	start := strings.Index(script, "{")
	end := strings.LastIndex(script, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("zstatic node data object not found")
	}
	var data zstaticNodeData
	if err := json.Unmarshal([]byte(script[start:end+1]), &data); err != nil {
		return nil, err
	}
	targets := make([]ExternalEndpoint, 0)
	for _, province := range data.ProvinceBaseData {
		for carrier, endpoint := range province.Carriers {
			target, err := zstaticEndpointFromHost(endpoint, province.Province, "", carrier, "province")
			if err != nil {
				return nil, err
			}
			targets = append(targets, target)
		}
	}
	for _, key := range data.CityKeyList {
		targets = append(targets, zstaticCityEndpoint(key, meta, data.ExtraCityNodeMeta[key]))
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
	return targets, nil
}

func zstaticEndpointFromHost(endpoint string, region string, city string, carrier string, level string) (ExternalEndpoint, error) {
	host, portText, ok := strings.Cut(endpoint, ":")
	if !ok {
		return ExternalEndpoint{}, fmt.Errorf("invalid zstatic endpoint %q", endpoint)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return ExternalEndpoint{}, err
	}
	key := strings.TrimSuffix(host, ".ip.zstaticcdn.com")
	carrierCode := normalizeZStaticCarrier(carrier)
	labelPlace := region
	group := region
	if city != "" {
		labelPlace = city
		group = region + " / " + city
	}
	return ExternalEndpoint{
		ID: "zstatic_cdn:" + key, Label: labelPlace + zstaticCarrierLabels[carrierCode], Provider: "zstatic_cdn",
		Region: region, City: city, Country: "CN", Network: zstaticCarrierNames[carrierCode], Group: group, Level: level,
		Host: host, Port: port, Methods: []string{MethodTCP},
	}, nil
}

func zstaticCityEndpoint(key string, pageMeta zstaticPageMetadata, extra zstaticExtraCityMeta) ExternalEndpoint {
	parts := strings.Split(key, "-")
	region := extra.Province
	city := extra.City
	carrier := extra.Carrier
	if len(parts) >= 4 {
		if region == "" {
			region = pageMeta.provinceNames[parts[0]]
			if region == "" {
				region = zstaticProvinceCodeNames[parts[0]]
			}
		}
		if city == "" {
			city = pageMeta.cityNames[parts[1]]
			if city == "" {
				city = zstaticCityCodeNames[parts[1]]
			}
		}
		if carrier == "" {
			carrier = parts[2]
		}
	}
	if region == "" {
		region = strings.ToUpper(parts[0])
	}
	if city == "" && len(parts) > 1 {
		city = strings.ToUpper(parts[1])
	}
	carrierCode := normalizeZStaticCarrier(carrier)
	group := region
	if city != "" {
		group = region + " / " + city
	}
	return ExternalEndpoint{
		ID: "zstatic_cdn:" + key, Label: city + zstaticCarrierLabels[carrierCode], Provider: "zstatic_cdn",
		Region: region, City: city, Country: "CN", Network: zstaticCarrierNames[carrierCode], Group: group, Level: "city",
		Host: key + ".ip.zstaticcdn.com", Port: 443, Methods: []string{MethodTCP},
	}
}

func normalizeZStaticCarrier(carrier string) string {
	switch strings.ToLower(carrier) {
	case "mobile", "cm":
		return "cm"
	case "unicom", "cu":
		return "cu"
	case "telecom", "ct":
		return "ct"
	default:
		return strings.ToLower(carrier)
	}
}
```

- [ ] **Step 5: Write failing Linode refresher test**

Add to `catalog_test.go`:

```go
func TestRefreshLinodeCatalogUsesConfiguredSpeedtestHosts(t *testing.T) {
	provider := refreshLinodeCatalog(time.Unix(1710000000, 0))
	if provider.ProviderID != "linode_speedtest" {
		t.Fatalf("expected linode provider, got %#v", provider)
	}
	if len(provider.Targets) < 10 {
		t.Fatalf("expected full linode target list, got %d", len(provider.Targets))
	}
	var foundAtlanta bool
	for _, target := range provider.Targets {
		if target.ID == "linode_speedtest:atlanta" {
			foundAtlanta = true
		}
		if target.Provider != "linode_speedtest" || target.Group == "" || target.Port != 80 {
			t.Fatalf("expected normalized linode target, got %#v", target)
		}
	}
	if !foundAtlanta {
		t.Fatal("expected Atlanta speedtest target")
	}
}
```

- [ ] **Step 6: Run Linode test and verify red**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui go test ./src/backend/internal/domain/services/ping -run TestRefreshLinodeCatalogUsesConfiguredSpeedtestHosts -count=1
```

Expected: FAIL because `refreshLinodeCatalog` does not exist.

- [ ] **Step 7: Implement Linode static refresher**

Create `src/backend/internal/domain/services/ping/catalog_linode.go`:

```go
package ping

import (
	"strings"
	"time"
)

type linodeSpeedtestHost struct {
	ID      string
	Label   string
	Region  string
	Country string
	Group   string
	Host    string
}

var linodeSpeedtestHosts = []linodeSpeedtestHost{
	{"newark", "Linode Newark", "Newark", "US", "United States", "speedtest.newark.linode.com"},
	{"atlanta", "Linode Atlanta", "Atlanta", "US", "United States", "speedtest.atlanta.linode.com"},
	{"dallas", "Linode Dallas", "Dallas", "US", "United States", "speedtest.dallas.linode.com"},
	{"fremont", "Linode Fremont", "Fremont", "US", "United States", "speedtest.fremont.linode.com"},
	{"frankfurt", "Linode Frankfurt", "Frankfurt", "DE", "Europe", "speedtest.frankfurt.linode.com"},
	{"london", "Linode London", "London", "GB", "Europe", "speedtest.london.linode.com"},
	{"singapore", "Linode Singapore", "Singapore", "SG", "Asia Pacific", "speedtest.singapore.linode.com"},
	{"tokyo", "Linode Tokyo", "Tokyo", "JP", "Asia Pacific", "speedtest.tokyo2.linode.com"},
	{"sydney", "Linode Sydney", "Sydney", "AU", "Asia Pacific", "speedtest.syd1.linode.com"},
	{"mumbai", "Linode Mumbai", "Mumbai", "IN", "Asia Pacific", "speedtest.mumbai1.linode.com"},
	{"toronto", "Linode Toronto", "Toronto", "CA", "Canada", "speedtest.toronto1.linode.com"},
}

func refreshLinodeCatalog(now time.Time) ExternalTargetProviderCatalog {
	targets := make([]ExternalEndpoint, 0, len(linodeSpeedtestHosts))
	for _, item := range linodeSpeedtestHosts {
		id := strings.TrimSpace(item.ID)
		targets = append(targets, ExternalEndpoint{
			ID: "linode_speedtest:" + id, Label: item.Label, Provider: "linode_speedtest",
			Region: item.Region, Country: item.Country, Group: item.Group,
			Host: item.Host, Port: 80, Methods: []string{MethodTCP, MethodHTTP, MethodICMP},
		})
	}
	return ExternalTargetProviderCatalog{
		ProviderID: "linode_speedtest", ProviderName: "Linode/Akamai Speed Test", UpdatedAt: now.Unix(), Targets: targets,
	}
}
```

- [ ] **Step 8: Add catalog service orchestration test**

Add to `catalog_test.go`:

```go
func TestTargetCatalogRefreshPreservesStaticProviders(t *testing.T) {
	store := newStoreWithDir(t.TempDir())
	svc := NewTargetCatalogService(store)
	svc.now = func() time.Time { return time.Unix(1710000000, 0) }
	svc.refreshZStatic = func(ctx context.Context, client *http.Client, entryURL string, now func() time.Time) (ExternalTargetProviderCatalog, error) {
		return ExternalTargetProviderCatalog{
			ProviderID: "zstatic_cdn", ProviderName: "ZStaticCDN", UpdatedAt: now().Unix(),
			Targets: []ExternalEndpoint{{ID: "zstatic_cdn:test", Label: "Test", Provider: "zstatic_cdn", Group: "Test", Host: "test.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}}},
		}, nil
	}

	catalog, err := svc.Refresh(context.Background(), nil)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if len(catalog.TargetsForProvider("zstatic_cdn")) != 1 {
		t.Fatalf("expected refreshed zstatic targets, got %#v", catalog.TargetsForProvider("zstatic_cdn"))
	}
	if len(catalog.TargetsForProvider("public_dns")) == 0 {
		t.Fatal("expected static public_dns targets to be preserved")
	}
	loaded, err := store.LoadExternalTargetCatalog()
	if err != nil {
		t.Fatalf("LoadExternalTargetCatalog: %v", err)
	}
	if loaded.UpdatedAt != 1710000000 {
		t.Fatalf("expected persisted updated_at, got %d", loaded.UpdatedAt)
	}
}
```

- [ ] **Step 9: Run service test and verify red**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui go test ./src/backend/internal/domain/services/ping -run TestTargetCatalogRefreshPreservesStaticProviders -count=1
```

Expected: FAIL because `TargetCatalogService` does not exist.

- [ ] **Step 10: Implement catalog service**

Create `src/backend/internal/domain/services/ping/catalog_refresh.go`:

```go
package ping

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type TargetCatalogService struct {
	store          *Store
	httpClient     *http.Client
	now            func() time.Time
	refreshZStatic func(context.Context, *http.Client, string, func() time.Time) (ExternalTargetProviderCatalog, error)
}

func NewTargetCatalogService(store *Store) *TargetCatalogService {
	if store == nil {
		store = NewStore()
	}
	return &TargetCatalogService{
		store: store,
		httpClient: &http.Client{Timeout: 20 * time.Second},
		now: time.Now,
		refreshZStatic: refreshZStaticCatalog,
	}
}

func (s *TargetCatalogService) Load() (*ExternalTargetCatalog, error) {
	if catalog, err := s.store.LoadExternalTargetCatalog(); err == nil {
		return catalog, nil
	}
	return loadSeedExternalTargetCatalog()
}

func (s *TargetCatalogService) Refresh(ctx context.Context, providerIDs []string) (*ExternalTargetCatalog, error) {
	base, err := s.Load()
	if err != nil {
		return nil, err
	}
	selected := providerSelection(providerIDs)
	refreshed := map[string]ExternalTargetProviderCatalog{}
	now := s.now()
	if shouldRefreshProvider(selected, "zstatic_cdn") {
		provider, err := s.refreshZStatic(ctx, s.httpClient, zstaticEntryURL, s.now)
		if err != nil {
			return nil, err
		}
		refreshed[provider.ProviderID] = provider
	}
	if shouldRefreshProvider(selected, "linode_speedtest") {
		provider := refreshLinodeCatalog(now)
		refreshed[provider.ProviderID] = provider
	}
	next := &ExternalTargetCatalog{UpdatedAt: now.Unix(), Providers: make([]ExternalTargetProviderCatalog, 0, len(base.Providers))}
	seen := map[string]bool{}
	for _, provider := range base.Providers {
		if replacement, ok := refreshed[provider.ProviderID]; ok {
			next.Providers = append(next.Providers, replacement)
			seen[provider.ProviderID] = true
			continue
		}
		next.Providers = append(next.Providers, provider)
		seen[provider.ProviderID] = true
	}
	for _, provider := range refreshed {
		if !seen[provider.ProviderID] {
			next.Providers = append(next.Providers, provider)
		}
	}
	if err := s.store.SaveExternalTargetCatalog(next); err != nil {
		return nil, err
	}
	return next, nil
}

func providerSelection(providerIDs []string) map[string]bool {
	if len(providerIDs) == 0 {
		return nil
	}
	selected := map[string]bool{}
	for _, id := range providerIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			selected[id] = true
		}
	}
	return selected
}

func shouldRefreshProvider(selected map[string]bool, providerID string) bool {
	return len(selected) == 0 || selected[providerID]
}

func filterCatalogTargetsByID(catalog *ExternalTargetCatalog, providerIDs []string, targetIDs []string) ([]ExternalEndpoint, error) {
	if len(targetIDs) == 0 {
		return nil, fmt.Errorf("outbound target_node_ids is required")
	}
	providers := providerSelection(providerIDs)
	selectedTargets := map[string]bool{}
	for _, id := range targetIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			selectedTargets[id] = true
		}
	}
	targets := make([]ExternalEndpoint, 0, len(selectedTargets))
	for _, provider := range catalog.Providers {
		if len(providers) > 0 && !providers[provider.ProviderID] {
			continue
		}
		for _, target := range provider.Targets {
			if selectedTargets[target.ID] {
				targets = append(targets, target)
			}
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no selected outbound targets matched the target catalog")
	}
	return targets, nil
}

func catalogTargetsForProviders(catalog *ExternalTargetCatalog, providerIDs []string) []ExternalEndpoint {
	providers := providerSelection(providerIDs)
	targets := make([]ExternalEndpoint, 0)
	for _, provider := range catalog.Providers {
		if len(providers) > 0 && !providers[provider.ProviderID] {
			continue
		}
		targets = append(targets, provider.Targets...)
	}
	return targets
}
```

- [ ] **Step 11: Run backend catalog tests and commit**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui go test ./src/backend/internal/domain/services/ping -run 'TestParseZStaticNodeDataImportsProvinceAndCityTargets|TestRefreshZStaticFollowsEntryRedirectAndResolvesScript|TestRefreshLinodeCatalogUsesConfiguredSpeedtestHosts|TestTargetCatalogRefreshPreservesStaticProviders' -count=1
```

Expected: PASS.

Commit:

```bash
git -c safe.directory=/home/bean/workspace/bproject/b-ui add src/backend/internal/domain/services/ping
git -c safe.directory=/home/bean/workspace/bproject/b-ui commit -m "feat: refresh external target catalog"
```

## Task 3: Outbound Filtering, API, and CLI Refresh Command

**Files:**
- Modify: `src/backend/internal/domain/services/ping/external.go`
- Modify: `src/backend/internal/domain/services/ping/external_test.go`
- Modify: `src/backend/internal/http/api/ping.go`
- Modify: `src/backend/internal/http/api/ping_test.go`
- Modify: `src/backend/internal/cli/cmd.go`
- Create: `src/backend/internal/cli/external_targets.go`
- Create: `scripts/dev/refresh-external-targets.sh`

- [ ] **Step 1: Replace old outbound ignore tests with selected target tests**

In `src/backend/internal/domain/services/ping/external_test.go`, replace `TestRunOutboundIgnoresDeprecatedTargetNodeIDs` and `TestRunExternalOutboundIgnoresTargetNodeIDs` with:

```go
func TestRunOutboundFiltersSelectedTargetNodeIDs(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	var probed []string
	svc.probeEndpoint = func(ctx context.Context, endpoint ExternalEndpoint, methods []string) (string, float64, error) {
		probed = append(probed, endpoint.ID)
		return MethodTCP, 9, nil
	}

	data, err := svc.Run(context.Background(), ExternalRunRequest{
		SourceIDs:     []string{"public_dns"},
		TargetNodeIDs: []string{"public_dns:cloudflare-dns", "public_dns:quad9-dns"},
		Direction:     DirectionOutbound,
		Methods:       []string{MethodTCP},
	}, nil)
	if err != nil {
		t.Fatalf("Run outbound: %v", err)
	}
	if len(data.Results) != 2 {
		t.Fatalf("expected two selected target results, got %d", len(data.Results))
	}
	if strings.Join(probed, ",") != "public_dns:cloudflare-dns,public_dns:quad9-dns" {
		t.Fatalf("unexpected probed targets: %v", probed)
	}
}

func TestRunOutboundRequiresSelectedTargetNodeIDs(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))

	_, err := svc.Run(context.Background(), ExternalRunRequest{
		SourceIDs: []string{"public_dns"},
		Direction: DirectionOutbound,
		Methods:   []string{MethodTCP},
	}, nil)

	if err == nil {
		t.Fatal("expected missing selected outbound targets to fail")
	}
	if !strings.Contains(err.Error(), "target_node_ids") {
		t.Fatalf("expected target_node_ids error, got %v", err)
	}
}
```

Also update `TestRunOutboundUsesCurrentNodeOnceWithoutMembers` so the request includes one selected target and expects one result:

```go
data, err := svc.Run(context.Background(), ExternalRunRequest{
	Direction:     DirectionOutbound,
	SourceIDs:     []string{"public_dns"},
	TargetNodeIDs: []string{"public_dns:cloudflare-dns"},
	Methods:       []string{MethodTCP},
}, nil)
if err != nil {
	t.Fatalf("Run outbound: %v", err)
}
if len(data.Results) != 1 {
	t.Fatalf("expected one selected outbound result, got %d", len(data.Results))
}
```

- [ ] **Step 2: Run outbound tests and verify red**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui go test ./src/backend/internal/domain/services/ping -run 'TestRunOutboundFiltersSelectedTargetNodeIDs|TestRunOutboundRequiresSelectedTargetNodeIDs' -count=1
```

Expected: FAIL because `Run` still ignores `TargetNodeIDs`.

- [ ] **Step 3: Implement outbound filtering**

Modify `src/backend/internal/domain/services/ping/external.go`:

```go
func (s *ExternalService) RunOutbound(ctx context.Context, sourceIDs []string, members []MeshMember) (*ExternalResultData, error) {
	return s.runOutboundWithMethods(ctx, sourceIDs, nil, nil)
}

func (s *ExternalService) runOutboundWithMethods(ctx context.Context, sourceIDs []string, methods []string, targetNodeIDs []string) (*ExternalResultData, error) {
	config := s.store.LoadExternalConfigOrDefault()
	enabledSources := enabledExternalSources(sourceIDs, config, DirectionOutbound)
	source := currentPanelEndpoint()
	catalog, err := NewTargetCatalogService(s.store).Load()
	if err != nil {
		return nil, err
	}
	var targets []ExternalEndpoint
	if len(targetNodeIDs) == 0 {
		targets = catalogTargetsForProviders(catalog, sourceIDs)
	} else {
		targets, err = filterCatalogTargetsByID(catalog, sourceIDs, targetNodeIDs)
		if err != nil {
			return nil, err
		}
	}
	targetsByProvider := map[string][]ExternalEndpoint{}
	for _, target := range targets {
		targetsByProvider[target.Provider] = append(targetsByProvider[target.Provider], target)
	}
	tasks := make([]func() ExternalTestResult, 0)
	for _, src := range enabledSources {
		providerTargets := targetsByProvider[src.ID]
		if len(providerTargets) == 0 {
			continue
		}
		for _, target := range providerTargets {
			target := target
			tasks = append(tasks, func() ExternalTestResult {
				return s.probeOutboundTarget(ctx, source, target, methods)
			})
		}
	}
	results := runExternalProbeTasks(DefaultExternalMaxConcurrent, tasks)
	data := &ExternalResultData{TestedAt: nowUnix(), Results: results}
	if err := s.store.SaveExternalResults(data); err != nil {
		return nil, err
	}
	return data, nil
}
```

Also change `Run` outbound call:

```go
if len(req.TargetNodeIDs) == 0 {
	return nil, fmt.Errorf("outbound target_node_ids is required")
}
outData, err := s.runOutboundWithMethods(ctx, enabledOut, req.Methods, req.TargetNodeIDs)
```

- [ ] **Step 4: Run outbound tests and verify green**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui go test ./src/backend/internal/domain/services/ping -run 'TestRunOutboundFiltersSelectedTargetNodeIDs|TestRunOutboundRequiresSelectedTargetNodeIDs|TestRunOutboundUsesCurrentNodeOnceWithoutMembers' -count=1
```

Expected: PASS.

- [ ] **Step 5: Add API tests for catalog endpoints**

Add `path/filepath` to the imports in `src/backend/internal/http/api/ping_test.go`, then add a dedicated catalog test store and router:

```go
func newPingCatalogStore(t *testing.T) *ping.Store {
	t.Helper()
	return ping.NewStoreWithDataDir(filepath.Join(t.TempDir(), ping.DataDir))
}

func newPingCatalogTestRouter(store *ping.Store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger.InitLogger(logging.ERROR)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(sessions.Sessions("b-ui", cookie.NewStore([]byte("test-secret"))))
	handler := &pingAPIHandler{store: store}
	router.GET("/api/ping/external/targets", handler.getExternalTargets)
	router.POST("/api/ping/external/targets/refresh", handler.refreshExternalTargets)
	router.GET("/__test/login/:username", func(c *gin.Context) {
		if err := SetLoginUser(c, c.Param("username"), 0); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})
	return router
}
```

Add tests:

```go
func TestGetExternalTargetsReturnsSeedCatalog(t *testing.T) {
	store := newPingCatalogStore(t)
	router := newPingCatalogTestRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/ping/external/targets", nil)
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	if !response.Success || response.Obj == nil {
		t.Fatalf("expected target catalog response, got %#v", response)
	}
}

func TestRefreshExternalTargetsAcceptsProviderList(t *testing.T) {
	store := newPingCatalogStore(t)
	router := newPingCatalogTestRouter(store)
	req := httptest.NewRequest(http.MethodPost, "/api/ping/external/targets/refresh", bytes.NewBufferString(`{"provider_ids":["linode_speedtest"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	if !response.Success {
		t.Fatalf("expected refresh success, got %#v", response)
	}
}
```

- [ ] **Step 6: Run API tests and verify red**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui go test ./src/backend/internal/http/api -run 'TestGetExternalTargetsReturnsSeedCatalog|TestRefreshExternalTargetsAcceptsProviderList' -count=1
```

Expected: FAIL because handlers and routes do not exist.

- [ ] **Step 7: Implement target catalog API**

In `src/backend/internal/http/api/ping.go`, add routes:

```go
g.GET("/ping/external/targets", h.getExternalTargets)
g.POST("/ping/external/targets/refresh", h.refreshExternalTargets)
```

Add request type and handlers:

```go
type refreshExternalTargetsRequest struct {
	ProviderIDs []string `json:"provider_ids"`
}

func (h *pingAPIHandler) getExternalTargets(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	catalog, err := ping.NewTargetCatalogService(h.store).Load()
	if err != nil {
		jsonMsg(c, "external target catalog", err)
		return
	}
	jsonObj(c, catalog, nil)
}

func (h *pingAPIHandler) refreshExternalTargets(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	var req refreshExternalTargetsRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "invalid request: " + err.Error()})
			return
		}
	}
	catalog, err := ping.NewTargetCatalogService(h.store).Refresh(c.Request.Context(), req.ProviderIDs)
	if err != nil {
		jsonMsg(c, "refresh external target catalog", err)
		return
	}
	jsonObj(c, catalog, nil)
}
```

- [ ] **Step 8: Add CLI command and wrapper**

Modify usage in `src/backend/internal/cli/cmd.go`:

```go
fmt.Println("    external-targets refresh outbound ping target catalog")
```

Add switch case:

```go
case "external-targets":
	refreshExternalTargetsCmd(os.Args[2:])
```

Create `src/backend/internal/cli/external_targets.go`:

```go
package cmd

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/ping"
)

func refreshExternalTargetsCmd(args []string) {
	cmd := flag.NewFlagSet("external-targets", flag.ExitOnError)
	var providers string
	cmd.StringVar(&providers, "providers", "", "comma-separated providers to refresh")
	if err := cmd.Parse(args); err != nil {
		fmt.Println(err)
		return
	}
	providerIDs := splitProviderIDs(providers)
	catalog, err := ping.NewTargetCatalogService(ping.NewStore()).Refresh(context.Background(), providerIDs)
	if err != nil {
		fmt.Println("refresh external targets failed:", err)
		return
	}
	fmt.Printf("refreshed external targets: %d providers\n", len(catalog.Providers))
}

func splitProviderIDs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
```

Create `scripts/dev/refresh-external-targets.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

go run ./src/backend/cmd/b-ui external-targets "$@"
```

Make executable:

```bash
chmod +x scripts/dev/refresh-external-targets.sh
```

- [ ] **Step 9: Run API/backend tests and commit**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui go test ./src/backend/internal/domain/services/ping ./src/backend/internal/http/api ./src/backend/internal/cli -run 'TestRunOutboundFiltersSelectedTargetNodeIDs|TestRunOutboundRequiresSelectedTargetNodeIDs|TestGetExternalTargetsReturnsSeedCatalog|TestRefreshExternalTargetsAcceptsProviderList|TestRefreshLinodeCatalogUsesConfiguredSpeedtestHosts' -count=1
```

Expected: PASS.

Commit:

```bash
git -c safe.directory=/home/bean/workspace/bproject/b-ui add src/backend/internal/domain/services/ping src/backend/internal/http/api src/backend/internal/cli scripts/dev/refresh-external-targets.sh
git -c safe.directory=/home/bean/workspace/bproject/b-ui commit -m "feat: select outbound ping targets"
```

## Task 4: Frontend Store and Types

**Files:**
- Modify: `src/frontend/src/types/ping.ts`
- Modify: `src/frontend/src/store/modules/ping.ts`
- Modify: `src/frontend/src/store/modules/ping.test.ts`

- [ ] **Step 1: Write failing store tests**

Add to `src/frontend/src/store/modules/ping.test.ts`:

```ts
  it('loads external target catalog', async () => {
    const catalog = {
      updated_at: 1710000000,
      providers: [
        {
          provider_id: 'public_dns',
          provider_name: 'Public DNS',
          static: true,
          targets: [
            { id: 'public_dns:cloudflare-dns', label: 'Cloudflare DNS', provider: 'public_dns', group: 'Global', host: '1.1.1.1', port: 53, methods: ['tcp'] },
          ],
        },
      ],
    }
    vi.mocked(api.get).mockResolvedValueOnce({ data: { success: true, obj: catalog } })

    const { usePingStore } = await import('./ping')
    const store = usePingStore()

    await expect(store.loadExternalTargetCatalog()).resolves.toEqual(catalog)
    expect(store.externalTargetCatalog).toEqual(catalog)
    expect(api.get).toHaveBeenCalledWith('api/ping/external/targets')
  })

  it('refreshes selected external target providers', async () => {
    const catalog = { updated_at: 1710000001, providers: [] }
    vi.mocked(api.post).mockResolvedValueOnce({ data: { success: true, obj: catalog } })

    const { usePingStore } = await import('./ping')
    const store = usePingStore()

    await expect(store.refreshExternalTargetCatalog(['zstatic_cdn'])).resolves.toEqual(catalog)
    expect(store.externalTargetCatalog).toEqual(catalog)
    expect(api.post).toHaveBeenCalledWith(
      'api/ping/external/targets/refresh',
      { provider_ids: ['zstatic_cdn'] },
      { headers: { 'Content-Type': 'application/json' } },
    )
  })
```

- [ ] **Step 2: Run store tests and verify red**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui bash -lc "cd src/frontend && npm test -- src/store/modules/ping.test.ts"
```

Expected: FAIL because store actions and types do not exist.

- [ ] **Step 3: Add frontend catalog types**

Modify `src/frontend/src/types/ping.ts`:

```ts
export interface ExternalEndpoint {
  id: string
  label: string
  provider: string
  region?: string
  country?: string
  city?: string
  network?: string
  group?: string
  level?: string
  host?: string
  port?: number
  methods?: string[]
}

export interface ExternalTargetProviderCatalog {
  provider_id: string
  provider_name: string
  static?: boolean
  updated_at?: number
  targets: ExternalEndpoint[]
}

export interface ExternalTargetCatalog {
  updated_at: number
  providers: ExternalTargetProviderCatalog[]
}
```

- [ ] **Step 4: Add store state and actions**

Modify imports and store in `src/frontend/src/store/modules/ping.ts`:

```ts
import type { ExternalTargetCatalog } from '@/types/ping'
```

Add state:

```ts
const externalTargetCatalog = ref<ExternalTargetCatalog | null>(null)
```

Add actions:

```ts
async function loadExternalTargetCatalog(): Promise<ExternalTargetCatalog> {
  error.value = null
  try {
    const { data } = await api.get('api/ping/external/targets')
    if (data.success) {
      externalTargetCatalog.value = data.obj
      return data.obj
    }
    throw new Error(data.msg)
  } catch (e: any) {
    error.value = e.message
    throw e
  }
}

async function refreshExternalTargetCatalog(providerIds: string[] = []): Promise<ExternalTargetCatalog> {
  loading.value = true
  error.value = null
  try {
    const { data } = await api.post(
      'api/ping/external/targets/refresh',
      { provider_ids: providerIds },
      { headers: { 'Content-Type': 'application/json' } },
    )
    if (data.success) {
      externalTargetCatalog.value = data.obj
      return data.obj
    }
    throw new Error(data.msg)
  } catch (e: any) {
    error.value = e.message
    throw e
  } finally {
    loading.value = false
  }
}
```

Return them:

```ts
externalTargetCatalog, loadExternalTargetCatalog, refreshExternalTargetCatalog,
```

- [ ] **Step 5: Run frontend store tests and commit**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui bash -lc "cd src/frontend && npm test -- src/store/modules/ping.test.ts"
```

Expected: PASS.

Commit:

```bash
git -c safe.directory=/home/bean/workspace/bproject/b-ui add src/frontend/src/types/ping.ts src/frontend/src/store/modules/ping.ts src/frontend/src/store/modules/ping.test.ts
git -c safe.directory=/home/bean/workspace/bproject/b-ui commit -m "feat: load external target catalog in frontend"
```

## Task 5: Multi-Location Ping Target Picker UI

**Files:**
- Modify: `src/frontend/src/views/MultiLocationPing.vue`
- Modify: `src/frontend/src/views/MultiLocationPing.test.ts`

- [ ] **Step 1: Update view source tests for desired behavior**

Modify `src/frontend/src/views/MultiLocationPing.test.ts`:

```ts
  it('sends selected outbound target node ids', () => {
    const source = readSource()

    expect(source).toContain('selectedOutboundTargetIds')
    expect(source).toContain('target_node_ids: selectedOutboundTargetIds.value')
    expect(source).not.toContain('sends outbound current-node requests without target node ids')
  })

  it('loads and refreshes the outbound target catalog', () => {
    const source = readSource()

    expect(source).toContain('loadExternalTargetCatalog')
    expect(source).toContain('refreshExternalTargetCatalog')
    expect(source).toContain('refreshOutboundTargets')
  })

  it('groups outbound targets by provider and target group', () => {
    const source = readSource()

    expect(source).toContain('outboundProviderGroups')
    expect(source).toContain('targetGroups')
    expect(source).toContain('toggleProviderTargets')
    expect(source).toContain('toggleTargetGroup')
  })
```

Remove the old test that asserts `target_node_ids` is absent.

- [ ] **Step 2: Run view test and verify red**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui bash -lc "cd src/frontend && npm test -- src/views/MultiLocationPing.test.ts"
```

Expected: FAIL because the view still has the provider table and no target picker state.

- [ ] **Step 3: Load catalog on mount**

Modify `onMounted` in `src/frontend/src/views/MultiLocationPing.vue`:

```ts
onMounted(async () => {
  await store.loadExternalConfig()
  await store.loadExternalTargetCatalog()
  try {
    const { data } = await (await import('axios')).default.get('/api/cluster/domains')
    if (data.success) {
      domainOptions.value = (data.obj ?? []).map((d: any) => ({
        title: d.domain,
        value: d.domain,
      }))
    }
  } catch { /* ignore */ }
})
```

- [ ] **Step 4: Add target selection state and computed groups**

Add after inbound port refs:

```ts
const selectedOutboundTargetIds = ref<string[]>([])
const expandedProviders = ref<string[]>([])
const expandedTargetGroups = ref<string[]>([])

type OutboundTargetGroup = {
  key: string
  label: string
  targets: ExternalEndpoint[]
}

type OutboundProviderGroup = {
  id: string
  name: string
  enabled: boolean
  targetGroups: OutboundTargetGroup[]
  targets: ExternalEndpoint[]
}

const outboundProviderGroups = computed<OutboundProviderGroup[]>(() => {
  const enabledByID = new Map(store.outboundSources.map(source => [source.id, source.enabled]))
  return (store.externalTargetCatalog?.providers ?? []).map(provider => {
    const groups = new Map<string, ExternalEndpoint[]>()
    for (const target of provider.targets ?? []) {
      const key = target.group?.trim() || target.region?.trim() || target.city?.trim() || 'Other'
      if (!groups.has(key)) groups.set(key, [])
      groups.get(key)!.push(target)
    }
    const targetGroups = [...groups.entries()]
      .map(([key, targets]) => ({
        key: `${provider.provider_id}:${key}`,
        label: key,
        targets: [...targets].sort((a, b) => endpointLabel(a, a.id).localeCompare(endpointLabel(b, b.id))),
      }))
      .sort((a, b) => a.label.localeCompare(b.label))
    return {
      id: provider.provider_id,
      name: provider.provider_name,
      enabled: enabledByID.get(provider.provider_id) ?? true,
      targetGroups,
      targets: provider.targets ?? [],
    }
  }).filter(provider => provider.targets.length > 0)
})

const selectedOutboundTargetSet = computed(() => new Set(selectedOutboundTargetIds.value))
const selectedOutboundProviderIds = computed(() => {
  const providers = new Set<string>()
  for (const group of outboundProviderGroups.value) {
    if (group.targets.some(target => selectedOutboundTargetSet.value.has(target.id))) {
      providers.add(group.id)
    }
  }
  return [...providers]
})
```

- [ ] **Step 5: Add picker helpers**

Add helper functions:

```ts
function setSelectedTargets(targets: ExternalEndpoint[], selected: boolean) {
  const next = new Set(selectedOutboundTargetIds.value)
  for (const target of targets) {
    if (selected) next.add(target.id)
    else next.delete(target.id)
  }
  selectedOutboundTargetIds.value = [...next]
}

function areAllTargetsSelected(targets: ExternalEndpoint[]) {
  return targets.length > 0 && targets.every(target => selectedOutboundTargetSet.value.has(target.id))
}

function areSomeTargetsSelected(targets: ExternalEndpoint[]) {
  return targets.some(target => selectedOutboundTargetSet.value.has(target.id))
}

function toggleProviderTargets(provider: OutboundProviderGroup) {
  setSelectedTargets(provider.targets, !areAllTargetsSelected(provider.targets))
}

function toggleTargetGroup(group: OutboundTargetGroup) {
  setSelectedTargets(group.targets, !areAllTargetsSelected(group.targets))
}

async function refreshOutboundTargets() {
  const previous = new Set(selectedOutboundTargetIds.value)
  const catalog = await store.refreshExternalTargetCatalog([])
  const available = new Set(catalog.providers.flatMap(provider => provider.targets.map(target => target.id)))
  selectedOutboundTargetIds.value = [...previous].filter(id => available.has(id))
}
```

- [ ] **Step 6: Change outbound run request**

Replace `runOutbound` with:

```ts
async function runOutbound() {
  const ids = selectedOutboundProviderIds.value
  if (ids.length === 0 || selectedOutboundTargetIds.value.length === 0) return
  await store.triggerExternalPing({
    direction: 'outbound',
    source_ids: ids,
    target_node_ids: selectedOutboundTargetIds.value,
    methods: ['tcp', 'http', 'icmp'],
  })
}
```

- [ ] **Step 7: Replace outbound provider table with grouped picker**

Replace the outbound tab body in the template with:

```vue
<div class="multi-location-ping__source-body">
  <div class="multi-location-ping__catalog-actions">
    <v-btn variant="tonal" color="primary" :loading="store.loading" @click="refreshOutboundTargets">
      Update Target Data
    </v-btn>
    <span class="multi-location-ping__selection-count">{{ selectedOutboundTargetIds.length }} selected</span>
  </div>

  <v-expansion-panels v-model="expandedProviders" multiple variant="accordion" class="multi-location-ping__target-providers">
    <v-expansion-panel v-for="provider in outboundProviderGroups" :key="provider.id" :value="provider.id">
      <v-expansion-panel-title>
        <div class="multi-location-ping__provider-title">
          <v-checkbox-btn
            :model-value="areAllTargetsSelected(provider.targets)"
            :indeterminate="areSomeTargetsSelected(provider.targets) && !areAllTargetsSelected(provider.targets)"
            @click.stop="toggleProviderTargets(provider)"
          />
          <span>{{ provider.name }}</span>
          <v-chip size="small" variant="tonal">{{ provider.targets.length }}</v-chip>
        </div>
      </v-expansion-panel-title>
      <v-expansion-panel-text>
        <v-expansion-panels v-model="expandedTargetGroups" multiple variant="accordion">
          <v-expansion-panel v-for="group in provider.targetGroups" :key="group.key" :value="group.key">
            <v-expansion-panel-title>
              <div class="multi-location-ping__group-title">
                <v-checkbox-btn
                  :model-value="areAllTargetsSelected(group.targets)"
                  :indeterminate="areSomeTargetsSelected(group.targets) && !areAllTargetsSelected(group.targets)"
                  @click.stop="toggleTargetGroup(group)"
                />
                <span>{{ group.label }}</span>
                <v-chip size="x-small" variant="tonal">{{ group.targets.length }}</v-chip>
              </div>
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <div class="multi-location-ping__target-grid">
                <v-checkbox
                  v-for="target in group.targets"
                  :key="target.id"
                  v-model="selectedOutboundTargetIds"
                  :value="target.id"
                  density="compact"
                  hide-details
                >
                  <template #label>
                    <span class="multi-location-ping__target-label">
                      <strong>{{ endpointLabel(target, target.id) }}</strong>
                      <span>{{ endpointAddressText(target) }}</span>
                    </span>
                  </template>
                </v-checkbox>
              </div>
            </v-expansion-panel-text>
          </v-expansion-panel>
        </v-expansion-panels>
      </v-expansion-panel-text>
    </v-expansion-panel>
  </v-expansion-panels>
</div>
<div class="multi-location-ping__source-actions">
  <v-btn
    color="primary"
    :loading="store.loading"
    :disabled="selectedOutboundTargetIds.length === 0"
    @click="runOutbound"
  >
    Start Outbound Test
  </v-btn>
</div>
```

- [ ] **Step 8: Add compact picker styles**

Add styles:

```css
.multi-location-ping__catalog-actions {
  align-items: center;
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
}
.multi-location-ping__selection-count {
  color: color-mix(in srgb, currentColor 68%, transparent);
  font-size: 13px;
}
.multi-location-ping__provider-title,
.multi-location-ping__group-title {
  align-items: center;
  display: flex;
  gap: 10px;
  min-width: 0;
}
.multi-location-ping__target-grid {
  display: grid;
  gap: 4px 12px;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
}
.multi-location-ping__target-label {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.multi-location-ping__target-label span {
  color: color-mix(in srgb, currentColor 64%, transparent);
  font-family: var(--app-font-mono, monospace);
  font-size: 11px;
  overflow-wrap: anywhere;
}
```

- [ ] **Step 9: Run view tests and commit**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui bash -lc "cd src/frontend && npm test -- src/views/MultiLocationPing.test.ts"
```

Expected: PASS.

Commit:

```bash
git -c safe.directory=/home/bean/workspace/bproject/b-ui add src/frontend/src/views/MultiLocationPing.vue src/frontend/src/views/MultiLocationPing.test.ts
git -c safe.directory=/home/bean/workspace/bproject/b-ui commit -m "feat: select outbound ping target nodes"
```

## Task 6: Generate Full ZStatic Seed and Final Verification

**Files:**
- Modify: `src/backend/internal/domain/services/ping/catalogs/external_targets.seed.json`

- [ ] **Step 1: Refresh runtime catalog from real providers**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui scripts/dev/refresh-external-targets.sh -providers zstatic_cdn,linode_speedtest
```

Expected: command prints `refreshed external targets: 5 providers` and writes `data/ping/external/targets.json`.

- [ ] **Step 2: Copy refreshed catalog into seed file**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui bash -lc "cp data/ping/external/targets.json src/backend/internal/domain/services/ping/catalogs/external_targets.seed.json"
```

Expected: seed file now contains full ZStaticCDN targets and expanded Linode targets.

- [ ] **Step 3: Verify full ZStatic count**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui bash -lc "python3 - <<'PY'
import json
data=json.load(open('src/backend/internal/domain/services/ping/catalogs/external_targets.seed.json'))
providers={p['provider_id']:p for p in data['providers']}
z=providers['zstatic_cdn']['targets']
print(len(z))
assert len(z) >= 316
assert any(t['level']=='province' for t in z)
assert any(t['level']=='city' for t in z)
PY"
```

Expected: prints a number at least `316` and exits 0.

- [ ] **Step 4: Run backend tests**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui go test ./src/backend/internal/domain/services/ping ./src/backend/internal/http/api ./src/backend/internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 5: Run frontend focused tests**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui bash -lc "cd src/frontend && npm test -- src/store/modules/ping.test.ts src/views/MultiLocationPing.test.ts"
```

Expected: PASS.

- [ ] **Step 6: Run frontend build**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui bash -lc "cd src/frontend && npm run build:dist"
```

Expected: PASS with `vue-tsc --noEmit` and Vite build success.

- [ ] **Step 7: Review final diff**

Run:

```bash
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui git -c safe.directory=/home/bean/workspace/bproject/b-ui status --short
rtk wsl -d Ubuntu --cd /home/bean/workspace/bproject/b-ui git -c safe.directory=/home/bean/workspace/bproject/b-ui diff --stat HEAD
```

Expected: only intended files from this plan are changed beyond pre-existing user edits.

- [ ] **Step 8: Commit generated seed and final adjustments**

Commit:

```bash
git -c safe.directory=/home/bean/workspace/bproject/b-ui add src/backend/internal/domain/services/ping/catalogs/external_targets.seed.json
git -c safe.directory=/home/bean/workspace/bproject/b-ui commit -m "chore: seed refreshed external ping targets"
```

## Final Verification Checklist

- [ ] `go test ./src/backend/internal/domain/services/ping ./src/backend/internal/http/api ./src/backend/internal/cli -count=1` passes.
- [ ] `cd src/frontend && npm test -- src/store/modules/ping.test.ts src/views/MultiLocationPing.test.ts` passes.
- [ ] `cd src/frontend && npm run build:dist` passes.
- [ ] Seed catalog includes at least 316 ZStaticCDN targets.
- [ ] Outbound UI starts with zero selected targets.
- [ ] Outbound API request includes `target_node_ids`.
- [ ] Refresh button calls `POST /api/ping/external/targets/refresh`.
