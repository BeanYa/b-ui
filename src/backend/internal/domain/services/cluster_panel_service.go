package service

import (
	"encoding/json"
	"strconv"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster"
	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/handler/action"
	"github.com/BeanYa/b-ui/src/backend/internal/shared/util"
)

// ClusterPanelActionService adapts the local panel service layer for cluster
// panel.* actions.
type ClusterPanelActionService struct {
	ConfigService
	ServerService
	StatsService
}

func (s *ClusterPanelActionService) Load(lu string, hostname string) (map[string]interface{}, error) {
	data := make(map[string]interface{}, 0)
	isUpdated, err := s.ConfigService.CheckChanges(lu)
	if err != nil {
		return nil, err
	}
	onlines, err := s.StatsService.GetOnlines()
	if err != nil {
		return nil, err
	}
	data["onlines"] = onlines

	if !isUpdated {
		return data, nil
	}

	config, err := s.SettingService.GetConfig()
	if err != nil {
		return nil, err
	}
	clients, err := s.ClientService.GetAll()
	if err != nil {
		return nil, err
	}
	tlsConfigs, err := s.TlsService.GetAll()
	if err != nil {
		return nil, err
	}
	inbounds, err := s.InboundService.GetAll()
	if err != nil {
		return nil, err
	}
	outbounds, err := s.OutboundService.GetAll()
	if err != nil {
		return nil, err
	}
	endpoints, err := s.EndpointService.GetAll()
	if err != nil {
		return nil, err
	}
	services, err := s.ServicesService.GetAll()
	if err != nil {
		return nil, err
	}
	subURI, err := s.SettingService.GetFinalSubURI(hostname)
	if err != nil {
		return nil, err
	}
	trafficAge, err := s.SettingService.GetTrafficAge()
	if err != nil {
		return nil, err
	}

	data["config"] = json.RawMessage(config)
	data["clients"] = clients
	data["tls"] = tlsConfigs
	data["inbounds"] = inbounds
	data["outbounds"] = outbounds
	data["endpoints"] = endpoints
	data["services"] = services
	data["subURI"] = subURI
	data["enableTraffic"] = trafficAge > 0
	return data, nil
}

func (s *ClusterPanelActionService) Partial(object string, id string, hostname string) (map[string]interface{}, error) {
	data := make(map[string]interface{}, 0)
	switch object {
	case "inbounds":
		inbounds, err := s.InboundService.Get(id)
		if err != nil {
			return nil, err
		}
		data[object] = inbounds
	case "clients":
		clients, err := s.ClientService.Get(id)
		if err != nil {
			return nil, err
		}
		data[object] = clients
	case "tls":
		tlsConfigs, err := s.TlsService.GetAll()
		if err != nil {
			return nil, err
		}
		data[object] = tlsConfigs
	case "outbounds":
		outbounds, err := s.OutboundService.GetAll()
		if err != nil {
			return nil, err
		}
		data[object] = outbounds
	case "endpoints":
		endpoints, err := s.EndpointService.GetAll()
		if err != nil {
			return nil, err
		}
		data[object] = endpoints
	case "services":
		services, err := s.ServicesService.GetAll()
		if err != nil {
			return nil, err
		}
		data[object] = services
	case "config":
		config, err := s.SettingService.GetConfig()
		if err != nil {
			return nil, err
		}
		data[object] = json.RawMessage(config)
	default:
		return nil, actionUnsupportedObject(object)
	}
	return data, nil
}

func (s *ClusterPanelActionService) Save(object string, act string, data json.RawMessage, initUsers string, hostname string) (map[string]interface{}, error) {
	if _, err := s.ConfigService.Save(object, act, data, initUsers, "ClusterRemotePanel", hostname); err != nil {
		return nil, err
	}
	return s.Load("", hostname)
}

func (s *ClusterPanelActionService) Keypairs(kind string, options string) ([]string, error) {
	return s.ServerService.GenKeypair(kind, options), nil
}

func (s *ClusterPanelActionService) LinkConvert(link string) (interface{}, error) {
	result, _, err := util.GetOutbound(link, 0)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ClusterPanelActionService) CheckOutbound(tag string, link string) (interface{}, error) {
	return s.ConfigService.CheckOutbound(tag, link), nil
}

func (s *ClusterPanelActionService) Stats(resource string, tag string, limit int) (interface{}, error) {
	return s.StatsService.GetStats(resource, tag, limit)
}

func (s *ClusterPanelActionService) ListService(object string) action.ListService {
	return &clusterPanelListService{panel: s, object: object}
}

type clusterPanelListService struct {
	panel  *ClusterPanelActionService
	object string
}

func (s *clusterPanelListService) List(page, pageSize int) ([]map[string]interface{}, int64, error) {
	data, err := s.panel.Partial(s.object, "", "")
	if err != nil {
		return nil, 0, err
	}
	items, err := collectionToMaps(data[s.object])
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(items))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []map[string]interface{}{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}

func collectionToMaps(value interface{}) ([]map[string]interface{}, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func actionUnsupportedObject(object string) error {
	return &panelObjectError{object: object}
}

type panelObjectError struct {
	object string
}

func (e *panelObjectError) Error() string {
	return "unsupported panel object: " + strconv.Quote(e.object)
}

func NewClusterPanelListServices(panel *ClusterPanelActionService) cluster.RuntimeListServices {
	return cluster.RuntimeListServices{
		Inbound:  panel.ListService("inbounds"),
		Client:   panel.ListService("clients"),
		TLS:      panel.ListService("tls"),
		Service:  panel.ListService("services"),
		Outbound: panel.ListService("outbounds"),
	}
}
