package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"

	"github.com/gin-gonic/gin"
)

type clusterAPIService interface {
	Register(service.ClusterRegisterRequest) (*service.ClusterOperationStatus, error)
	GetOperation(string) (*service.ClusterOperationStatus, error)
	ListDomains() ([]service.ClusterDomainResponse, error)
	ListMembers() ([]service.ClusterMemberResponse, error)
	GetMemberConnection(string) (*service.ClusterMemberConnectionResponse, error)
	GetMemberInfo(string) (*clustertypes.InfoResponse, error)
	SendMemberAction(string, clustertypes.ActionRequest) (*clustertypes.ActionResponse, error)
	ManualSync() (*service.ClusterOperationStatus, error)
	CheckDomainPanelUpdate(uint) (*service.ClusterPanelUpdateCheckResult, error)
	RequestMemberPanelUpdate(uint, string) (*service.ClusterPanelMemberUpdateResult, error)
	DeleteMember(uint) error
	LeaveDomain(uint) error
	ReceivePeerMessage(*service.PeerMessage, string) error
	ReceiveMessage(*service.ClusterEnvelope, string) error
	Heartbeat(remoteNodeID string, token string) (*service.ClusterPeerStatus, error)
	Ping(remoteNodeID string, token string) (*service.ClusterPeerStatus, error)
	HandleAction(c *gin.Context)
	Info(c *gin.Context)
}

func (a *APIHandler) requireClusterAdmin(c *gin.Context) bool {
	username := GetLoginUser(c)
	isAdmin, err := a.ApiService.getUserService().IsFirstUser(username)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, Msg{Success: false, Msg: err.Error()})
		return false
	}
	if !isAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, Msg{Success: false, Msg: "admin access required"})
		return false
	}
	return true
}

func (a *APIHandler) registerCluster(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	var request service.ClusterRegisterRequest
	if err := c.ShouldBind(&request); err != nil {
		jsonMsg(c, "cluster register", err)
		return
	}
	if err := service.NormalizeClusterRegisterRequest(&request); err != nil {
		jsonMsg(c, "cluster register", err)
		return
	}
	status, err := a.clusterService.Register(request)
	jsonObj(c, status, err)
}

func (a *APIHandler) getClusterOperation(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	status, err := a.clusterService.GetOperation(c.Param("id"))
	jsonObj(c, status, err)
}

func (a *APIHandler) listClusterDomains(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	domains, err := a.clusterService.ListDomains()
	jsonObj(c, domains, err)
}

func (a *APIHandler) listClusterMembers(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	members, err := a.clusterService.ListMembers()
	jsonObj(c, members, err)
}

func (a *APIHandler) getClusterMemberConnection(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	nodeID := strings.TrimSpace(c.Query("node_id"))
	connection, err := a.clusterService.GetMemberConnection(nodeID)
	if err == nil && connection != nil {
		connection.Token = ""
	}
	jsonObj(c, connection, err)
}

func (a *APIHandler) getClusterMemberInfo(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	nodeID := strings.TrimSpace(c.Query("node_id"))
	info, err := a.clusterService.GetMemberInfo(nodeID)
	jsonObj(c, info, err)
}

func (a *APIHandler) sendClusterMemberAction(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	var request service.ClusterMemberActionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonMsg(c, "cluster member action", err)
		return
	}
	response, err := a.clusterService.SendMemberAction(strings.TrimSpace(request.NodeID), request.Request)
	jsonObj(c, response, err)
}

func (a *APIHandler) manualClusterSync(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	status, err := a.clusterService.ManualSync()
	jsonObj(c, status, err)
}

func (a *APIHandler) checkClusterDomainPanelUpdate(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonMsg(c, "cluster domain update check", err)
		return
	}
	result, err := a.clusterService.CheckDomainPanelUpdate(uint(id))
	jsonObj(c, result, err)
}

func (a *APIHandler) requestClusterMemberPanelUpdate(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonMsg(c, "cluster member panel update", err)
		return
	}
	var request struct {
		TargetVersion string `json:"targetVersion"`
	}
	if err := c.ShouldBindJSON(&request); err != nil && err.Error() != "EOF" {
		jsonMsg(c, "cluster member panel update", err)
		return
	}
	result, err := a.clusterService.RequestMemberPanelUpdate(uint(id), strings.TrimSpace(request.TargetVersion))
	jsonObj(c, result, err)
}

func (a *APIHandler) deleteClusterMember(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonMsg(c, "delete cluster member", err)
		return
	}
	jsonMsg(c, "delete cluster member", a.clusterService.DeleteMember(uint(id)))
}

func (a *APIHandler) leaveClusterDomain(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonMsg(c, "leave cluster domain", err)
		return
	}
	jsonMsg(c, "leave cluster domain", a.clusterService.LeaveDomain(uint(id)))
}

func (a *APIHandler) getClusterLogs(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	domainIDStr := c.Query("domain_id")
	count := 200
	if c := c.Query("count"); c != "" {
		if n, err := strconv.Atoi(c); err == nil && n > 0 {
			count = n
		}
	}
	if count > 200 {
		count = 200
	}

	var domainName string
	if domainIDStr != "" {
		id, err := strconv.ParseUint(domainIDStr, 10, 64)
		if err != nil {
			jsonMsg(c, "cluster logs", err)
			return
		}
		domains, err := a.clusterService.ListDomains()
		if err != nil {
			jsonMsg(c, "cluster logs", err)
			return
		}
		for _, d := range domains {
			if d.ID == uint(id) {
				domainName = d.Domain
				break
			}
		}
	}

	logs := logger.GetClusterLogs(count, domainName)
	jsonObj(c, logs, nil)
}

const maxClusterMessageBytes = 1 << 20

