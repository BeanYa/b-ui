package clustertypes

import "encoding/json"

type DomainInboundTLS struct {
	Name   string          `json:"name"`
	Server json.RawMessage `json:"server"`
	Client json.RawMessage `json:"client"`
}

type DomainInboundCreatePayload struct {
	RequestID       string                `json:"request_id"`
	DomainID        string                `json:"domain_id"`
	GroupID         string                `json:"group_id"`
	TagSeed         string                `json:"tag_seed"`
	TargetMembers   []DomainInboundTarget `json:"target_members"`
	Prefix          string                `json:"prefix"`
	Suffix          string                `json:"suffix"`
	IncludeProtocol bool                  `json:"include_protocol,omitempty"`
	IncludeSecurity bool                  `json:"include_security,omitempty"`
	IncludeFlag     bool                  `json:"include_flag,omitempty"`
	Inbound         json.RawMessage       `json:"inbound"`
	TLSTemplate     string                `json:"tls_template"`
	TLS             *DomainInboundTLS     `json:"tls,omitempty"`
}

type DomainInboundUpdatePayload = DomainInboundCreatePayload

type DomainInboundDeletePayload struct {
	RequestID     string                `json:"request_id"`
	GroupID       string                `json:"group_id"`
	DomainID      string                `json:"domain_id"`
	TargetMembers []DomainInboundTarget `json:"target_members"`
}

type DomainInboundTarget struct {
	MemberID               string `json:"member_id"`
	NodeID                 string `json:"node_id"`
	DisplayName            string `json:"display_name"`
	CountryCode            string `json:"country_code,omitempty"`
	TargetTag              string `json:"target_tag,omitempty"`
	TargetRemark           string `json:"target_remark,omitempty"`
	RemoteInboundID        uint   `json:"remote_inbound_id,omitempty"`
	DomainInboundRequestID string `json:"domain_inbound_request_id,omitempty"`
}
