package service

import (
	"encoding/json"
	"testing"
)

func TestSettingsPayloadIncludesTimeLocation(t *testing.T) {
	if !settingsPayloadIncludesTimeLocation(json.RawMessage(`{"timeLocation":"Asia/Tokyo"}`)) {
		t.Fatal("expected timeLocation settings payload to be detected")
	}
	if settingsPayloadIncludesTimeLocation(json.RawMessage(`{"webPort":"2095"}`)) {
		t.Fatal("expected unrelated settings payload to be ignored")
	}
	if settingsPayloadIncludesTimeLocation(json.RawMessage(`{`)) {
		t.Fatal("expected invalid settings payload to be ignored")
	}
}