func RegisterClusterMessageRoute(router gin.IRoutes, clusterService clusterAPIService) {
	router.POST(ClusterMessagePath("/"), func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxClusterMessageBytes)
		body, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "cluster message: " + err.Error()})
			return
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil {
			c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "cluster message: " + err.Error()})
			return
		}
		token := c.GetHeader("X-Cluster-Token")
		remoteIP := c.ClientIP()
		if _, ok := fields["protocolVersion"]; ok {
			var message service.PeerMessage
			if err := json.Unmarshal(body, &message); err != nil {
				c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "cluster message: " + err.Error()})
				return
			}
			logger.ClusterInfo(logger.ClusterInbound, message.Action, map[string]interface{}{
				"msg_type":     "PeerMessage",
				"category":     message.Category,
				"sourceNode":   message.SourceNodeID,
				"domain":       message.DomainID,
				"route":        message.Route.Mode,
				"remote_ip":    remoteIP,
				"payload_keys": logger.PayloadKeys(message.Payload),
			})
			err = clusterService.ReceivePeerMessage(&message, token)
		} else {
			var envelope service.ClusterEnvelope
			if err := json.Unmarshal(body, &envelope); err != nil {
				c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "cluster message: " + err.Error()})
				return
			}
			logger.ClusterInfo(logger.ClusterInbound, "sync.notify_version", map[string]interface{}{
				"msg_type":   "Envelope",
				"sourceNode": envelope.SourceNodeID,
				"domain":     envelope.Domain,
				"remote_ip":  remoteIP,
			})
			err = clusterService.ReceiveMessage(&envelope, token)
		}
		if err != nil {
			logger.ClusterError(logger.ClusterInbound, "events.receive_failed", map[string]interface{}{
				"domain":    extractDomainFromBody(fields),
				"remote_ip": remoteIP,
				"error":     err.Error(),
			})
			c.JSON(http.StatusUnauthorized, Msg{Success: false, Msg: clusterMessage(err)})
			return
		}
		c.JSON(http.StatusOK, Msg{Success: true, Msg: clusterMessage(nil)})
	})
	router.GET(ClusterHeartbeatPath("/"), func(c *gin.Context) {
		remoteIP := c.ClientIP()
		remoteNodeID := c.Query("node_id")
		status, err := clusterService.Heartbeat(remoteNodeID, c.GetHeader("X-Cluster-Token"))
		if err != nil {
			logger.ClusterError(logger.ClusterInbound, "heartbeat", map[string]interface{}{
				"remote_node_id": remoteNodeID,
				"remote_ip":      remoteIP,
				"error":          err.Error(),
			})
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "code": "internal_error", "message": err.Error()})
			return
		}
		logger.ClusterDebug(logger.ClusterInbound, "heartbeat", map[string]interface{}{
			"remote_node_id": remoteNodeID,
			"remote_ip":      remoteIP,
			"status":         status.Status,
		})
		c.JSON(http.StatusOK, status)
	})
	router.GET(ClusterPingPath("/"), func(c *gin.Context) {
		remoteIP := c.ClientIP()
		remoteNodeID := c.Query("node_id")
		status, err := clusterService.Ping(remoteNodeID, c.GetHeader("X-Cluster-Token"))
		if err != nil {
			logger.ClusterError(logger.ClusterInbound, "ping", map[string]interface{}{
				"remote_node_id": remoteNodeID,
				"remote_ip":      remoteIP,
				"error":          err.Error(),
			})
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "code": "internal_error", "message": err.Error()})
			return
		}
		logger.ClusterDebug(logger.ClusterInbound, "ping", map[string]interface{}{
			"remote_node_id": remoteNodeID,
			"remote_ip":      remoteIP,
			"status":         status.Status,
		})
		c.JSON(http.StatusOK, status)
	})
	router.GET(ClusterInfoPath("/"), func(c *gin.Context) {
		clusterService.Info(c)
	})
	router.POST(ClusterActionPath("/"), func(c *gin.Context) {
		logger.ClusterInfo(logger.ClusterInbound, "action", map[string]interface{}{
			"remote_ip": c.ClientIP(),
		})
		clusterService.HandleAction(c)
	})
}

func extractDomainFromBody(fields map[string]json.RawMessage) string {
	if raw, ok := fields["domain"]; ok {
		var domain string
		if err := json.Unmarshal(raw, &domain); err == nil && domain != "" {
			return domain
		}
	}
	if raw, ok := fields["domainId"]; ok {
		var domain string
		if err := json.Unmarshal(raw, &domain); err == nil && domain != "" {
			return domain
		}
	}
	return ""
}

func ClusterMessagePath(basePath string) string {
	return clusterProtocolPath(basePath, "events")
}

func ClusterHeartbeatPath(basePath string) string {
	return clusterProtocolPath(basePath, "heartbeat")
}

func ClusterPingPath(basePath string) string {
	return clusterProtocolPath(basePath, "ping")
}

func ClusterInfoPath(basePath string) string {
	return clusterProtocolPath(basePath, "info")
}

func ClusterActionPath(basePath string) string {
	return clusterProtocolPath(basePath, "action")
}

func clusterProtocolPath(basePath string, action string) string {
	trimmed := strings.TrimSuffix(basePath, "/")
	if trimmed == "" {
		return service.ClusterCommunicationEndpointPath + "/" + service.ClusterCommunicationProtocolVersion + "/" + action
	}
	return trimmed + service.ClusterCommunicationEndpointPath + "/" + service.ClusterCommunicationProtocolVersion + "/" + action
}

func clusterMessage(err error) string {
	if err != nil {
		return "cluster message: " + err.Error()
	}
	return "cluster message received"
}
