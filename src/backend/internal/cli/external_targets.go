package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/ping"
)

func refreshExternalTargetsCmd(args []string) {
	externalTargetsCmd := flag.NewFlagSet("external-targets", flag.ExitOnError)
	var providers string
	externalTargetsCmd.StringVar(&providers, "providers", "", "comma-separated provider IDs to refresh")
	if err := externalTargetsCmd.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	providerIDs := splitProviderIDs(providers)
	catalog, err := ping.NewTargetCatalogService(ping.NewStore()).Refresh(context.Background(), providerIDs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "refresh external targets:", err)
		os.Exit(1)
	}
	fmt.Printf("refreshed %d providers\n", refreshedProviderCount(providerIDs, catalog))
}

func splitProviderIDs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	providerIDs := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id != "" && !seen[id] {
			seen[id] = true
			providerIDs = append(providerIDs, id)
		}
	}
	return providerIDs
}

func refreshedProviderCount(providerIDs []string, catalog *ping.ExternalTargetCatalog) int {
	if len(providerIDs) > 0 {
		return len(providerIDs)
	}
	if catalog == nil {
		return 0
	}
	return len(catalog.Providers)
}
