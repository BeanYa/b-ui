package service

import "testing"

func TestProtocolLabel(t *testing.T) {
	cases := map[string]string{
		"vless": "Vless", "vmess": "Vmess", "trojan": "Trojan",
		"hysteria2": "Hy2", "Hysteria2": "Hy2", "shadowsocks": "Shadowsocks",
		"": "", "vless ": "Vless",
	}
	for in, want := range cases {
		if got := ProtocolLabel(in); got != want {
			t.Errorf("ProtocolLabel(%q)=%q want %q", in, got, want)
		}
	}
}

func TestSecurityLabel(t *testing.T) {
	cases := []struct {
		template string
		reality  bool
		want     string
	}{
		{"standard", false, "TLS"},
		{"reality", true, "Reality"},
		{"hysteria2", false, "Hy2"},
		{"none", false, ""},
		{"", false, ""},
	}
	for _, c := range cases {
		if got := SecurityLabel(c.template, c.reality); got != c.want {
			t.Errorf("SecurityLabel(%q,%v)=%q want %q", c.template, c.reality, got, c.want)
		}
	}
}

func TestCountryFlagEmoji(t *testing.T) {
	if got := CountryFlagEmoji("JP"); got != "🇯🇵" {
		t.Errorf("JP=%q want 🇯🇵", got)
	}
	if got := CountryFlagEmoji("us"); got != "🇺🇸" {
		t.Errorf("us=%q want 🇺🇸", got)
	}
	if got := CountryFlagEmoji(""); got != "" {
		t.Errorf("empty=%q want empty", got)
	}
	if got := CountryFlagEmoji("USA"); got != "" {
		t.Errorf("USA=%q want empty (not 2 letters)", got)
	}
}

func TestBuildInboundRemark(t *testing.T) {
	all := NamingRule{IncludeProtocol: true, IncludeSecurity: true, IncludeFlag: true}
	cases := []struct {
		name                      string
		rule                      NamingRule
		proto, sec, country, disp string
		want                      string
	}{
		{"hy2+hy2 dedup", all, "Hy2", "Hy2", "JP", "Mynode", "🇯🇵 Hy2 Mynode"},
		{"hy2 proto + tls sec", all, "Hy2", "TLS", "JP", "Mynode", "🇯🇵 Hy2 TLS Mynode"},
		{"vless + hy2 sec", all, "Vless", "Hy2", "JP", "Mynode", "🇯🇵 Vless Hy2 Mynode"},
		{"vless reality us", all, "Vless", "Reality", "US", "Box", "🇺🇸 Vless Reality Box"},
		{"empty country omits flag", all, "Vless", "TLS", "", "Box", "Vless TLS Box"},
		{"flag off", NamingRule{IncludeProtocol: true, IncludeSecurity: true, IncludeFlag: false}, "Vless", "TLS", "JP", "Box", "Vless TLS Box"},
		{"only name", NamingRule{}, "Vless", "TLS", "JP", "Box", "Box"},
		{"sec none omitted", all, "Vless", "", "JP", "Box", "🇯🇵 Vless Box"},
	}
	for _, c := range cases {
		got := BuildInboundRemark(c.rule, c.proto, c.sec, c.country, c.disp)
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestBuildInboundSlug(t *testing.T) {
	all := NamingRule{IncludeProtocol: true, IncludeSecurity: true, IncludeFlag: true}
	cases := []struct {
		name                      string
		rule                      NamingRule
		proto, sec, country, disp string
		want                      string
	}{
		{"hy2+hy2 dedup", all, "Hy2", "Hy2", "JP", "Mynode", "jp-hy2-mynode"},
		{"vless reality us", all, "Vless", "Reality", "US", "Box", "us-vless-reality-box"},
		{"empty country", all, "Vless", "TLS", "", "Box", "vless-tls-box"},
		{"flag off", NamingRule{IncludeProtocol: true, IncludeSecurity: true, IncludeFlag: false}, "Vless", "TLS", "JP", "Box", "vless-tls-box"},
		{"displayname sanitized", all, "Vless", "TLS", "JP", "My Node!!", "jp-vless-tls-my-node"},
		{"only name", NamingRule{}, "Vless", "TLS", "JP", "Box", "box"},
	}
	for _, c := range cases {
		got := BuildInboundSlug(c.rule, c.proto, c.sec, c.country, c.disp)
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
