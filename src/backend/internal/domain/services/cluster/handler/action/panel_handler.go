package action

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/router"
	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
)

// PanelService mirrors local panel API operations behind cluster actions.
type PanelService interface {
	Load(lu string, hostname string) (map[string]interface{}, error)
	Partial(object string, id string, hostname string) (map[string]interface{}, error)
	Save(object string, act string, data json.RawMessage, initUsers string, hostname string) (map[string]interface{}, error)
	Keypairs(kind string, options string) ([]string, error)
	LinkConvert(link string) (interface{}, error)
	CheckOutbound(tag string, link string) (interface{}, error)
	Stats(resource string, tag string, limit int) (interface{}, error)
}

// PanelHandler handles panel.* actions for remote local-panel management.
type PanelHandler struct {
	svc PanelService
}

func NewPanelHandler(svc PanelService) *PanelHandler {
	return &PanelHandler{svc: svc}
}

func (h *PanelHandler) RegisterAll(r *router.ActionRouter) {
	r.Register("panel.load", h.Load)
	r.Register("panel.partial", h.Partial)
	r.Register("panel.save", h.Save)
	r.Register("panel.keypairs", h.Keypairs)
	r.Register("panel.linkConvert", h.LinkConvert)
	r.Register("panel.checkOutbound", h.CheckOutbound)
	r.Register("panel.stats", h.Stats)
}

func (h *PanelHandler) Load(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	var payload struct {
		LU       json.RawMessage `json:"lu"`
		Hostname string          `json:"hostname"`
	}
	if err := decodePanelPayload(req.Payload, &payload); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid payload: %v", err)}
	}
	lu, err := normalizePanelScalarString(payload.LU)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid lu: %v", err)}
	}
	data, err := h.svc.Load(lu, payload.Hostname)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	return panelSuccess(data), nil
}

func (h *PanelHandler) Partial(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	var payload struct {
		Object   string          `json:"object"`
		ID       json.RawMessage `json:"id"`
		Hostname string          `json:"hostname"`
	}
	if err := decodePanelPayload(req.Payload, &payload); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid payload: %v", err)}
	}
	if strings.TrimSpace(payload.Object) == "" {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: "object is required"}
	}
	id, err := normalizePanelScalarString(payload.ID)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid id: %v", err)}
	}
	data, err := h.svc.Partial(payload.Object, id, payload.Hostname)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	return panelSuccess(data), nil
}

func (h *PanelHandler) Save(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	var payload struct {
		Object    string          `json:"object"`
		Action    string          `json:"action"`
		Data      json.RawMessage `json:"data"`
		InitUsers json.RawMessage `json:"initUsers"`
		Hostname  string          `json:"hostname"`
	}
	if err := decodePanelPayload(req.Payload, &payload); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid payload: %v", err)}
	}
	if strings.TrimSpace(payload.Object) == "" {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: "object is required"}
	}
	if strings.TrimSpace(payload.Action) == "" {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: "action is required"}
	}
	if len(payload.Data) == 0 || string(payload.Data) == "null" {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: "data is required"}
	}

	initUsers, err := normalizePanelInitUsers(payload.InitUsers)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid initUsers: %v", err)}
	}
	data, err := h.svc.Save(payload.Object, payload.Action, payload.Data, initUsers, payload.Hostname)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	return panelSuccess(data), nil
}

func (h *PanelHandler) Keypairs(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	var payload struct {
		K string `json:"k"`
		O string `json:"o"`
	}
	if err := decodePanelPayload(req.Payload, &payload); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid payload: %v", err)}
	}
	data, err := h.svc.Keypairs(payload.K, payload.O)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	return panelSuccess(data), nil
}

func (h *PanelHandler) LinkConvert(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	var payload struct {
		Link string `json:"link"`
	}
	if err := decodePanelPayload(req.Payload, &payload); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid payload: %v", err)}
	}
	data, err := h.svc.LinkConvert(payload.Link)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	return panelSuccess(data), nil
}

func (h *PanelHandler) CheckOutbound(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	var payload struct {
		Tag  string `json:"tag"`
		Link string `json:"link"`
	}
	if err := decodePanelPayload(req.Payload, &payload); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid payload: %v", err)}
	}
	data, err := h.svc.CheckOutbound(payload.Tag, payload.Link)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	return panelSuccess(data), nil
}

func (h *PanelHandler) Stats(ctx context.Context, req clustertypes.ActionRequest) (clustertypes.ActionResponse, error) {
	var payload struct {
		Resource string `json:"resource"`
		Tag      string `json:"tag"`
		Limit    int    `json:"limit"`
	}
	if err := decodePanelPayload(req.Payload, &payload); err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: fmt.Sprintf("invalid payload: %v", err)}
	}
	if payload.Limit == 0 {
		payload.Limit = 100
	}
	data, err := h.svc.Stats(payload.Resource, payload.Tag, payload.Limit)
	if err != nil {
		return clustertypes.ActionResponse{}, router.HandlerError{Message: err.Error()}
	}
	return panelSuccess(data), nil
}

func decodePanelPayload(payload map[string]interface{}, target interface{}) error {
	raw, err := marshalUnmarshalPayload(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func panelSuccess(data interface{}) clustertypes.ActionResponse {
	return clustertypes.ActionResponse{
		Status: "success",
		Data:   data,
	}
}

func normalizePanelScalarString(raw json.RawMessage) (string, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return "", nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString, nil
	}

	var asNumber json.Number
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	if err := decoder.Decode(&asNumber); err == nil {
		return asNumber.String(), nil
	}

	return "", fmt.Errorf("unsupported scalar value %s", text)
}

func normalizePanelInitUsers(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString, nil
	}

	var asNumbers []json.Number
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&asNumbers); err == nil {
		values := make([]string, 0, len(asNumbers))
		for _, item := range asNumbers {
			n, err := item.Int64()
			if err != nil {
				return "", err
			}
			values = append(values, strconv.FormatInt(n, 10))
		}
		return strings.Join(values, ","), nil
	}

	var asInterfaces []interface{}
	if err := json.Unmarshal(raw, &asInterfaces); err != nil {
		return "", err
	}
	values := make([]string, 0, len(asInterfaces))
	for _, item := range asInterfaces {
		switch value := item.(type) {
		case float64:
			values = append(values, strconv.FormatInt(int64(value), 10))
		case string:
			values = append(values, value)
		default:
			return "", fmt.Errorf("unsupported init user value %T", item)
		}
	}
	return strings.Join(values, ","), nil
}
