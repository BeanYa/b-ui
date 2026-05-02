package clustertypes

import "encoding/json"

type DomainInboundTLS struct {
	Name   string          `json:"name"`
	Server json.RawMessage `json:"server"`
	Client json.RawMessage `json:"client"`
}

type DomainInboundCreatePayload struct {
	RequestID   string            `json:"request_id"`
	DomainID    string            `json:"domain_id"`
	Prefix      string            `json:"prefix"`
	Suffix      string            `json:"suffix"`
	Inbound     json.RawMessage   `json:"inbound"`
	TLSTemplate string            `json:"tls_template"`
	TLS         *DomainInboundTLS `json:"tls,omitempty"`
}
