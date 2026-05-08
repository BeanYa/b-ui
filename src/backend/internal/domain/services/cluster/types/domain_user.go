package clustertypes

import "encoding/json"

type DomainUserPayload struct {
	UUID                 string          `json:"uuid"`
	Name                 string          `json:"name"`
	Enable               bool            `json:"enable"`
	Desc                 string          `json:"desc,omitempty"`
	Group                string          `json:"group,omitempty"`
	SubToken             string          `json:"sub_token,omitempty"`
	BoundInboundGroupIDs []string        `json:"bound_inbound_group_ids,omitempty"`
	Config               json.RawMessage `json:"config"`
	Volume               int64           `json:"volume,omitempty"`
	Expiry               int64           `json:"expiry,omitempty"`
	Down                 int64           `json:"down,omitempty"`
	Up                   int64           `json:"up,omitempty"`
	DelayStart           bool            `json:"delay_start,omitempty"`
	AutoReset            bool            `json:"auto_reset,omitempty"`
	ResetDays            int             `json:"reset_days,omitempty"`
	NextReset            int64           `json:"next_reset,omitempty"`
	TotalUp              int64           `json:"total_up,omitempty"`
	TotalDown            int64           `json:"total_down,omitempty"`
}

type DomainUserUpsertPayload struct {
	RequestID     string                `json:"request_id"`
	DomainID      string                `json:"domain_id"`
	User          DomainUserPayload     `json:"user"`
	Inbounds      []string              `json:"inbounds,omitempty"`
	TargetMembers []DomainInboundTarget `json:"target_members,omitempty"`
}

type DomainUserDeletePayload struct {
	RequestID     string                `json:"request_id"`
	DomainID      string                `json:"domain_id"`
	UUID          string                `json:"uuid"`
	TargetMembers []DomainInboundTarget `json:"target_members,omitempty"`
}
