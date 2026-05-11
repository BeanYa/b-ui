package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	"github.com/google/uuid"
)

type ClusterHubDomainResources struct {
	Inbounds []ClusterHubDomainResourceInbound `json:"domain_inbounds"`
	Users    []ClusterHubDomainResourceUser    `json:"domain_users"`
}

type ClusterHubDomainResourceInbound struct {
	GroupID             string                             `json:"group_id"`
	TagSeed             string                             `json:"tag_seed,omitempty"`
	Prefix              string                             `json:"prefix,omitempty"`
	Suffix              string                             `json:"suffix,omitempty"`
	Type                string                             `json:"type"`
	TLSTemplate         string                             `json:"tls_template,omitempty"`
	OptionsJSON         string                             `json:"options_json,omitempty"`
	Status              string                             `json:"status,omitempty"`
	Revision            int64                              `json:"revision,omitempty"`
	LastOperationID     string                             `json:"last_operation_id,omitempty"`
	LastOperationStatus string                             `json:"last_operation_status,omitempty"`
	Instances           []ClusterHubDomainResourceInstance `json:"instances,omitempty"`
}

type ClusterHubDomainResourceInstance struct {
	MemberID        string `json:"member_id,omitempty"`
	NodeID          string `json:"node_id"`
	DisplayName     string `json:"display_name,omitempty"`
	TargetTag       string `json:"target_tag,omitempty"`
	Status          string `json:"status"`
	AttemptCount    int    `json:"attempt_count"`
	LocalResourceID uint   `json:"local_resource_id,omitempty"`
	Error           string `json:"error,omitempty"`
	UpdatedAt       int64  `json:"updated_at,omitempty"`
}

type ClusterHubDomainResourceUser struct {
	ClientID             uint            `json:"client_id"`
	NodeID               string          `json:"node_id,omitempty"`
	UUID                 string          `json:"uuid"`
	Name                 string          `json:"name"`
	Enable               bool            `json:"enable"`
	Desc                 string          `json:"desc,omitempty"`
	Group                string          `json:"group,omitempty"`
	SubToken             string          `json:"sub_token"`
	Config               json.RawMessage `json:"config"`
	Links                json.RawMessage `json:"links,omitempty"`
	Inbounds             json.RawMessage `json:"inbounds,omitempty"`
	BoundInboundGroupIDs []string        `json:"bound_inbound_group_ids,omitempty"`
	Volume               int64           `json:"volume"`
	Down                 int64           `json:"down"`
	Up                   int64           `json:"up"`
	Expiry               int64           `json:"expiry"`
	DelayStart           bool            `json:"delay_start"`
	AutoReset            bool            `json:"auto_reset"`
	ResetDays            int             `json:"reset_days"`
	NextReset            string          `json:"next_reset,omitempty"`
	TotalUp              int64           `json:"total_up"`
	TotalDown            int64           `json:"total_down"`
	RequestID            string          `json:"request_id"`
	UpdatedAt            int64           `json:"updated_at"`
}

