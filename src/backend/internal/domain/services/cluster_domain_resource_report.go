package service

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	"github.com/google/uuid"
)

type ClusterHubDomainResources struct {
	Inbounds []ClusterHubDomainResourceInbound `json:"domain_inbounds"`
	Users    []ClusterHubDomainResourceUser    `json:"domain_users"`
}

type ClusterHubDomainResourceInbound struct {
	GroupID             string                               `json:"group_id"`
	TagSeed             string                               `json:"tag_seed,omitempty"`
	Prefix              string                               `json:"prefix,omitempty"`
	Suffix              string                               `json:"suffix,omitempty"`
	Type                string                               `json:"type"`
	TLSTemplate         string                               `json:"tls_template,omitempty"`
	OptionsJSON         string                               `json:"options_json,omitempty"`
	Status              string                               `json:"status,omitempty"`
	Revision            int64                                `json:"revision,omitempty"`
	LastOperationID     string                               `json:"last_operation_id,omitempty"`
	LastOperationStatus string                               `json:"last_operation_status,omitempty"`
	Instances           []ClusterDomainOperationInstanceView `json:"instances,omitempty"`
}

type ClusterHubDomainResourceUser struct {
	ClientID   uint            `json:"client_id"`
	UUID       string          `json:"uuid"`
	Name       string          `json:"name"`
	Enable     bool            `json:"enable"`
	Desc       string          `json:"desc,omitempty"`
	Group      string          `json:"group,omitempty"`
	SubToken   string          `json:"sub_token"`
	Config     json.RawMessage `json:"config"`
	Inbounds   json.RawMessage `json:"inbounds"`
	Volume     int64           `json:"volume"`
	Down       int64           `json:"down"`
	Up         int64           `json:"up"`
	Expiry     int64           `json:"expiry"`
	DelayStart bool            `json:"delay_start"`
	AutoReset  bool            `json:"auto_reset"`
	ResetDays  int             `json:"reset_days"`
	NextReset  string          `json:"next_reset,omitempty"`
	TotalUp    int64           `json:"total_up"`
	TotalDown  int64           `json:"total_down"`
	RequestID  string          `json:"request_id"`
	UpdatedAt  int64           `json:"updated_at"`
}

type ClusterHubDomainOperationState struct {
	ResourceKind string                               `json:"resource_kind"`
	ResourceID   string                               `json:"resource_id"`
	Action       string                               `json:"action"`
	Revision     int64                                `json:"revision"`
	Status       string                               `json:"status"`
	Instances    []ClusterDomainOperationInstanceView `json:"instances"`
}

func (c *ClusterDomainResourceCoordinator) ReportDomainResourceState(ctx context.Context, domain *model.ClusterDomain, op *ClusterDomainOperationView) error {
	if domain == nil || op == nil {
		return nil
	}
	client := c.hubClient()
	if client == nil {
		return nil
	}
	domainToken, err := c.domainToken(domain)
	if err != nil {
		return err
	}
	resources, err := c.buildDomainResources(domain.Id)
	if err != nil {
		return err
	}
	body := ClusterHubResourceStateReportRequest{
		ReportID:         uuid.New().String(),
		OperationID:      op.OperationID,
		ReportedByNodeID: op.OperationID,
		Resources:        resources,
		OperationSummary: ClusterHubDomainOperationState{
			ResourceKind: op.ResourceKind,
			ResourceID:   op.ResourceID,
			Action:       op.Action,
			Revision:     op.Revision,
			Status:       op.Status,
			Instances:    op.Instances,
		},
	}
	if local, err := c.identity().GetOrCreate(); err == nil && local != nil {
		body.ReportedByNodeID = local.NodeID
	}
	return client.ReportDomainResourceState(ctx, domain.HubURL, domain.Domain, domainToken, body)
}

