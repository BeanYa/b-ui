package util

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func TestLinkGeneratorFallsBackToInboundAddressWhenAddrsMissing(t *testing.T) {
	tests := []struct {
		name  string
		addrs json.RawMessage
	}{
		{name: "nil"},
		{name: "null", addrs: json.RawMessage(`null`)},
		{name: "empty list", addrs: json.RawMessage(`[]`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := LinkGenerator(
				json.RawMessage(`{"vless":{"uuid":"11111111-1111-4111-8111-111111111111"}}`),
				&model.Inbound{
					Type:    "vless",
					Tag:     "edge-main-node-a",
					Addrs:   tt.addrs,
					Options: json.RawMessage(`{"listen_port":32001}`),
				},
				"edge.example.com",
			)

			if len(links) != 1 {
				t.Fatalf("expected generated fallback link, got %#v", links)
			}
			if !strings.Contains(links[0], "vless://11111111-1111-4111-8111-111111111111@edge.example.com:32001") {
				t.Fatalf("expected fallback address and port, got %q", links[0])
			}
			if !strings.Contains(links[0], "#edge-main-node-a") {
				t.Fatalf("expected inbound tag remark, got %q", links[0])
			}
		})
	}
}
