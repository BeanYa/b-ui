package cluster

import (
	"encoding/json"
	"slices"
	"testing"
)

type runtimeStubPanelService struct{}

func (s *runtimeStubPanelService) Load(string, string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (s *runtimeStubPanelService) Partial(string, string, string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (s *runtimeStubPanelService) Save(string, string, json.RawMessage, string, string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (s *runtimeStubPanelService) Keypairs(string, string) ([]string, error) {
	return []string{}, nil
}

func (s *runtimeStubPanelService) LinkConvert(string) (interface{}, error) {
	return map[string]interface{}{}, nil
}

func (s *runtimeStubPanelService) CheckOutbound(string, string) (interface{}, error) {
	return map[string]interface{}{}, nil
}

func (s *runtimeStubPanelService) Stats(string, string, int) (interface{}, error) {
	return []interface{}{}, nil
}

func TestRuntimeRegistersPanelActions(t *testing.T) {
	rt := NewRuntimeWithPanel(RuntimeListServices{}, &runtimeStubPanelService{})
	actions := rt.InfoResponse().Actions

	expected := []string{
		"panel.load",
		"panel.partial",
		"panel.save",
		"panel.keypairs",
		"panel.linkConvert",
		"panel.checkOutbound",
		"panel.stats",
	}
	for _, want := range expected {
		if !slices.Contains(actions, want) {
			t.Fatalf("expected action %s in %#v", want, actions)
		}
	}
}