func (c *ClusterDomainResourceCoordinator) buildDomainResources(domainID uint) (ClusterHubDomainResources, error) {
	resources := ClusterHubDomainResources{
		Inbounds: []ClusterHubDomainResourceInbound{},
		Users:    []ClusterHubDomainResourceUser{},
	}
	var wrappers []model.ClusterInbound
	if err := c.db().Model(model.ClusterInbound{}).Preload("Inbound.Tls").Where("domain_id = ?", domainID).Order("id asc").Find(&wrappers).Error; err != nil {
		return resources, err
	}
	for _, wrapper := range wrappers {
		if wrapper.Inbound == nil {
			continue
		}
		status := ClusterDomainOperationApplied
		resources.Inbounds = append(resources.Inbounds, ClusterHubDomainResourceInbound{
			GroupID:     wrapper.GroupID,
			TagSeed:     wrapper.GroupID,
			Prefix:      wrapper.Prefix,
			Suffix:      wrapper.Suffix,
			Type:        wrapper.Inbound.Type,
			TLSTemplate: wrapper.Template,
			OptionsJSON: string(cloneRawMessage(wrapper.Inbound.Options)),
			Status:      "active",
			Instances: []ClusterDomainOperationInstanceView{{
				MemberID:        wrapper.MemberID,
				NodeID:          wrapper.NodeID,
				DisplayName:     wrapper.NodeID,
				TargetTag:       wrapper.Inbound.Tag,
				Status:          status,
				LocalResourceID: wrapper.InboundID,
				UpdatedAt:       wrapper.UpdatedAt,
			}},
		})
	}

	var clients []model.ClusterClient
	if err := c.db().Model(model.ClusterClient{}).Preload("Client").Where("domain_id = ?", domainID).Order("id asc").Find(&clients).Error; err != nil {
		return resources, err
	}
	for _, wrapper := range clients {
		if wrapper.Client == nil {
			continue
		}
		nextReset := ""
		if wrapper.Client.NextReset > 0 {
			nextReset = strconv.FormatInt(wrapper.Client.NextReset, 10)
		}
		resources.Users = append(resources.Users, ClusterHubDomainResourceUser{
			ClientID:   wrapper.ClientID,
			UUID:       wrapper.HubUserUUID,
			Name:       wrapper.Client.Name,
			Enable:     wrapper.Client.Enable,
			Desc:       wrapper.Client.Desc,
			Group:      wrapper.Client.Group,
			SubToken:   domainResourceUserSubToken(wrapper),
			Config:     cloneRawMessage(wrapper.Client.Config),
			Inbounds:   cloneRawMessage(wrapper.Client.Inbounds),
			Volume:     wrapper.Client.Volume,
			Down:       wrapper.Client.Down,
			Up:         wrapper.Client.Up,
			Expiry:     wrapper.Client.Expiry,
			DelayStart: wrapper.Client.DelayStart,
			AutoReset:  wrapper.Client.AutoReset,
			ResetDays:  wrapper.Client.ResetDays,
			NextReset:  nextReset,
			TotalUp:    wrapper.Client.TotalUp,
			TotalDown:  wrapper.Client.TotalDown,
			RequestID:  wrapper.RequestID,
			UpdatedAt:  wrapper.UpdatedAt,
		})
	}
	return resources, nil
}

func domainResourceUserSubToken(wrapper model.ClusterClient) string {
	if wrapper.RequestID != "" {
		return wrapper.RequestID
	}
	return wrapper.HubUserUUID
}

func (c *ClusterDomainResourceCoordinator) domainToken(domain *model.ClusterDomain) (string, error) {
	secret, err := c.secretProvider().GetSecret()
	if err != nil {
		return "", err
	}
	return DecryptClusterDomainToken(secret, domain.TokenEncrypted)
}

func (c *ClusterDomainResourceCoordinator) hubClient() clusterHubClient {
	if c != nil && c.HubClient != nil {
		return c.HubClient
	}
	return &ClusterHubClient{}
}
