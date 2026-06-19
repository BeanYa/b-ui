package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveSelf(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","countryCode":"JP","country":"Japan"}`))
	}))
	defer srv.Close()
	svc := NewGeoIPServiceWithURL(&http.Client{}, srv.URL)
	code, name, err := svc.ResolveSelf(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if code != "JP" || name != "Japan" {
		t.Errorf("got code=%q name=%q", code, name)
	}
}

func TestResolveSelfFailureStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"fail"}`))
	}))
	defer srv.Close()
	svc := NewGeoIPServiceWithURL(&http.Client{}, srv.URL)
	_, _, err := svc.ResolveSelf(context.Background())
	if err == nil {
		t.Fatal("expected error on status=fail")
	}
}
