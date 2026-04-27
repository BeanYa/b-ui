package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"

	"github.com/gin-gonic/gin"
)

type clusterAPIService interface {
	Register(service.ClusterRegisterRequest) (*service.ClusterOperationStatus, error)
	GetOperation(string) (*service.ClusterOperationStatus, error)
	ListDomains() ([]service.ClusterDomainResponse, error)
	ListMembers() ([]service.ClusterMemberResponse, error)
	GetMemberConnection(string) (*service.ClusterMemberConnectionResponse, error)
	ManualSync() (*service.ClusterOperationStatus, error)
	DeleteMember(uint) error
	LeaveDomain(uint) error
	ReceivePeerMessage(*service.PeerMessage, string) error
	ReceiveMessage(*service.ClusterEnvelope, string) error
	Heartbeat(string) (*service.ClusterPeerStatus, error)
	Ping(string) (*service.ClusterPeerStatus, error)
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
	jsonObj(c, connection, err)
}

func (a *APIHandler) manualClusterSync(c *gin.Context) {
	if !a.requireClusterAdmin(c) {
		return
	}
	status, err := a.clusterService.ManualSync()
	jsonObj(c, status, err)
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
		if _, ok := fields["protocolVersion"]; ok {
			var message service.PeerMessage
			if err := json.Unmarshal(body, &message); err != nil {
				c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "cluster message: " + err.Error()})
				return
			}
			err = clusterService.ReceivePeerMessage(&message, token)
		} else {
			var envelope service.ClusterEnvelope
			if err := json.Unmarshal(body, &envelope); err != nil {
				c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "cluster message: " + err.Error()})
				return
			}
			err = clusterService.ReceiveMessage(&envelope, token)
		}
		if err != nil {
			c.JSON(http.StatusUnauthorized, Msg{Success: false, Msg: clusterMessage(err)})
			return
		}
		c.JSON(http.StatusOK, Msg{Success: true, Msg: clusterMessage(nil)})
	})
	router.GET(ClusterHeartbeatPath("/"), func(c *gin.Context) {
		status, err := clusterService.Heartbeat(c.GetHeader("X-Cluster-Token"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "code": "internal_error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, status)
	})
	router.GET(ClusterPingPath("/"), func(c *gin.Context) {
		status, err := clusterService.Ping(c.GetHeader("X-Cluster-Token"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "code": "internal_error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, status)
	})
	router.GET(ClusterInfoPath("/"), func(c *gin.Context) {
		clusterService.Info(c)
	})
	router.POST(ClusterActionPath("/"), func(c *gin.Context) {
		clusterService.HandleAction(c)
	})
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
