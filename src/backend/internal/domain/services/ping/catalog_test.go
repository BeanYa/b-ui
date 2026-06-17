package ping

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
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
	if len(targets) != 10 {
		t.Fatalf("expected 10 targets, got %d: %#v", len(targets), targets)
	}
	byID := map[string]ExternalEndpoint{}
	targetIDs := make([]string, 0, len(targets))
	for _, target := range targets {
		targetIDs = append(targetIDs, target.ID)
		byID[target.ID] = target
		if target.Provider != "zstatic_cdn" {
			t.Fatalf("expected zstatic provider, got %#v", target)
		}
		if len(target.Methods) != 1 || target.Methods[0] != MethodTCP {
			t.Fatalf("expected TCP-only target, got %#v", target)
		}
	}
	if !sort.StringsAreSorted(targetIDs) {
		t.Fatalf("expected zstatic targets sorted by ID, got %#v", targetIDs)
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
	if byID["zstatic_cdn:mo-mac-ct-v4"].Region != "澳门" || byID["zstatic_cdn:mo-mac-ct-v4"].City != "澳门" || byID["zstatic_cdn:mo-mac-ct-v4"].Network != "China Telecom" {
		t.Fatalf("expected extra-only city metadata, got %#v", byID["zstatic_cdn:mo-mac-ct-v4"])
	}
	if byID["zstatic_cdn:ah-anqing-cu-v4"].Region != "安徽" || byID["zstatic_cdn:ah-anqing-cu-v4"].City != "安庆" || byID["zstatic_cdn:ah-anqing-cu-v4"].Group != "安徽 / 安庆" || byID["zstatic_cdn:ah-anqing-cu-v4"].Network != "China Unicom" {
		t.Fatalf("expected string extra metadata, got %#v", byID["zstatic_cdn:ah-anqing-cu-v4"])
	}
	guilin := byID["zstatic_cdn:gx-guilin-cu-v4"]
	if guilin.City != "桂林" || !strings.Contains(guilin.Group, "桂林") || strings.Contains(guilin.Region, "桂林") {
		t.Fatalf("expected autonomous region metadata split, got %#v", guilin)
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
	if provider.ProviderID != "zstatic_cdn" || len(provider.Targets) != 10 {
		t.Fatalf("expected refreshed zstatic provider, got %#v", provider)
	}
}

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

func TestTargetCatalogRefreshPreservesStaticProviders(t *testing.T) {
	store := newStoreWithDir(t.TempDir())
	svc := NewTargetCatalogService(store)
	svc.now = func() time.Time { return time.Unix(1710000000, 0) }
	svc.refreshZStatic = func(ctx context.Context, client *http.Client, entryURL string, now func() time.Time) (ExternalTargetProviderCatalog, error) {
		return ExternalTargetProviderCatalog{
			ProviderID:   "zstatic_cdn",
			ProviderName: "ZStaticCDN",
			UpdatedAt:    now().Unix(),
			Targets: []ExternalEndpoint{{
				ID: "zstatic_cdn:test", Label: "Test", Provider: "zstatic_cdn", Group: "Test",
				Host: "test.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP},
			}},
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

func TestFilterCatalogTargetsByIDRequiresSelectionAndMatchesProviders(t *testing.T) {
	catalog, err := loadSeedExternalTargetCatalog()
	if err != nil {
		t.Fatalf("loadSeedExternalTargetCatalog: %v", err)
	}
	_, err = filterCatalogTargetsByID(catalog, []string{"public_dns"}, nil)
	if err == nil || !strings.Contains(err.Error(), "target_node_ids") {
		t.Fatalf("expected target_node_ids error, got %v", err)
	}
	targets, err := filterCatalogTargetsByID(catalog, []string{"public_dns"}, []string{"public_dns:cloudflare-dns", "cdn_edges:cloudflare-edge"})
	if err != nil {
		t.Fatalf("filterCatalogTargetsByID: %v", err)
	}
	if len(targets) != 1 || targets[0].ID != "public_dns:cloudflare-dns" {
		t.Fatalf("expected only public dns target, got %#v", targets)
	}
	_, err = filterCatalogTargetsByID(catalog, []string{"public_dns"}, []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), "no selected outbound targets matched") {
		t.Fatalf("expected no match error, got %v", err)
	}
}