type ClusterHubDomainOperationState struct {
	ResourceKind string                             `json:"resource_kind"`
	ResourceID   string                             `json:"resource_id"`
	Action       string                             `json:"action"`
	Revision     int64                              `json:"revision"`
	Status       string                             `json:"status"`
	Instances    []ClusterHubDomainResourceInstance `json:"instances"`
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
			Instances:    clusterHubDomainResourceInstances(op.Instances),
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
	inboundIndexes := map[string]int{}
	for _, wrapper := range wrappers {
		if wrapper.Inbound == nil {
			continue
		}
		status := ClusterDomainOperationApplied
		inboundIndexes[wrapper.GroupID] = len(resources.Inbounds)
		resources.Inbounds = append(resources.Inbounds, ClusterHubDomainResourceInbound{
			GroupID:     wrapper.GroupID,
			TagSeed:     wrapper.GroupID,
			Prefix:      wrapper.Prefix,
			Suffix:      wrapper.Suffix,
			Type:        wrapper.Inbound.Type,
			TLSTemplate: wrapper.Template,
			OptionsJSON: materializedDomainInboundOptionsJSON(wrapper.Inbound),
			Status:      "active",
			Instances: []ClusterHubDomainResourceInstance{{
				MemberID:        wrapper.MemberID,
				NodeID:          wrapper.NodeID,
				DisplayName:     wrapper.NodeID,
				TargetTag:       wrapper.Inbound.Tag,
				Status:          status,
				AttemptCount:    1,
				LocalResourceID: wrapper.InboundID,
				UpdatedAt:       wrapper.UpdatedAt,
			}},
		})
	}
	if err := c.mergeDomainInboundOperationResources(domainID, &resources, inboundIndexes); err != nil {
		return resources, err
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
		boundGroups := decodeDomainUserBoundGroups(wrapper.BoundInboundGroupIDs)
		resources.Users = append(resources.Users, ClusterHubDomainResourceUser{
			ClientID:             wrapper.ClientID,
			NodeID:               wrapper.NodeID,
			UUID:                 wrapper.HubUserUUID,
			Name:                 wrapper.Client.Name,
			Enable:               wrapper.Client.Enable,
			Desc:                 wrapper.Client.Desc,
			Group:                wrapper.Client.Group,
			SubToken:             domainResourceUserSubToken(wrapper),
			Config:               cloneRawMessage(wrapper.Client.Config),
			Links:                nonLocalDomainUserLinks(wrapper.Client.Links),
			Inbounds:             domainUserGroupSelectorsRaw(boundGroups),
			BoundInboundGroupIDs: boundGroups,
			Volume:               wrapper.Client.Volume,
			Down:                 wrapper.Client.Down,
			Up:                   wrapper.Client.Up,
			Expiry:               wrapper.Client.Expiry,
			DelayStart:           wrapper.Client.DelayStart,
			AutoReset:            wrapper.Client.AutoReset,
			ResetDays:            wrapper.Client.ResetDays,
			NextReset:            nextReset,
			TotalUp:              wrapper.Client.TotalUp,
			TotalDown:            wrapper.Client.TotalDown,
			RequestID:            wrapper.RequestID,
			UpdatedAt:            wrapper.UpdatedAt,
		})
	}
	if err := c.mergeDomainUserOperationResources(domainID, &resources); err != nil {
		return resources, err
	}
	return resources, nil
}

func (c *ClusterDomainResourceCoordinator) mergeDomainInboundOperationResources(domainID uint, resources *ClusterHubDomainResources, inboundIndexes map[string]int) error {
	if resources == nil {
		return nil
	}
	if inboundIndexes == nil {
		inboundIndexes = map[string]int{}
	}
	var ops []model.ClusterDomainOperation
	if err := c.db().Where("domain_id = ? AND resource_kind = ?", domainID, ClusterDomainResourceInbound).Order("id asc").Find(&ops).Error; err != nil {
		return err
	}
	store := c.operationStore()
	for _, op := range ops {
		if op.Action == ClusterDomainOperationDelete {
			c.mergeDomainInboundDeleteOperationResource(op, resources, inboundIndexes, store)
			continue
		}
		resource, ok := domainInboundResourceFromOperation(op, store)
		if !ok {
			continue
		}
		if idx, exists := inboundIndexes[resource.GroupID]; exists {
			applyDomainInboundOperationResource(&resources.Inbounds[idx], resource)
			continue
		}
		if !domainInboundResourceMaterialized(resource) {
			continue
		}
		inboundIndexes[resource.GroupID] = len(resources.Inbounds)
		resources.Inbounds = append(resources.Inbounds, resource)
	}
	return nil
}

func applyDomainInboundOperationResource(existing *ClusterHubDomainResourceInbound, resource ClusterHubDomainResourceInbound) {
	if existing == nil {
		return
	}
	existing.LastOperationID = resource.LastOperationID
	existing.LastOperationStatus = resource.LastOperationStatus
	if domainInboundResourceMaterialized(resource) {
		existing.TagSeed = resource.TagSeed
		existing.Prefix = resource.Prefix
		existing.Suffix = resource.Suffix
		existing.Type = resource.Type
		existing.TLSTemplate = resource.TLSTemplate
		existing.OptionsJSON = resource.OptionsJSON
	}
	if len(resource.Instances) > 0 {
		existing.Instances = mergeClusterDomainOperationInstanceViews(existing.Instances, resource.Instances)
	}
}

