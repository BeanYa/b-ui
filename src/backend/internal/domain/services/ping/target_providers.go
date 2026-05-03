package ping

func zstaticTargets() []ExternalEndpoint {
	return []ExternalEndpoint{
		{ID: "he-cm-v4", Label: "Hebei Mobile", Provider: "zstatic_cdn", Region: "Hebei", Country: "CN", Network: "China Mobile", Host: "he-cm-v4.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}},
		{ID: "he-cu-v4", Label: "Hebei Unicom", Provider: "zstatic_cdn", Region: "Hebei", Country: "CN", Network: "China Unicom", Host: "he-cu-v4.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}},
		{ID: "he-ct-v4", Label: "Hebei Telecom", Provider: "zstatic_cdn", Region: "Hebei", Country: "CN", Network: "China Telecom", Host: "he-ct-v4.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}},
		{ID: "sx-cm-v4", Label: "Shanxi Mobile", Provider: "zstatic_cdn", Region: "Shanxi", Country: "CN", Network: "China Mobile", Host: "sx-cm-v4.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}},
		{ID: "sx-cu-v4", Label: "Shanxi Unicom", Provider: "zstatic_cdn", Region: "Shanxi", Country: "CN", Network: "China Unicom", Host: "sx-cu-v4.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}},
		{ID: "sx-ct-v4", Label: "Shanxi Telecom", Provider: "zstatic_cdn", Region: "Shanxi", Country: "CN", Network: "China Telecom", Host: "sx-ct-v4.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}},
	}
}

func linodeSpeedtestTargets() []ExternalEndpoint {
	return []ExternalEndpoint{
		{ID: "linode-newark", Label: "Linode Newark", Provider: "linode_speedtest", Region: "New Jersey", Country: "US", Host: "speedtest.newark.linode.com", Port: 80, Methods: []string{MethodTCP, MethodHTTP, MethodICMP}},
		{ID: "linode-fremont", Label: "Linode Fremont", Provider: "linode_speedtest", Region: "California", Country: "US", Host: "speedtest.fremont.linode.com", Port: 80, Methods: []string{MethodTCP, MethodHTTP, MethodICMP}},
		{ID: "linode-frankfurt", Label: "Linode Frankfurt", Provider: "linode_speedtest", Region: "Frankfurt", Country: "DE", Host: "speedtest.frankfurt.linode.com", Port: 80, Methods: []string{MethodTCP, MethodHTTP, MethodICMP}},
		{ID: "linode-singapore", Label: "Linode Singapore", Provider: "linode_speedtest", Region: "Singapore", Country: "SG", Host: "speedtest.singapore.linode.com", Port: 80, Methods: []string{MethodTCP, MethodHTTP, MethodICMP}},
		{ID: "linode-tokyo", Label: "Linode Tokyo", Provider: "linode_speedtest", Region: "Tokyo", Country: "JP", Host: "speedtest.tokyo2.linode.com", Port: 80, Methods: []string{MethodTCP, MethodHTTP, MethodICMP}},
	}
}

func publicDNSTargets() []ExternalEndpoint {
	return []ExternalEndpoint{
		{ID: "cloudflare-dns", Label: "Cloudflare DNS", Provider: "public_dns", Country: "Global", Network: "Cloudflare", Host: "1.1.1.1", Port: 53, Methods: []string{MethodTCP, MethodICMP}},
		{ID: "google-dns", Label: "Google DNS", Provider: "public_dns", Country: "Global", Network: "Google", Host: "8.8.8.8", Port: 53, Methods: []string{MethodTCP, MethodICMP}},
		{ID: "quad9-dns", Label: "Quad9 DNS", Provider: "public_dns", Country: "Global", Network: "Quad9", Host: "9.9.9.9", Port: 53, Methods: []string{MethodTCP, MethodICMP}},
		{ID: "114-dns", Label: "114 DNS", Provider: "public_dns", Country: "CN", Network: "114DNS", Host: "114.114.114.114", Port: 53, Methods: []string{MethodTCP, MethodICMP}},
	}
}

func cdnEdgeTargets() []ExternalEndpoint {
	return []ExternalEndpoint{
		{ID: "cloudflare-edge", Label: "Cloudflare Edge", Provider: "cdn_edges", Country: "Global", Network: "Cloudflare", Host: "cloudflare.com", Port: 443, Methods: []string{MethodHTTP, MethodTCP}},
		{ID: "fastly-edge", Label: "Fastly Edge", Provider: "cdn_edges", Country: "Global", Network: "Fastly", Host: "www.fastly.com", Port: 443, Methods: []string{MethodHTTP, MethodTCP}},
		{ID: "akamai-edge", Label: "Akamai Edge", Provider: "cdn_edges", Country: "Global", Network: "Akamai", Host: "www.akamai.com", Port: 443, Methods: []string{MethodHTTP, MethodTCP}},
	}
}

func cloudTestTargets() []ExternalEndpoint {
	return []ExternalEndpoint{
		{ID: "aws-tokyo", Label: "AWS Tokyo", Provider: "cloud_test_ips", Region: "Tokyo", Country: "JP", Network: "AWS", Host: "ec2.ap-northeast-1.amazonaws.com", Port: 443, Methods: []string{MethodTCP, MethodHTTP}},
		{ID: "aws-singapore", Label: "AWS Singapore", Provider: "cloud_test_ips", Region: "Singapore", Country: "SG", Network: "AWS", Host: "ec2.ap-southeast-1.amazonaws.com", Port: 443, Methods: []string{MethodTCP, MethodHTTP}},
		{ID: "aws-virginia", Label: "AWS N. Virginia", Provider: "cloud_test_ips", Region: "Virginia", Country: "US", Network: "AWS", Host: "ec2.us-east-1.amazonaws.com", Port: 443, Methods: []string{MethodTCP, MethodHTTP}},
	}
}

func targetsForProvider(id string) []ExternalEndpoint {
	switch id {
	case "zstatic_cdn":
		return zstaticTargets()
	case "linode_speedtest":
		return linodeSpeedtestTargets()
	case "public_dns":
		return publicDNSTargets()
	case "cdn_edges":
		return cdnEdgeTargets()
	case "cloud_test_ips":
		return cloudTestTargets()
	default:
		return nil
	}
}
