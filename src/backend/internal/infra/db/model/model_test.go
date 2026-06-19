package model

import "testing"

func TestInboundRemarkRoundTrip(t *testing.T) {
	raw := []byte(`{"type":"vless","tag":"jp-vless-box","remark":"🇯🇵 Vless Box","listen_port":443}`)
	var in Inbound
	if err := in.UnmarshalJSON(raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.Tag != "jp-vless-box" {
		t.Errorf("tag=%q", in.Tag)
	}
	if in.Remark != "🇯🇵 Vless Box" {
		t.Errorf("remark=%q want pretty", in.Remark)
	}
	out, err := in.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !contains(out, `"remark":"🇯🇵 Vless Box"`) {
		t.Errorf("marshal missing remark: %s", out)
	}
}

func contains(haystack []byte, needle string) bool {
	return string(haystack) != "" && indexOf(string(haystack), needle) >= 0
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