func (c *ClusterDomainResourceCoordinator) mergeDomainInboundDeleteOperationResource(op model.ClusterDomainOperation, resources *ClusterHubDomainResources, inboundIndexes map[string]int, store *ClusterDomainOperationStore) {
	groupID := domainInboundOperationGroupID(op)
	if groupID == "" {
		return
	}
	idx, exists := inboundIndexes[groupID]
	if !exists {
		return
	}
	instances := []ClusterHubDomainResourceInstance{}
	if store != nil {
		if operationInstances, err := store.ListInstances(op.OperationID); err == nil {
			instances = clusterDomainOperationInstanceViews(operationInstances)
		}
	}
	if op.Status == ClusterDomainOperationApplied || op.Status == ClusterDomainOperationSkipped {
		resources.Inbounds = append(resources.Inbounds[:idx], resources.Inbounds[idx+1:]...)
		delete(inboundIndexes, groupID)
		for i, inbound := range resources.Inbounds {
			inboundIndexes[inbound.GroupID] = i
		}
		return
	}
	resources.Inbounds[idx].Status = statusForDomainInboundOperation(op.Status)
	resources.Inbounds[idx].LastOperationID = op.OperationID
	resources.Inbounds[idx].LastOperationStatus = op.Status
	if len(instances) > 0 {
		resources.Inbounds[idx].Instances = instances
	}
}

func domainInboundResourceFromOperation(op model.ClusterDomainOperation, store *ClusterDomainOperationStore) (ClusterHubDomainResourceInbound, bool) {
	groupID := domainInboundOperationGroupID(op)
	if groupID == "" {
		return ClusterHubDomainResourceInbound{}, false
	}
	if op.Action == ClusterDomainOperationDelete {
		return ClusterHubDomainResourceInbound{}, false
	}
	var payload clustertypes.DomainInboundCreatePayload
	if err := json.Unmarshal(op.DesiredPayload, &payload); err != nil {
		return ClusterHubDomainResourceInbound{}, false
	}
	if strings.TrimSpace(payload.GroupID) != "" {
		groupID = strings.TrimSpace(payload.GroupID)
	}
	resource := ClusterHubDomainResourceInbound{
		GroupID:             groupID,
		TagSeed:             strings.TrimSpace(payload.TagSeed),
		Prefix:              strings.TrimSpace(payload.Prefix),
		Suffix:              strings.TrimSpace(payload.Suffix),
		TLSTemplate:         strings.TrimSpace(payload.TLSTemplate),
		Status:              statusForDomainInboundOperation(op.Status),
		Revision:            op.Revision,
		LastOperationID:     op.OperationID,
		LastOperationStatus: op.Status,
	}
	if inboundType, optionsJSON, ok := desiredDomainInboundOptions(payload.Inbound); ok {
		resource.Type = strings.TrimSpace(inboundType)
		resource.OptionsJSON = optionsJSON
	}
	if resource.Type == "" {
		resource.Type = "unknown"
	}
	if resource.TagSeed == "" {
		resource.TagSeed = groupID
	}
	if store != nil {
		if instances, err := store.ListInstances(op.OperationID); err == nil {
			resource.Instances = clusterDomainOperationInstanceViews(instances)
		}
	}
	return resource, true
}

func desiredDomainInboundOptions(raw json.RawMessage) (string, string, bool) {
	if len(raw) == 0 {
		return "", "", false
	}
	var inbound map[string]json.RawMessage
	if err := json.Unmarshal(raw, &inbound); err != nil {
		return "", "", false
	}
	var inboundType string
	_ = json.Unmarshal(inbound["type"], &inboundType)
	if options := strings.TrimSpace(string(inbound["options"])); options != "" && options != "null" {
		return inboundType, options, true
	}
	for _, key := range []string{"id", "type", "tag", "tls_id", "tls", "users", "options"} {
		delete(inbound, key)
	}
	optionsJSON, err := json.Marshal(inbound)
	if err != nil {
		return inboundType, "", true
	}
	return inboundType, string(optionsJSON), true
}

func materializedDomainInboundOptionsJSON(inbound *model.Inbound) string {
	if inbound == nil {
		return ""
	}
	options := map[string]json.RawMessage{}
	if len(inbound.Options) > 0 && strings.TrimSpace(string(inbound.Options)) != "null" {
		_ = json.Unmarshal(inbound.Options, &options)
	}
	if raw := cloneRawMessage(inbound.OutJson); len(raw) > 0 && strings.TrimSpace(string(raw)) != "null" {
		options["out_json"] = raw
	}
	if raw := cloneRawMessage(inbound.Addrs); len(raw) > 0 && strings.TrimSpace(string(raw)) != "null" {
		options["addrs"] = raw
	}
	data, err := json.Marshal(options)
	if err != nil {
		return string(cloneRawMessage(inbound.Options))
	}
	return string(data)
}

