package ping

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const zstaticProviderID = "zstatic_cdn"

type zstaticPageMetadata struct {
	provinceNames map[string]string
	cityNames     map[string]string
}

type zstaticNodeData struct {
	ProvinceBaseData []struct {
		Province string            `json:"province"`
		Carriers map[string]string `json:"carriers"`
	} `json:"provinceBaseData"`
	CityKeyList       []string                       `json:"cityKeyList"`
	ExtraCityNodeMeta map[string]zstaticCityNodeMeta `json:"extraCityNodeMeta"`
}

type zstaticCityNodeMeta struct {
	Province string `json:"province"`
	City     string `json:"city"`
	Carrier  string `json:"carrier"`
}

func (m *zstaticCityNodeMeta) UnmarshalJSON(data []byte) error {
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.Province = firstNonEmpty(raw["province"], raw["provinceName"], raw["region"], raw["p"])
	m.City = firstNonEmpty(raw["city"], raw["cityName"], raw["c"])
	m.Carrier = firstNonEmpty(raw["carrier"], raw["operator"], raw["network"], raw["isp"], raw["i"])
	return nil
}

func refreshZStaticCatalog(ctx context.Context, client *http.Client, entryURL string, now func() time.Time) (ExternalTargetProviderCatalog, error) {
	if strings.TrimSpace(entryURL) == "" {
		entryURL = "https://zstaticcdn.com/"
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
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
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return ExternalTargetProviderCatalog{}, fmt.Errorf("zstatic entry returned %d", resp.StatusCode)
	}
	htmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	finalURL := resp.Request.URL
	html := string(htmlBytes)
	scriptURL, err := zstaticNodeScriptURL(html, finalURL)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	scriptReq, err := http.NewRequestWithContext(ctx, http.MethodGet, scriptURL, nil)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	scriptResp, err := client.Do(scriptReq)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	defer scriptResp.Body.Close()
	if scriptResp.StatusCode < 200 || scriptResp.StatusCode >= 400 {
		return ExternalTargetProviderCatalog{}, fmt.Errorf("zstatic node data returned %d", scriptResp.StatusCode)
	}
	scriptBytes, err := io.ReadAll(scriptResp.Body)
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	targets, err := parseZStaticNodeData(string(scriptBytes), parseZStaticPageMetadata(html))
	if err != nil {
		return ExternalTargetProviderCatalog{}, err
	}
	return ExternalTargetProviderCatalog{
		ProviderID:   zstaticProviderID,
		ProviderName: "ZStaticCDN",
		UpdatedAt:    now().Unix(),
		Targets:      targets,
	}, nil
}

func zstaticNodeScriptURL(html string, finalURL *url.URL) (string, error) {
	re := regexp.MustCompile(`(?is)<script[^>]+src=["']([^"']*nodes_data\.js[^"']*)["']`)
	match := re.FindStringSubmatch(html)
	if len(match) < 2 {
		return "", fmt.Errorf("zstatic nodes_data.js script not found")
	}
	ref, err := url.Parse(strings.TrimSpace(match[1]))
	if err != nil {
		return "", err
	}
	return finalURL.ResolveReference(ref).String(), nil
}

func parseZStaticPageMetadata(html string) zstaticPageMetadata {
	return zstaticPageMetadata{
		provinceNames: parseZStaticMapLiteral(html, "provinceNameMap"),
		cityNames:     parseZStaticMapLiteral(html, "cityNameMap"),
	}
}

func parseZStaticMapLiteral(html, name string) map[string]string {
	re := regexp.MustCompile(`(?s)const\s+` + regexp.QuoteMeta(name) + `\s*=\s*\{(.*?)\}\s*;`)
	match := re.FindStringSubmatch(html)
	if len(match) < 2 {
		return nil
	}
	result := map[string]string{}
	entryRe := regexp.MustCompile(`["']?([A-Za-z0-9_-]+)["']?\s*:\s*["']([^"']+)["']`)
	for _, entry := range entryRe.FindAllStringSubmatch(match[1], -1) {
		result[entry[1]] = entry[2]
	}
	return result
}

func parseZStaticNodeData(script string, meta zstaticPageMetadata) ([]ExternalEndpoint, error) {
	raw, err := zstaticObjectLiteral(script)
	if err != nil {
		return nil, err
	}
	var data zstaticNodeData
	if err := json.Unmarshal([]byte(quoteJSObjectKeys(raw)), &data); err != nil {
		return nil, err
	}
	cityKeys := zstaticCityKeys(data.CityKeyList, data.ExtraCityNodeMeta)
	targets := make([]ExternalEndpoint, 0, len(data.ProvinceBaseData)*3+len(cityKeys))
	for _, province := range data.ProvinceBaseData {
		for carrier, hostPort := range province.Carriers {
			host, port, key := parseZStaticPublishedHost(hostPort)
			targets = append(targets, zstaticTarget(key, province.Province, "", carrier, host, port, "province"))
		}
	}
	for _, key := range cityKeys {
		province, city, carrier := zstaticCityMetadata(key, meta, data.ExtraCityNodeMeta[key])
		targets = append(targets, zstaticTarget(key, province, city, carrier, key+".ip.zstaticcdn.com", 443, "city"))
	}
	return targets, nil
}

