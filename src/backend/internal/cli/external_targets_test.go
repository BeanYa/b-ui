package cmd

import (
	"testing"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/ping"
)

func TestSplitProviderIDsTrimsAndDedupes(t *testing.T) {
	got := splitProviderIDs(" public_dns,public_dns, cdn_edges ,, ")
	want := []string{"public_dns", "cdn_edges"}
	if len(got) != len(want) {
		t.Fatalf("expected %d provider IDs, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected provider ID %d to be %q, got %q", i, want[i], got[i])
		}
	}
}

func TestRefreshedProviderCountUsesRequestedProviderIDsWhenSupplied(t *testing.T) {
	catalog := &ping.ExternalTargetCatalog{Providers: []ping.ExternalTargetProviderCatalog{
		{ProviderID: "public_dns"},
		{ProviderID: "cdn_edges"},
		{ProviderID: "cloud_test_ips"},
	}}

	if got := refreshedProviderCount([]string{"public_dns"}, catalog); got != 1 {
		t.Fatalf("expected one requested provider, got %d", got)
	}
	if got := refreshedProviderCount(nil, catalog); got != 3 {
		t.Fatalf("expected full catalog provider count, got %d", got)
	}
}
