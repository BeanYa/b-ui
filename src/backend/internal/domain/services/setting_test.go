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

func TestResolveSubTLSFilesUsesPanelTLSWhenSubscriptionTLSIsBlank(t *testing.T) {
	certFile, keyFile := resolveSubTLSFiles("", "", "/tmp/panel.crt", "/tmp/panel.key")

	if certFile != "/tmp/panel.crt" || keyFile != "/tmp/panel.key" {
		t.Fatalf("expected linked panel TLS files, got cert=%q key=%q", certFile, keyFile)
	}
}

func TestResolveSubTLSFilesUsesCustomTLSOnlyWhenBothSubscriptionPathsAreSet(t *testing.T) {
	certFile, keyFile := resolveSubTLSFiles("/tmp/sub.crt", "/tmp/sub.key", "/tmp/panel.crt", "/tmp/panel.key")

	if certFile != "/tmp/sub.crt" || keyFile != "/tmp/sub.key" {
		t.Fatalf("expected custom subscription TLS files, got cert=%q key=%q", certFile, keyFile)
	}

	certFile, keyFile = resolveSubTLSFiles("/tmp/sub.crt", "", "/tmp/panel.crt", "/tmp/panel.key")
	if certFile != "/tmp/panel.crt" || keyFile != "/tmp/panel.key" {
		t.Fatalf("expected incomplete custom TLS to keep linked panel files, got cert=%q key=%q", certFile, keyFile)
	}
}
