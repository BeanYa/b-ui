package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	"gorm.io/gorm"
)

const clusterResourceScopeDomain = "domain"

func annotateClusterManagedClients(tx *gorm.DB, clients []model.Client) error {
	ids := make([]uint, 0, len(clients))
	for _, client := range clients {
		if client.Id > 0 {
			ids = append(ids, client.Id)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	wrappers, err := clusterClientsByClientIDs(tx, ids)
	if err != nil {
		return err
	}
	for i := range clients {
		if wrapper, ok := wrappers[clients[i].Id]; ok {
			clients[i].ClusterManaged = true
			clients[i].ClusterReadOnly = true
			clients[i].ClusterScope = clusterResourceScopeDomain
			clients[i].ClusterDomain = wrapper.Domain
			clients[i].ClusterHubUserUUID = wrapper.HubUserUUID
			clients[i].ClusterRequestID = wrapper.RequestID
		}
	}
	return nil
}

func annotateClusterManagedInboundMaps(tx *gorm.DB, inbounds []map[string]interface{}) error {
	ids := make([]uint, 0, len(inbounds))
	for _, inbound := range inbounds {
		if id := uintFromInterface(inbound["id"]); id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	wrappers, err := clusterInboundsByInboundIDs(tx, ids)
	if err != nil {
		return err
	}
	for _, inbound := range inbounds {
		id := uintFromInterface(inbound["id"])
		if wrapper, ok := wrappers[id]; ok {
			inbound["cluster_managed"] = true
			inbound["cluster_read_only"] = true
			inbound["cluster_scope"] = clusterResourceScopeDomain
			inbound["cluster_domain"] = wrapper.Domain
			inbound["cluster_request_id"] = wrapper.RequestID
		}
	}
	return nil
}

func annotateClusterManagedTLS(tx *gorm.DB, tlsConfigs []model.Tls) error {
	ids := make([]uint, 0, len(tlsConfigs))
	for _, tlsConfig := range tlsConfigs {
		if tlsConfig.Id > 0 {
			ids = append(ids, tlsConfig.Id)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	wrappers, err := clusterInboundsByTLSIDs(tx, ids)
	if err != nil {
		return err
	}
	for i := range tlsConfigs {
		if wrapper, ok := wrappers[tlsConfigs[i].Id]; ok {
			tlsConfigs[i].ClusterManaged = true
			tlsConfigs[i].ClusterReadOnly = true
			tlsConfigs[i].ClusterScope = clusterResourceScopeDomain
			tlsConfigs[i].ClusterDomain = wrapper.Domain
			tlsConfigs[i].ClusterRequestID = wrapper.RequestID
		}
	}
	return nil
}

func rejectClusterManagedClientMutation(tx *gorm.DB, action string, data json.RawMessage) error {
	switch action {
	case "edit":
		var client model.Client
		if err := json.Unmarshal(data, &client); err != nil {
			return err
		}
		return rejectClusterManagedClientIDs(tx, []uint{client.Id})
	case "del":
		var id uint
		if err := json.Unmarshal(data, &id); err != nil {
			return err
		}
		return rejectClusterManagedClientIDs(tx, []uint{id})
	case "editbulk":
		var clients []model.Client
		if err := json.Unmarshal(data, &clients); err != nil {
			return err
		}
		ids := make([]uint, 0, len(clients))
		for _, client := range clients {
			ids = append(ids, client.Id)
		}
		return rejectClusterManagedClientIDs(tx, ids)
	case "delbulk":
		var ids []uint
		if err := json.Unmarshal(data, &ids); err != nil {
			return err
		}
		return rejectClusterManagedClientIDs(tx, ids)
	default:
		return nil
	}
}

func rejectClusterManagedClientIDs(tx *gorm.DB, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	wrappers, err := clusterClientsByClientIDs(tx, ids)
	if err != nil {
		return err
	}
	if len(wrappers) > 0 {
		return errors.New("hub-managed domain user is read-only in this panel")
	}
	return nil
}

func rejectClusterManagedInboundID(tx *gorm.DB, id uint) error {
	if id == 0 {
		return nil
	}
	wrappers, err := clusterInboundsByInboundIDs(tx, []uint{id})
	if err != nil {
		return err
	}
	if len(wrappers) > 0 {
		return errors.New("hub-managed domain inbound is read-only in this panel")
	}
	return nil
}

func rejectClusterManagedInboundTag(tx *gorm.DB, tag string) error {
	if tag == "" {
		return nil
	}
	var inbound model.Inbound
	err := tx.Select("id").Where("tag = ?", tag).First(&inbound).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return rejectClusterManagedInboundID(tx, inbound.Id)
}

func rejectClusterManagedTLSID(tx *gorm.DB, id uint) error {
	if id == 0 {
		return nil
	}
	wrappers, err := clusterInboundsByTLSIDs(tx, []uint{id})
	if err != nil {
		return err
	}
	if len(wrappers) > 0 {
		return errors.New("hub-managed domain TLS is read-only in this panel")
	}
	return nil
}

func clusterClientsByClientIDs(tx *gorm.DB, ids []uint) (map[uint]model.ClusterClient, error) {
	var wrappers []model.ClusterClient
	if err := tx.Where("client_id IN ?", ids).Find(&wrappers).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]model.ClusterClient, len(wrappers))
	for _, wrapper := range wrappers {
		byID[wrapper.ClientID] = wrapper
	}
	return byID, nil
}

func clusterInboundsByInboundIDs(tx *gorm.DB, ids []uint) (map[uint]model.ClusterInbound, error) {
	var wrappers []model.ClusterInbound
	if err := tx.Where("inbound_id IN ?", ids).Find(&wrappers).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]model.ClusterInbound, len(wrappers))
	for _, wrapper := range wrappers {
		byID[wrapper.InboundID] = wrapper
	}
	return byID, nil
}

type clusterTLSInboundRow struct {
	TLSID uint
	model.ClusterInbound
}

func clusterInboundsByTLSIDs(tx *gorm.DB, tlsIDs []uint) (map[uint]model.ClusterInbound, error) {
	var rows []clusterTLSInboundRow
	err := tx.Model(&model.ClusterInbound{}).
		Select("inbounds.tls_id AS tls_id, cluster_inbounds.*").
		Joins("JOIN inbounds ON inbounds.id = cluster_inbounds.inbound_id").
		Where("inbounds.tls_id IN ?", tlsIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]model.ClusterInbound, len(rows))
	for _, row := range rows {
		byID[row.TLSID] = row.ClusterInbound
	}
	return byID, nil
}

func uintFromInterface(value interface{}) uint {
	switch v := value.(type) {
	case uint:
		return v
	case uint64:
		return uint(v)
	case int:
		return uint(v)
	case int64:
		return uint(v)
	case float64:
		return uint(v)
	case json.Number:
		n, _ := v.Int64()
		return uint(n)
	default:
		var id uint
		_, _ = fmt.Sscan(fmt.Sprint(v), &id)
		return id
	}
}
