package ping

import "time"

const linodeSpeedtestProviderID = "linode_speedtest"

type linodeSpeedtestNode struct {
	key     string
	label   string
	region  string
	country string
	group   string
	host    string
}

var linodeSpeedtestNodes = []linodeSpeedtestNode{
	{key: "newark", label: "Linode Newark", region: "Newark", country: "US", group: "United States", host: "speedtest.newark.linode.com"},
	{key: "atlanta", label: "Linode Atlanta", region: "Atlanta", country: "US", group: "United States", host: "speedtest.atlanta.linode.com"},
	{key: "dallas", label: "Linode Dallas", region: "Dallas", country: "US", group: "United States", host: "speedtest.dallas.linode.com"},
	{key: "fremont", label: "Linode Fremont", region: "Fremont", country: "US", group: "United States", host: "speedtest.fremont.linode.com"},
	{key: "frankfurt", label: "Linode Frankfurt", region: "Frankfurt", country: "DE", group: "Europe", host: "speedtest.frankfurt.linode.com"},
	{key: "london", label: "Linode London", region: "London", country: "GB", group: "Europe", host: "speedtest.london.linode.com"},
	{key: "singapore", label: "Linode Singapore", region: "Singapore", country: "SG", group: "Asia Pacific", host: "speedtest.singapore.linode.com"},
	{key: "tokyo", label: "Linode Tokyo", region: "Tokyo", country: "JP", group: "Asia Pacific", host: "speedtest.tokyo2.linode.com"},
	{key: "sydney", label: "Linode Sydney", region: "Sydney", country: "AU", group: "Asia Pacific", host: "speedtest.syd1.linode.com"},
	{key: "mumbai", label: "Linode Mumbai", region: "Mumbai", country: "IN", group: "Asia Pacific", host: "speedtest.mumbai1.linode.com"},
	{key: "toronto", label: "Linode Toronto", region: "Toronto", country: "CA", group: "North America", host: "speedtest.toronto1.linode.com"},
}

func refreshLinodeCatalog(now time.Time) ExternalTargetProviderCatalog {
	targets := make([]ExternalEndpoint, 0, len(linodeSpeedtestNodes))
	for _, node := range linodeSpeedtestNodes {
		targets = append(targets, ExternalEndpoint{
			ID:       linodeSpeedtestProviderID + ":" + node.key,
			Label:    node.label,
			Provider: linodeSpeedtestProviderID,
			Region:   node.region,
			Country:  node.country,
			Group:    node.group,
			Host:     node.host,
			Port:     80,
			Methods:  []string{MethodTCP, MethodHTTP, MethodICMP},
		})
	}
	return ExternalTargetProviderCatalog{
		ProviderID:   linodeSpeedtestProviderID,
		ProviderName: "Linode/Akamai Speed Test",
		UpdatedAt:    now.Unix(),
		Targets:      targets,
	}
}