func domainInboundResourceMaterialized(resource ClusterHubDomainResourceInbound) bool {
	switch resource.LastOperationStatus {
	case ClusterDomainOperationApplied, ClusterDomainOperationPartial, ClusterDomainOperationReported:
		return true
	}
	for _, instance := range resource.Instances {
		if instance.Status == ClusterDomainOperationApplied {
			return true
		}
	}
	return false
}

func domainInboundOperationGroupID(op model.ClusterDomainOperation) string {
	groupID := strings.TrimSpace(op.ResourceID)
	switch op.Action {
	case ClusterDomainOperationDelete:
		var payload clustertypes.DomainInboundDeletePayload
		if len(op.DesiredPayload) > 0 && json.Unmarshal(op.DesiredPayload, &payload) == nil && strings.TrimSpace(payload.GroupID) != "" {
			groupID = strings.TrimSpace(payload.GroupID)
		}
	default:
		var payload clustertypes.DomainInboundCreatePayload
		if len(op.DesiredPayload) > 0 && json.Unmarshal(op.DesiredPayload, &payload) == nil && strings.TrimSpace(payload.GroupID) != "" {
			groupID = strings.TrimSpace(payload.GroupID)
		}
	}
	return groupID
}

func statusForDomainInboundOperation(status string) string {
	switch status {
	case ClusterDomainOperationApplied:
		return "active"
	case ClusterDomainOperationFailed, ClusterDomainOperationTimeout:
		return "error"
	case ClusterDomainOperationPartial:
		return "partial"
	case ClusterDomainOperationSkipped:
		return "skipped"
	default:
		return status
	}
}

func clusterDomainOperationInstanceViews(instances []model.ClusterDomainOperationInstance) []ClusterHubDomainResourceInstance {
	views := make([]ClusterHubDomainResourceInstance, 0, len(instances))
	for _, instance := range instances {
		views = append(views, ClusterHubDomainResourceInstance{
			MemberID:        instance.MemberID,
			NodeID:          instance.NodeID,
			DisplayName:     instance.DisplayName,
			TargetTag:       instance.TargetTag,
			Status:          instance.Status,
			AttemptCount:    instance.AttemptCount,
			LocalResourceID: instance.LocalResourceID,
			Error:           instance.Error,
			UpdatedAt:       instance.UpdatedAt,
		})
	}
	return views
}

func clusterHubDomainResourceInstances(instances []ClusterDomainOperationInstanceView) []ClusterHubDomainResourceInstance {
	views := make([]ClusterHubDomainResourceInstance, 0, len(instances))
	for _, instance := range instances {
		views = append(views, ClusterHubDomainResourceInstance{
			MemberID:        instance.MemberID,
			NodeID:          instance.NodeID,
			DisplayName:     instance.DisplayName,
			TargetTag:       instance.TargetTag,
			Status:          instance.Status,
			AttemptCount:    instance.AttemptCount,
			LocalResourceID: instance.LocalResourceID,
			Error:           instance.Error,
			UpdatedAt:       instance.UpdatedAt,
		})
	}
	return views
}

func mergeClusterDomainOperationInstanceViews(existing []ClusterHubDomainResourceInstance, updates []ClusterHubDomainResourceInstance) []ClusterHubDomainResourceInstance {
	merged := append([]ClusterHubDomainResourceInstance{}, existing...)
	indexes := map[string]int{}
	for i, instance := range merged {
		if instance.NodeID != "" {
			indexes[instance.NodeID] = i
		}
	}
	for _, update := range updates {
		if update.NodeID != "" {
			if idx, exists := indexes[update.NodeID]; exists {
				merged[idx] = update
				continue
			}
			indexes[update.NodeID] = len(merged)
		}
		merged = append(merged, update)
	}
	return merged
}

