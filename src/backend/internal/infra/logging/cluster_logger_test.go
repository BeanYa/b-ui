package logger

import (
	"bytes"
	"testing"
	"time"

	"github.com/op/go-logging"
)

func TestClusterLogEntryUsesConfiguredPanelTimeLocation(t *testing.T) {
	oldBuf := clusterBuf
	clusterBuf = nil
	t.Cleanup(func() {
		clusterBuf = oldBuf
	})

	tehran, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		t.Fatalf("load panel location: %v", err)
	}
	SetClusterLogLocation(tehran)
	t.Cleanup(func() {
		SetClusterLogLocation(time.Local)
	})

	before := time.Now().In(tehran).Add(-time.Second)
	addClusterBuf("INFO", ClusterInbound, "domain.inbound.delete", map[string]interface{}{
		"domain": "edge.example.com",
	})
	after := time.Now().In(tehran).Add(time.Second)

	logs := GetClusterLogs(1, "edge.example.com")
	if len(logs) != 1 {
		t.Fatalf("expected one log entry, got %d", len(logs))
	}
	got, err := time.ParseInLocation("2006/01/02 15:04:05 Asia/Tehran", logs[0].Time, tehran)
	if err != nil {
		t.Fatalf("expected panel location display time, got %q: %v", logs[0].Time, err)
	}
	if got.Before(before) || got.After(after) {
		t.Fatalf("expected panel location time between %s and %s, got %s", before, after, got)
	}
	if _, err := time.Parse(time.RFC3339, logs[0].TimeUTC); err != nil {
		t.Fatalf("expected UTC reference time, got %q: %v", logs[0].TimeUTC, err)
	}
}

func TestClusterLogFormatterIncludesConfiguredTimeAndMessage(t *testing.T) {
	tehran, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		t.Fatalf("load panel location: %v", err)
	}
	SetClusterLogLocation(tehran)
	t.Cleanup(func() {
		SetClusterLogLocation(time.Local)
	})

	record := &logging.Record{
		Time:  time.Date(2026, 5, 4, 10, 57, 3, 0, time.UTC),
		Level: logging.INFO,
		Args:  []interface{}{"[OUTBOUND] action=domain.inbound.delete status=ok"},
	}
	var out bytes.Buffer
	if err := (clusterLogFormatter{}).Format(0, record, &out); err != nil {
		t.Fatalf("format cluster log: %v", err)
	}

	want := "[2026/05/04 14:27:03 Asia/Tehran] [INFO] [OUTBOUND] action=domain.inbound.delete status=ok"
	if got := out.String(); got != want {
		t.Fatalf("unexpected formatted log:\nwant: %q\n got: %q", want, got)
	}
}
