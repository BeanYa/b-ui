package clustertypes

import "encoding/json"

type DomainInboundTLS struct {
	Name   string          `json:"name"`
	Server json.RawMessage `json:"server"`
	Client json.RawMessage `json:"client"`
}

type DomainInboundCreatePayload struct {
	RequestID     string                `json:"request_id"`
	DomainID      string                `json:"domain_id"`
	GroupID       string                `json:"group_id"`
	TagSeed       string                `json:"tag_seed"`
	TargetMembers []DomainInboundTarget `json:"target_members"`
	Prefix        string                `json:"prefix"`
	Suffix        string                `json:"suffix"`
	Inbound       json.RawMessage       `json:"inbound"`
	TLSTemplate   string                `json:"tls_template"`
	TLS           *DomainInboundTLS     `json:"tls,omitempty"`
}

type DomainInboundUpdatePayload = DomainInboundCreatePayload

type DomainInboundDeletePayload struct {
	RequestID     string                `json:"request_id"`
	GroupID       string                `json:"group_id"`
	DomainID      string                `json:"domain_id"`
	TargetMembers []DomainInboundTarget `json:"target_members"`
}

type DomainInboundTarget struct {
	MemberID    string `json:"member_id"`
	NodeID      string `json:"node_id"`
	DisplayName string `json:"display_name"`
}