func (c *ClusterDomainResourceCoordinator) mergeDomainUserOperationResources(domainID uint, resources *ClusterHubDomainResources) error {
	if resources == nil {
		return nil
	}
	userIndexes := map[string]int{}
	for i, user := range resources.Users {
		if key := domainUserResourceKey(user.UUID, user.NodeID); key != "" {
			userIndexes[key] = i
		}
	}

	var ops []model.ClusterDomainOperation
	if err := c.db().Where("domain_id = ? AND resource_kind = ?", domainID, ClusterDomainResourceUser).Order("id asc").Find(&ops).Error; err != nil {
		return err
	}
	store := c.operationStore()
	for _, op := range ops {
		switch op.Action {
		case ClusterDomainOperationDelete:
			mergeDomainUserDeleteOperationResource(op, resources, userIndexes, store)
		case ClusterDomainOperationCreate, ClusterDomainOperationUpdate:
			mergeDomainUserUpsertOperationResource(op, resources, userIndexes, store)
		}
	}
	return nil
}

func mergeDomainUserUpsertOperationResource(op model.ClusterDomainOperation, resources *ClusterHubDomainResources, userIndexes map[string]int, store *ClusterDomainOperationStore) {
	var payload clustertypes.DomainUserUpsertPayload
	if len(op.DesiredPayload) == 0 || json.Unmarshal(op.DesiredPayload, &payload) != nil || strings.TrimSpace(payload.User.UUID) == "" {
		return
	}
	if store == nil {
		return
	}
	instances, err := store.ListInstances(op.OperationID)
	if err != nil {
		return
	}
	for _, instance := range instances {
		mergeDomainUserUpsertOperationInstance(instance, payload, resources, userIndexes)
	}
}

func mergeDomainUserUpsertOperationInstance(instance model.ClusterDomainOperationInstance, payload clustertypes.DomainUserUpsertPayload, resources *ClusterHubDomainResources, userIndexes map[string]int) {
	if resources == nil || instance.Status != ClusterDomainOperationApplied {
		return
	}
	var result clustertypes.DomainResourceCommandResult
	if rawResponse := strings.TrimSpace(string(instance.ResponseJSON)); rawResponse != "" && rawResponse != "null" && json.Unmarshal(instance.ResponseJSON, &result) != nil {
		return
	}
	nodeID := strings.TrimSpace(result.NodeID)
	if nodeID == "" {
		nodeID = strings.TrimSpace(instance.NodeID)
	}
	if nodeID == "" {
		return
	}
	resource := domainUserResourceFromOperation(payload, instance, result, nodeID)
	key := domainUserResourceKey(resource.UUID, resource.NodeID)
	if key == "" {
		return
	}
	if userIndexes == nil {
		userIndexes = map[string]int{}
	}
	if idx, exists := userIndexes[key]; exists {
		resources.Users[idx] = resource
		return
	}
	userIndexes[key] = len(resources.Users)
	resources.Users = append(resources.Users, resource)
}

func domainUserResourceFromOperation(payload clustertypes.DomainUserUpsertPayload, instance model.ClusterDomainOperationInstance, result clustertypes.DomainResourceCommandResult, nodeID string) ClusterHubDomainResourceUser {
	nextReset := ""
	if payload.User.NextReset > 0 {
		nextReset = strconv.FormatInt(payload.User.NextReset, 10)
	}
	clientID := result.LocalResourceID
	if clientID == 0 {
		clientID = instance.LocalResourceID
	}
	subToken := strings.TrimSpace(result.SubToken)
	if subToken == "" {
		subToken = strings.TrimSpace(payload.User.SubToken)
	}
	if subToken == "" {
		subToken = strings.TrimSpace(payload.RequestID)
	}
	config := result.UserConfig
	if len(config) == 0 {
		config = payload.User.Config
	}
	return ClusterHubDomainResourceUser{
		ClientID:             clientID,
		NodeID:               nodeID,
		UUID:                 payload.User.UUID,
		Name:                 payload.User.Name,
		Enable:               payload.User.Enable,
		Desc:                 payload.User.Desc,
		Group:                payload.User.Group,
		SubToken:             subToken,
		Config:               cloneRawMessage(config),
		Links:                nonLocalDomainUserLinks(payload.User.Links),
		Inbounds:             domainUserGroupSelectorsRaw(payload.User.BoundInboundGroupIDs),
		BoundInboundGroupIDs: normalizeDomainUserBoundGroups(payload.User.BoundInboundGroupIDs, payload.Inbounds),
		Volume:               payload.User.Volume,
		Down:                 payload.User.Down,
		Up:                   payload.User.Up,
		Expiry:               payload.User.Expiry,
		DelayStart:           payload.User.DelayStart,
		AutoReset:            payload.User.AutoReset,
		ResetDays:            payload.User.ResetDays,
		NextReset:            nextReset,
		TotalUp:              payload.User.TotalUp,
		TotalDown:            payload.User.TotalDown,
		RequestID:            payload.RequestID,
		UpdatedAt:            instance.UpdatedAt,
	}
}

