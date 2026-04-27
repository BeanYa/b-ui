package api

import (
	"net/http"
	"strings"

	service "github.com/alireza0/b-ui/src/backend/internal/domain/services"
	"github.com/alireza0/b-ui/src/backend/internal/domain/services/ping"

	"github.com/gin-gonic/gin"
)

type pingAPIHandler struct {
	clusterService *service.ClusterService
	meshService    *ping.MeshService
	externalSvc    *ping.ExternalService
	store          *ping.Store
}

func RegisterPingRoutes(g *gin.RouterGroup) {
	h := &pingAPIHandler{
		clusterService: &service.ClusterService{},
		meshService:    ping.NewMeshService(),
		store:          ping.NewStore(),
	}
	h.externalSvc = ping.NewExternalService(h.store)

	g.POST("/ping/mesh/:domainId", h.triggerMeshPing)
	g.GET("/ping/mesh/:domainId", h.getMeshPing)
	g.POST("/ping/external", h.triggerExternalPing)
	g.GET("/ping/external/results", h.getExternalResults)
	g.GET("/ping/external/config", h.getExternalConfig)
	g.PUT("/ping/external/config", h.putExternalConfig)
}

func (h *pingAPIHandler) triggerMeshPing(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	domainID := c.Param("domainId")
	if domainID == "" {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "domainId is required"})
		return
	}

	domains, err := h.clusterService.ListDomains()
	if err != nil {
		jsonMsg(c, "trigger mesh ping", err)
		return
	}

	var targetDomain *service.ClusterDomainResponse
	for _, d := range domains {
		if d.Domain == domainID {
			targetDomain = &d
			break
		}
	}
	if targetDomain == nil {
		c.JSON(http.StatusNotFound, Msg{Success: false, Msg: "domain not found"})
		return
	}

	members, err := h.clusterService.ListMembers()
	if err != nil {
		jsonMsg(c, "trigger mesh ping", err)
		return
	}

	pingMembers := make([]ping.MeshMember, 0)
	for _, m := range members {
		if m.DomainID != targetDomain.ID {
			continue
		}
		conn, _ := h.clusterService.GetMemberConnection(m.NodeID)
		pm := ping.MeshMember{
			MemberID: m.NodeID,
			NodeID:   m.NodeID,
			Name:     m.DisplayName,
			BaseURL:  m.BaseURL,
		}
		if m.Name != "" {
			pm.Name = m.Name
		}
		if conn != nil {
			pm.PeerToken = conn.Token
			pm.Address = extractAddrFromBaseURL(conn.BaseURL)
		}
		pingMembers = append(pingMembers, pm)
	}

	localID, _ := getLocalNodeID(h.clusterService)

	result, err := h.meshService.Run(c.Request.Context(), domainID, pingMembers, localID)
	if err != nil {
		c.JSON(http.StatusBadGateway, Msg{Success: false, Msg: "mesh ping failed: " + err.Error()})
		return
	}

	if err := h.store.SaveMeshResult(result); err != nil {
		jsonMsg(c, "save mesh ping", err)
		return
	}
	jsonObj(c, result, nil)
}

func (h *pingAPIHandler) getMeshPing(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	domainID := c.Param("domainId")
	result, err := h.store.LoadMeshResult(domainID)
	if err != nil {
		c.JSON(http.StatusNotFound, Msg{Success: false, Msg: "no mesh ping data for domain: " + domainID})
		return
	}
	jsonObj(c, result, nil)
}

func (h *pingAPIHandler) triggerExternalPing(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	var req ping.ExternalRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "invalid request: " + err.Error()})
		return
	}

	members, err := h.clusterService.ListMembers()
	if err != nil {
		jsonMsg(c, "external ping", err)
		return
	}

	pingMembers := make([]ping.MeshMember, 0, len(members))
	for _, m := range members {
		conn, _ := h.clusterService.GetMemberConnection(m.NodeID)
		pm := ping.MeshMember{
			MemberID: m.NodeID,
			NodeID:   m.NodeID,
			Name:     m.DisplayName,
			BaseURL:  m.BaseURL,
		}
		if m.Name != "" {
			pm.Name = m.Name
		}
		if conn != nil {
			pm.PeerToken = conn.Token
			pm.Address = extractAddrFromBaseURL(conn.BaseURL)
		}
		pingMembers = append(pingMembers, pm)
	}

	data, err := h.externalSvc.Run(c.Request.Context(), req, pingMembers)
	if err != nil {
		jsonMsg(c, "external ping", err)
		return
	}
	jsonObj(c, data, nil)
}

func (h *pingAPIHandler) getExternalResults(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	data, err := h.store.LoadExternalResults()
	if err != nil {
		c.JSON(http.StatusNotFound, Msg{Success: false, Msg: "no external ping results"})
		return
	}
	jsonObj(c, data, nil)
}

func (h *pingAPIHandler) getExternalConfig(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	config := h.store.LoadExternalConfigOrDefault()
	jsonObj(c, config, nil)
}

func (h *pingAPIHandler) putExternalConfig(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	var config ping.ExternalConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "invalid config: " + err.Error()})
		return
	}
	if err := h.store.SaveExternalConfig(&config); err != nil {
		jsonMsg(c, "save external config", err)
		return
	}
	jsonObj(c, Msg{Success: true, Msg: "config saved"}, nil)
}

func requireAdmin(c *gin.Context) bool {
	username := GetLoginUser(c)
	if username == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, Msg{Success: false, Msg: "login required"})
		return false
	}
	return true
}

func getLocalNodeID(cs *service.ClusterService) (string, error) {
	identity := service.ClusterLocalIdentityService{}
	local, err := identity.GetOrCreate()
	if err != nil {
		return "", err
	}
	return local.NodeID, nil
}

func extractAddrFromBaseURL(baseURL string) string {
	s := baseURL
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.Index(s, ":"); idx >= 0 {
		s = s[:idx]
	}
	return s
}
