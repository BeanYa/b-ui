package ping

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type zstaticRefreshFunc func(context.Context, *http.Client, string, func() time.Time) (ExternalTargetProviderCatalog, error)
type linodeRefreshFunc func(time.Time) ExternalTargetProviderCatalog

type TargetCatalogService struct {
	store           *Store
	httpClient      *http.Client
	zstaticEntryURL string
	now             func() time.Time
	refreshZStatic  func(context.Context, *http.Client, string, func() time.Time) (ExternalTargetProviderCatalog, error)
	refreshLinode   linodeRefreshFunc
}

func NewTargetCatalogService(store *Store) *TargetCatalogService {
	return &TargetCatalogService{
		store:           store,
		httpClient:      &http.Client{Timeout: 15 * time.Second},
		zstaticEntryURL: "https://zstaticcdn.com/",
		now:             time.Now,
		refreshZStatic:  refreshZStaticCatalog,
		refreshLinode:   refreshLinodeCatalog,
	}
}

func (s *TargetCatalogService) Load() (*ExternalTargetCatalog, error) {
	if s.store != nil {
		if catalog, err := s.store.LoadExternalTargetCatalog(); err == nil {
			return catalog, nil
		}
	}
	return loadSeedExternalTargetCatalog()
}

func (s *TargetCatalogService) Refresh(ctx context.Context, providerIDs []string) (*ExternalTargetCatalog, error) {
	base, err := s.Load()
	if err != nil {
		return nil, err
	}
	selected := selectedProviderSet(providerIDs)
	refreshAll := len(selected) == 0
	providers := make([]ExternalTargetProviderCatalog, 0, len(base.Providers))
	for _, provider := range base.Providers {
		switch provider.ProviderID {
		case zstaticProviderID:
			if refreshAll || selected[provider.ProviderID] {
				refreshed, err := s.refreshZStatic(ctx, s.httpClient, s.zstaticEntryURL, s.now)
				if err != nil {
					return nil, err
				}
				providers = append(providers, refreshed)
				continue
			}
		case linodeSpeedtestProviderID:
			if refreshAll || selected[provider.ProviderID] {
				providers = append(providers, s.refreshLinode(s.now()))
				continue
			}
		}
		providers = append(providers, provider)
	}
	catalog := &ExternalTargetCatalog{
		UpdatedAt: s.now().Unix(),
		Providers: providers,
	}
	if s.store != nil {
		if err := s.store.SaveExternalTargetCatalog(catalog); err != nil {
			return nil, err
		}
	}
	return catalog, nil
}

func selectedProviderSet(providerIDs []string) map[string]bool {
	selected := map[string]bool{}
	for _, id := range providerIDs {
		if id != "" {
			selected[id] = true
		}
	}
	return selected
}

func filterCatalogTargetsByID(catalog *ExternalTargetCatalog, providerIDs []string, targetIDs []string) ([]ExternalEndpoint, error) {
	if len(targetIDs) == 0 {
		return nil, fmt.Errorf("target_node_ids is required")
	}
	allowedProviders := selectedProviderSet(providerIDs)
	wantedTargets := map[string]bool{}
	for _, id := range targetIDs {
		wantedTargets[id] = true
	}
	var targets []ExternalEndpoint
	for _, target := range catalogTargetsForProviders(catalog, providerIDs) {
		if len(allowedProviders) > 0 && !allowedProviders[target.Provider] {
			continue
		}
		if wantedTargets[target.ID] {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no selected outbound targets matched")
	}
	return targets, nil
}

func catalogTargetsForProviders(catalog *ExternalTargetCatalog, providerIDs []string) []ExternalEndpoint {
	if catalog == nil {
		return nil
	}
	selected := selectedProviderSet(providerIDs)
	var targets []ExternalEndpoint
	for _, provider := range catalog.Providers {
		if len(selected) > 0 && !selected[provider.ProviderID] {
			continue
		}
		targets = append(targets, provider.Targets...)
	}
	return targets
}
