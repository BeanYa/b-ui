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
