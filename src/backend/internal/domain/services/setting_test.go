package service

import (
	"reflect"
	"testing"
)

func TestSettingServiceExposesTerminalIdleTimeoutGetter(t *testing.T) {
	service := &SettingService{}
	getter := reflect.ValueOf(service).MethodByName("GetWebTerminalIdleTimeout")
	if !getter.IsValid() {
		t.Fatal("expected terminal idle timeout getter to exist")
	}
	if defaultValueMap["webTerminalIdleTimeout"] != "300" {
		t.Fatalf("expected terminal idle timeout default to be 300 seconds, got %q", defaultValueMap["webTerminalIdleTimeout"])
	}
	if _, exists := defaultValueMap["webSSHIdleTimeout"]; exists {
		t.Fatal("expected legacy webssh idle timeout key to be removed from defaults")
	}
}
