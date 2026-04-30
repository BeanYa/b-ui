package logger

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/config"
	"github.com/op/go-logging"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	ClusterInbound  = "INBOUND"
	ClusterOutbound = "OUTBOUND"
	ClusterHub      = "HUB"
	ClusterCron     = "CRON"

	clusterLogBufMax = 2048
)

type ClusterLogEntry struct {
	Time      string                 `json:"time"`
	Level     string                 `json:"level"`
	Direction string                 `json:"direction"`
	Action    string                 `json:"action"`
	Fields    map[string]interface{} `json:"fields"`
}

var (
	clusterLogger *logging.Logger
	clusterBufMu  sync.RWMutex
	clusterBuf    []ClusterLogEntry
)

func InitClusterLogger() {
	logPath := filepath.Join(config.GetDBFolderPath(), "cluster.log")
	writer := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,
		MaxBackups: 3,
		Compress:   true,
	}
	backend := logging.NewLogBackend(writer, "", 0)
	format := logging.MustStringFormatter(`[%{time:2006/01/02 15:04:05}] [%{level}]`)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(logging.DEBUG, "cluster")

	l := logging.MustGetLogger("cluster")
	l.SetBackend(backendLeveled)
	clusterLogger = l
}

func ClusterDebug(direction, action string, fields map[string]interface{}) {
	if clusterLogger == nil {
		return
	}
	clusterLogger.Debug(formatClusterEntry(direction, action, fields))
	addClusterBuf("DEBUG", direction, action, fields)
}

func ClusterInfo(direction, action string, fields map[string]interface{}) {
	if clusterLogger == nil {
		return
	}
	clusterLogger.Info(formatClusterEntry(direction, action, fields))
	addClusterBuf("INFO", direction, action, fields)
}

func ClusterWarn(direction, action string, fields map[string]interface{}) {
	if clusterLogger == nil {
		return
	}
	clusterLogger.Warning(formatClusterEntry(direction, action, fields))
	addClusterBuf("WARN", direction, action, fields)
}

func ClusterError(direction, action string, fields map[string]interface{}) {
	if clusterLogger == nil {
		return
	}
	clusterLogger.Error(formatClusterEntry(direction, action, fields))
	addClusterBuf("ERROR", direction, action, fields)
}

func addClusterBuf(level, direction, action string, fields map[string]interface{}) {
	entry := ClusterLogEntry{
		Time:      time.Now().Format("2006/01/02 15:04:05"),
		Level:     level,
		Direction: direction,
		Action:    action,
		Fields:    fields,
	}
	clusterBufMu.Lock()
	if len(clusterBuf) >= clusterLogBufMax {
		clusterBuf = clusterBuf[1:]
	}
	clusterBuf = append(clusterBuf, entry)
	clusterBufMu.Unlock()
}

// GetClusterLogs returns the last count entries filtered by domain name.
// If domain is empty, returns all entries.
func GetClusterLogs(count int, domain string) []ClusterLogEntry {
	clusterBufMu.RLock()
	defer clusterBufMu.RUnlock()

	var result []ClusterLogEntry
	for i := len(clusterBuf) - 1; i >= 0 && len(result) < count; i-- {
		e := clusterBuf[i]
		if domain != "" {
			if dv, ok := e.Fields["domain"]; !ok || fmt.Sprintf("%v", dv) != domain {
				continue
			}
		}
		result = append(result, e)
	}
	return result
}

func PayloadKeys(payload map[string]interface{}) []string {
	if payload == nil {
		return nil
	}
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatClusterEntry(direction, action string, fields map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("[" + direction + "] ")
	sb.WriteString("action=" + action)
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := fields[k]
		if slice, ok := v.([]string); ok {
			sb.WriteString(fmt.Sprintf(" %s=[%s]", k, strings.Join(slice, ",")))
		} else {
			sb.WriteString(fmt.Sprintf(" %s=%v", k, v))
		}
	}
	return sb.String()
}

// ClusterLatency returns a human-readable duration string for logging.
func ClusterLatency(start time.Time) string {
	d := time.Since(start)
	if d < time.Millisecond {
		return fmt.Sprintf("%dus", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Round(time.Millisecond).String()
}