func mergeDomainUserDeleteOperationResource(op model.ClusterDomainOperation, resources *ClusterHubDomainResources, userIndexes map[string]int, store *ClusterDomainOperationStore) {
	var payload clustertypes.DomainUserDeletePayload
	userUUID := strings.TrimSpace(op.ResourceID)
	if len(op.DesiredPayload) > 0 && json.Unmarshal(op.DesiredPayload, &payload) == nil && strings.TrimSpace(payload.UUID) != "" {
		userUUID = strings.TrimSpace(payload.UUID)
	}
	if userUUID == "" {
		return
	}
	if op.Status != ClusterDomainOperationApplied && op.Status != ClusterDomainOperationSkipped {
		return
	}
	nodes := map[string]struct{}{}
	if store != nil {
		if instances, err := store.ListInstances(op.OperationID); err == nil {
			for _, instance := range instances {
				if instance.Status == ClusterDomainOperationApplied {
					nodes[strings.TrimSpace(instance.NodeID)] = struct{}{}
				}
			}
		}
	}
	if len(nodes) == 0 {
		for key := range userIndexes {
			if strings.HasPrefix(key, userUUID+"\x00") {
				deleteDomainUserResourceByKey(resources, userIndexes, key)
			}
		}
		return
	}
	for nodeID := range nodes {
		deleteDomainUserResourceByKey(resources, userIndexes, domainUserResourceKey(userUUID, nodeID))
	}
}

func deleteDomainUserResourceByKey(resources *ClusterHubDomainResources, userIndexes map[string]int, key string) {
	idx, exists := userIndexes[key]
	if !exists || resources == nil || idx < 0 || idx >= len(resources.Users) {
		return
	}
	resources.Users = append(resources.Users[:idx], resources.Users[idx+1:]...)
	for k := range userIndexes {
		delete(userIndexes, k)
	}
	for i, user := range resources.Users {
		if nextKey := domainUserResourceKey(user.UUID, user.NodeID); nextKey != "" {
			userIndexes[nextKey] = i
		}
	}
}

func domainUserResourceKey(userUUID string, nodeID string) string {
	userUUID = strings.TrimSpace(userUUID)
	nodeID = strings.TrimSpace(nodeID)
	if userUUID == "" || nodeID == "" {
		return ""
	}
	return userUUID + "\x00" + nodeID
}

func domainResourceUserSubToken(wrapper model.ClusterClient) string {
	if wrapper.SubToken != "" {
		return wrapper.SubToken
	}
	if wrapper.RequestID != "" {
		return wrapper.RequestID
	}
	return wrapper.HubUserUUID
}

func nonLocalDomainUserLinks(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var links []map[string]string
	if err := json.Unmarshal(raw, &links); err != nil {
		return nil
	}
	filtered := make([]map[string]string, 0, len(links))
	seen := map[string]struct{}{}
	for _, link := range links {
		linkType := strings.TrimSpace(link["type"])
		uri := strings.TrimSpace(link["uri"])
		if uri == "" || (linkType != "external" && linkType != "sub") {
			continue
		}
		key := linkType + "\x00" + uri
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		next := map[string]string{
			"type": linkType,
			"uri":  uri,
		}
		if remark := strings.TrimSpace(link["remark"]); remark != "" {
			next["remark"] = remark
		}
		filtered = append(filtered, next)
	}
	if len(filtered) == 0 {
		return json.RawMessage(`[]`)
	}
	out, err := json.Marshal(filtered)
	if err != nil {
		return nil
	}
	return out
}

func decodeDomainUserBoundGroups(raw json.RawMessage) []string {
	var groups []string
	if len(raw) == 0 || json.Unmarshal(raw, &groups) != nil {
		return nil
	}
	return normalizeDomainUserBoundGroups(groups, nil)
}

func domainUserGroupSelectorsRaw(groups []string) json.RawMessage {
	selectors := domainUserGroupSelectors(groups)
	if len(selectors) == 0 {
		return nil
	}
	raw, err := json.Marshal(selectors)
	if err != nil {
		return nil
	}
	return raw
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
