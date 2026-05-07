package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"

	"github.com/gin-gonic/gin"
)

func serviceErrUnsupported(feature string) error {
	return errors.New(feature + " service is unavailable")
}

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
	ListDomainResources(context.Context, uint) (service.ClusterHubDomainResources, error)
	CreateDomainInboundResource(context.Context, uint, service.ClusterDomainInboundCommandInput) (*service.ClusterDomainOperationView, error)
	UpdateDomainInboundResource(context.Context, uint, string, service.ClusterDomainInboundCommandInput) (*service.ClusterDomainOperationView, error)
	DeleteDomainInboundResource(context.Context, uint, string) (*service.ClusterDomainOperationView, error)
	CreateDomainUserResource(context.Context, uint, service.ClusterDomainUserCommandInput) (*service.ClusterDomainOperationView, error)
	UpdateDomainUserResource(context.Context, uint, string, service.ClusterDomainUserCommandInput) (*service.ClusterDomainOperationView, error)
	DeleteDomainUserResource(context.Context, uint, string) (*service.ClusterDomainOperationView, error)
	DeleteMember(uint, bool) error
	LeaveDomain(uint, bool) error
	ReceivePeerMessage(*service.PeerMessage, string) error
	ReceivePeerMessageWithResult(*service.PeerMessage, string) (*clustertypes.DomainResourceCommandResult, error)
	ReceiveMessage(*service.ClusterEnvelope, string) error
	Heartbeat(remoteNodeID string, token string) (*service.ClusterPeerStatus, error)
	Ping(remoteNodeID string, token string) (*service.ClusterPeerStatus, error)
	HandleAction(c *gin.Context)
	Info(c *gin.Context)
	ListScatterTasksForAdmin(domainID string) ([]service.TaskSummary, error)
	CreateScatterTaskForAdmin(domainID string, taskType string, scope string, params map[string]any) (*service.TaskSummary, error)
	GetScatterTaskResultForAdmin(domainID string, taskID string) (*service.TaskResultDetail, error)
	ListScatterTasks(domainID string, token string) ([]service.TaskSummary, error)
	CreateScatterTask(domainID string, token string, taskType string, scope string, params map[string]any) (*service.TaskSummary, error)
	GetScatterTaskResult(domainID string, token string, taskID string) (*service.TaskResultDetail, error)
}

type clusterDomainOperationRetryService interface {
	RetryDomainOperation(context.Context, string) (*service.ClusterDomainOperationView, error)
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

func (a *APIHandler) listClusterDomainResources(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonMsg(c, "cluster domain resources", err)
		return
	}
	result, err := a.clusterService.ListDomainResources(c.Request.Context(), uint(id))
	jsonObj(c, result, err)
}

func (a *APIHandler) createClusterDomainInboundResource(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonMsg(c, "cluster domain inbound resource create", err)
		return
	}
	var input service.ClusterDomainInboundCommandInput
	if err := c.ShouldBindJSON(&input); err != nil {
		jsonMsg(c, "cluster domain inbound resource create", err)
		return
	}
	result, err := a.clusterService.CreateDomainInboundResource(c.Request.Context(), uint(id), input)
	jsonObj(c, result, err)
}

func (a *APIHandler) updateClusterDomainInboundResource(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonMsg(c, "cluster domain inbound resource update", err)
		return
	}
	var input service.ClusterDomainInboundCommandInput
	if err := c.ShouldBindJSON(&input); err != nil {
		jsonMsg(c, "cluster domain inbound resource update", err)
		return
	}
	result, err := a.clusterService.UpdateDomainInboundResource(c.Request.Context(), uint(id), strings.TrimSpace(c.Param("groupId")), input)
	jsonObj(c, result, err)
}

func (a *APIHandler) deleteClusterDomainInboundResource(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonMsg(c, "cluster domain inbound resource delete", err)
		return
	}
	result, err := a.clusterService.DeleteDomainInboundResource(c.Request.Context(), uint(id), strings.TrimSpace(c.Param("groupId")))
	jsonObj(c, result, err)
}

func (a *APIHandler) createClusterDomainUserResource(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonMsg(c, "cluster domain user resource create", err)
		return
	}
	var input service.ClusterDomainUserCommandInput
	if err := c.ShouldBindJSON(&input); err != nil {
		jsonMsg(c, "cluster domain user resource create", err)
		return
	}
	result, err := a.clusterService.CreateDomainUserResource(c.Request.Context(), uint(id), input)
	jsonObj(c, result, err)
}

func (a *APIHandler) updateClusterDomainUserResource(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonMsg(c, "cluster domain user resource update", err)
		return
	}
	var input service.ClusterDomainUserCommandInput
	if err := c.ShouldBindJSON(&input); err != nil {
		jsonMsg(c, "cluster domain user resource update", err)
		return
	}
	result, err := a.clusterService.UpdateDomainUserResource(c.Request.Context(), uint(id), strings.TrimSpace(c.Param("uuid")), input)
	jsonObj(c, result, err)
}

