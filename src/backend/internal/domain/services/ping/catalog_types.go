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