func zstaticCityKeys(cityKeyList []string, extra map[string]zstaticCityNodeMeta) []string {
	keys := make([]string, 0, len(cityKeyList)+len(extra))
	seen := map[string]bool{}
	for _, key := range cityKeyList {
		key = strings.TrimSpace(key)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range extra {
		key = strings.TrimSpace(key)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

func zstaticObjectLiteral(script string) (string, error) {
	start := strings.Index(script, "{")
	end := strings.LastIndex(script, "}")
	if start < 0 || end <= start {
		return "", fmt.Errorf("zstatic node data object not found")
	}
	return script[start : end+1], nil
}

func quoteJSObjectKeys(raw string) string {
	re := regexp.MustCompile(`([{\[,]\s*)([A-Za-z_][A-Za-z0-9_]*)\s*:`)
	return re.ReplaceAllString(raw, `${1}"$2":`)
}

func parseZStaticPublishedHost(hostPort string) (string, int, string) {
	hostPort = strings.TrimSpace(hostPort)
	host, portText, err := net.SplitHostPort(hostPort)
	if err != nil {
		host = hostPort
		portText = "80"
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 {
		port = 80
	}
	key := strings.TrimSuffix(host, ".ip.zstaticcdn.com")
	return host, port, key
}

func zstaticCityMetadata(key string, meta zstaticPageMetadata, extra zstaticCityNodeMeta) (string, string, string) {
	parts := strings.Split(key, "-")
	provinceCode, cityCode, carrier := "", "", ""
	if len(parts) >= 4 {
		provinceCode, cityCode, carrier = parts[0], parts[1], parts[2]
	}
	province := lookupZStaticName(provinceCode, meta.provinceNames, fallbackZStaticProvinceNames)
	city := lookupZStaticName(cityCode, meta.cityNames, fallbackZStaticCityNames)
	if extra.Province != "" {
		province = extra.Province
	}
	if extra.City != "" {
		city = extra.City
	}
	if extra.Carrier != "" {
		carrier = extra.Carrier
	}
	return province, city, carrier
}

func lookupZStaticName(code string, primary map[string]string, fallback map[string]string) string {
	if primary != nil {
		if name := primary[code]; name != "" {
			return name
		}
	}
	if name := fallback[code]; name != "" {
		return name
	}
	return strings.ToUpper(code)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func zstaticTarget(key, province, city, carrier, host string, port int, level string) ExternalEndpoint {
	network := zstaticCarrierNetwork(carrier)
	group := province
	label := province + zstaticCarrierLabel(carrier)
	if level == "city" {
		group = province + " / " + city
		label = province + city + zstaticCarrierLabel(carrier)
	}
	return ExternalEndpoint{
		ID:       zstaticProviderID + ":" + key,
		Label:    label,
		Provider: zstaticProviderID,
		Region:   province,
		Country:  "CN",
		City:     city,
		Network:  network,
		Group:    group,
		Level:    level,
		Host:     host,
		Port:     port,
		Methods:  []string{MethodTCP},
	}
}

func zstaticCarrierNetwork(carrier string) string {
	switch carrier {
	case "mobile", "cm":
		return "China Mobile"
	case "unicom", "cu":
		return "China Unicom"
	case "telecom", "ct":
		return "China Telecom"
	default:
		return strings.ToUpper(carrier)
	}
}

func zstaticCarrierLabel(carrier string) string {
	switch carrier {
	case "mobile", "cm":
		return "移动"
	case "unicom", "cu":
		return "联通"
	case "telecom", "ct":
		return "电信"
	default:
		return strings.ToUpper(carrier)
	}
}

var fallbackZStaticProvinceNames = map[string]string{
	"bj": "北京", "sh": "上海", "tj": "天津", "cq": "重庆", "hb": "河北", "sx": "山西",
	"ln": "辽宁", "jl": "吉林", "hlj": "黑龙江", "js": "江苏", "zj": "浙江", "ah": "安徽",
	"fj": "福建", "jx": "江西", "sd": "山东", "ha": "河南", "hub": "湖北", "hn": "湖南",
	"gd": "广东", "gx": "广西", "hi": "海南", "sc": "四川", "gz": "贵州", "yn": "云南",
	"xz": "西藏", "sn": "陕西", "gs": "甘肃", "qh": "青海", "nx": "宁夏", "xj": "新疆",
	"nm": "内蒙古", "hk": "香港", "mo": "澳门", "tw": "台湾",
}

var fallbackZStaticCityNames = map[string]string{
	"sjz": "石家庄", "hkg": "香港", "pek": "北京", "sha": "上海", "can": "广州", "sZX": "深圳",
}