func (a *APIHandler) deleteClusterDomainUserResource(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonMsg(c, "cluster domain user resource delete", err)
		return
	}
	result, err := a.clusterService.DeleteDomainUserResource(c.Request.Context(), uint(id), strings.TrimSpace(c.Param("uuid")))
	jsonObj(c, result, err)
}

func (a *APIHandler) retryClusterDomainOperation(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	retryService, ok := a.clusterService.(clusterDomainOperationRetryService)
	if !ok {
		jsonMsg(c, "cluster domain operation retry", serviceErrUnsupported("cluster domain operation retry"))
		return
	}
	result, err := retryService.RetryDomainOperation(c.Request.Context(), strings.TrimSpace(c.Param("operationId")))
	jsonObj(c, result, err)
}

func registerClusterDomainResourceRoutes(g gin.IRoutes, handler *APIHandler) {
	g.GET("/cluster/domains/:id/resources", handler.listClusterDomainResources)
	g.POST("/cluster/domains/:id/resources/inbounds", handler.createClusterDomainInboundResource)
	g.PUT("/cluster/domains/:id/resources/inbounds/:groupId", handler.updateClusterDomainInboundResource)
	g.DELETE("/cluster/domains/:id/resources/inbounds/:groupId", handler.deleteClusterDomainInboundResource)
	g.POST("/cluster/domains/:id/resources/users", handler.createClusterDomainUserResource)
	g.PUT("/cluster/domains/:id/resources/users/:uuid", handler.updateClusterDomainUserResource)
	g.DELETE("/cluster/domains/:id/resources/users/:uuid", handler.deleteClusterDomainUserResource)
	g.POST("/cluster/domain-operations/:operationId/retry", handler.retryClusterDomainOperation)
}

func (a *APIHandler) listClusterScatterTasks(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	tasks, err := a.clusterService.ListScatterTasksForAdmin(c.Param("id"))
	jsonObj(c, tasks, err)
}

func (a *APIHandler) createClusterScatterTask(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	var req struct {
		TaskType string         `json:"taskType"`
		Scope    string         `json:"scope"`
		Params   map[string]any `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, "create scatter task", err)
		return
	}
	result, err := a.clusterService.CreateScatterTaskForAdmin(c.Param("id"), req.TaskType, req.Scope, req.Params)
	jsonObj(c, result, err)
}

func (a *APIHandler) getClusterScatterTaskResult(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	result, err := a.clusterService.GetScatterTaskResultForAdmin(c.Param("id"), c.Param("taskId"))
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
	jsonMsg(c, "delete cluster member", a.clusterService.DeleteMember(uint(id), requestForceDelete(c)))
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
	jsonMsg(c, "leave cluster domain", a.clusterService.LeaveDomain(uint(id), requestForceDelete(c)))
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
		var result *clustertypes.DomainResourceCommandResult
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
			result, err = clusterService.ReceivePeerMessageWithResult(&message, token)
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
			c.JSON(http.StatusOK, gin.H{"status": "rejected", "code": "request_rejected", "message": clusterMessage(err)})
			return
		}
		if result != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"msg":     clusterMessage(nil),
				"result":  result,
			})
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
			c.JSON(http.StatusOK, gin.H{"status": "failed", "code": "internal_error", "message": err.Error()})
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
			c.JSON(http.StatusOK, gin.H{"status": "failed", "code": "internal_error", "message": err.Error()})
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
	router.GET("/_cluster/v1/domains/:domainId/tasks", func(c *gin.Context) {
		tasks, err := clusterService.ListScatterTasks(c.Param("domainId"), c.GetHeader("X-Cluster-Token"))
		jsonObj(c, tasks, err)
	})
	router.POST("/_cluster/v1/domains/:domainId/tasks", func(c *gin.Context) {
		var req struct {
			TaskType string         `json:"taskType"`
			Scope    string         `json:"scope"`
			Params   map[string]any `json:"params"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			jsonMsg(c, "create scatter task", err)
			return
		}
		result, err := clusterService.CreateScatterTask(c.Param("domainId"), c.GetHeader("X-Cluster-Token"), req.TaskType, req.Scope, req.Params)
		jsonObj(c, result, err)
	})
	router.GET("/_cluster/v1/domains/:domainId/tasks/:taskId/result", func(c *gin.Context) {
		result, err := clusterService.GetScatterTaskResult(c.Param("domainId"), c.GetHeader("X-Cluster-Token"), c.Param("taskId"))
		jsonObj(c, result, err)
	})
}

func requestForceDelete(c *gin.Context) bool {
	force := c.Query("force")
	if force == "1" || strings.EqualFold(force, "true") {
		return true
	}
	if c.Request != nil {
		force = c.Request.FormValue("force")
	}
	return force == "1" || strings.EqualFold(force, "true")
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
