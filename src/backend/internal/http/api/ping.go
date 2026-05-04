package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/ping"

	"github.com/gin-gonic/gin"
)

type externalPingService interface {
	Run(context.Context, ping.ExternalRunRequest, []ping.MeshMember) (*ping.ExternalResultData, error)
}

var defaultEnabledInboundExternalSourceIDs = map[string]struct{}{
	"check_host": {},
}

type pingAPIHandler struct {
	clusterService *service.ClusterService
	meshService    *ping.MeshService
	externalSvc    externalPingService
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
	g.POST("/ping/mesh/:domainId/stream", h.streamMeshPing)
	g.POST("/ping/external", h.triggerExternalPing)
	g.GET("/ping/external/results", h.getExternalResults)
	g.GET("/ping/external/config", h.getExternalConfig)
	g.PUT("/ping/external/config", h.putExternalConfig)
	g.GET("/ping/policy/:domainId", h.getPingPolicy)
	g.PUT("/ping/policy/:domainId", h.putPingPolicy)
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

	pingMembers, localID, ok := h.collectMeshPingMembers(c, domainID)
	if !ok {
		return
	}

	policy := h.loadPingPolicyForDomain(domainID)
	result, err := h.meshService.Run(c.Request.Context(), domainID, pingMembers, localID, policy.MaxConcurrent)
	if err != nil {
		c.JSON(http.StatusBadGateway, Msg{Success: false, Msg: "mesh ping failed: " + err.Error()})
		return
	}

	if err := h.store.SaveMeshResult(result); err != nil {
		jsonMsg(c, "save mesh ping", err)
		return
	}
	ping.SetMeshResult(domainID, result)
	jsonObj(c, result, nil)
}

func (h *pingAPIHandler) collectMeshPingMembers(c *gin.Context, domainID string) ([]ping.MeshMember, string, bool) {
	domains, err := h.clusterService.ListDomains()
	if err != nil {
		jsonMsg(c, "trigger mesh ping", err)
		return nil, "", false
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
		return nil, "", false
	}

	members, err := h.clusterService.ListMembers()
	if err != nil {
		jsonMsg(c, "trigger mesh ping", err)
		return nil, "", false
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
	return pingMembers, localID, true
}

type meshPingStreamEvent struct {
	Type   string `json:"type"`
	Result any    `json:"result,omitempty"`
	Msg    string `json:"msg,omitempty"`
}

func (h *pingAPIHandler) streamMeshPing(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	domainID := c.Param("domainId")
	if domainID == "" {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "domainId is required"})
		return
	}

	if !ping.AcquireMeshPingLock() {
		c.JSON(http.StatusConflict, Msg{Success: false, Msg: "mesh ping already in progress"})
		return
	}
	defer ping.ReleaseMeshPingLock()

	pingMembers, localID, ok := h.collectMeshPingMembers(c, domainID)
	if !ok {
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, Msg{Success: false, Msg: "streaming is not supported"})
		return
	}

	c.Writer.Header().Set("Content-Type", "application/x-ndjson")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	encoder := json.NewEncoder(c.Writer)
	writeFailed := false
	writeEvent := func(event meshPingStreamEvent) {
		if writeFailed {
			return
		}
		if err := encoder.Encode(event); err != nil {
			writeFailed = true
			return
		}
		flusher.Flush()
	}

	policy := h.loadPingPolicyForDomain(domainID)
	result, err := h.meshService.RunWithProgress(c.Request.Context(), domainID, pingMembers, localID, func(pairResult ping.MeshPairResult) {
		writeEvent(meshPingStreamEvent{Type: "result", Result: pairResult})
	}, policy.MaxConcurrent)
	if writeFailed {
		return
	}
	if err != nil {
		writeEvent(meshPingStreamEvent{Type: "error", Msg: "mesh ping failed: " + err.Error()})
		return
	}
	if err := h.store.SaveMeshResult(result); err != nil {
		writeEvent(meshPingStreamEvent{Type: "error", Msg: "save mesh ping: " + err.Error()})
		return
	}
	ping.SetMeshResult(domainID, result)
	writeEvent(meshPingStreamEvent{Type: "done", Result: result})
}

func (h *pingAPIHandler) getMeshPing(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	domainID := c.Param("domainId")

	if cached := ping.GetLatestMeshResult(domainID); cached != nil {
		jsonObj(c, cached, nil)
		return
	}

	result, err := h.store.LoadMeshResult(domainID)
	if err != nil {
		if os.IsNotExist(err) {
			jsonObj(c, nil, nil)
			return
		}
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

	if h.shouldDeriveExternalTarget(req) {
		target, err := externalTargetFromRequest(c)
		if err != nil {
			jsonMsg(c, "external ping", err)
			return
		}
		req.Target = target
	}

	data, err := h.externalSvc.Run(c.Request.Context(), req, nil)
	if err != nil {
		jsonMsg(c, "external ping", err)
		return
	}
	jsonObj(c, data, nil)
}

func (h *pingAPIHandler) shouldDeriveExternalTarget(req ping.ExternalRunRequest) bool {
	if req.Target != nil {
		return false
	}
	if req.Direction == ping.DirectionOutbound {
		return false
	}
	return h.requestIncludesInboundExternalSource(req.SourceIDs)
}

func (h *pingAPIHandler) requestIncludesInboundExternalSource(sourceIDs []string) bool {
	if len(sourceIDs) == 0 {
		return false
	}

	requested := make(map[string]struct{}, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		sourceID = strings.TrimSpace(sourceID)
		if sourceID != "" {
			requested[sourceID] = struct{}{}
		}
	}
	if len(requested) == 0 {
		return false
	}

	if h.store == nil {
		for sourceID := range requested {
			if _, ok := defaultEnabledInboundExternalSourceIDs[sourceID]; ok {
				return true
			}
		}
		return false
	}

	config := h.store.LoadExternalConfigOrDefault()
	for _, src := range config.Sources {
		if _, ok := requested[src.ID]; ok && src.Enabled && src.Direction == ping.DirectionInbound {
			return true
		}
	}
	return false
}

func externalTargetFromRequest(c *gin.Context) (*ping.ExternalTargetRequest, error) {
	host := strings.TrimSpace(c.Request.Host)
	if host == "" {
		return nil, fmt.Errorf("inbound target host is required")
	}

	hostname, port, err := net.SplitHostPort(host)
	if err == nil {
		hostname = strings.TrimSpace(hostname)
		if hostname == "" {
			return nil, fmt.Errorf("inbound target host is required")
		}
		if err := validateDerivedExternalTargetHost(hostname); err != nil {
			return nil, err
		}
		parsedPort, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid inbound target port: %w", err)
		}
		return &ping.ExternalTargetRequest{Host: hostname, Port: parsedPort, Label: hostname}, nil
	}

	if strings.HasPrefix(host, "[") || strings.HasSuffix(host, "]") {
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			hostname := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(host, "["), "]"))
			if hostname == "" {
				return nil, fmt.Errorf("inbound target host is required")
			}
			if err := validateDerivedExternalTargetHost(hostname); err != nil {
				return nil, err
			}
			return &ping.ExternalTargetRequest{Host: hostname, Label: hostname}, nil
		}
		return nil, fmt.Errorf("invalid inbound target host: %s", host)
	}

	if strings.Contains(host, ":") {
		return nil, fmt.Errorf("invalid inbound target host: %s", host)
	}

	if err := validateDerivedExternalTargetHost(host); err != nil {
		return nil, err
	}
	return &ping.ExternalTargetRequest{Host: host, Label: host}, nil
}

func validateDerivedExternalTargetHost(hostname string) error {
	if strings.EqualFold(strings.TrimSuffix(hostname, "."), "localhost") {
		return fmt.Errorf("inbound target host must be public or explicitly provided")
	}
	ip := net.ParseIP(hostname)
	if ip == nil {
		return nil
	}
	if ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast() {
		return fmt.Errorf("inbound target host must be public or explicitly provided")
	}
	return nil
}

func (h *pingAPIHandler) getExternalResults(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	data, err := h.store.LoadExternalResults()
	if err != nil {
		if os.IsNotExist(err) {
			jsonObj(c, &ping.ExternalResultData{TestedAt: 0, Results: []ping.ExternalTestResult{}}, nil)
			return
		}
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

func (h *pingAPIHandler) resolveDomainID(domainStr string) (uint, error) {
	domains, err := h.clusterService.ListDomains()
	if err != nil {
		return 0, err
	}
	for _, d := range domains {
		if d.Domain == domainStr {
			return d.ID, nil
		}
	}
	return 0, fmt.Errorf("domain not found: %s", domainStr)
}

func (h *pingAPIHandler) loadPingPolicyForDomain(domainStr string) ping.PingPolicy {
	domainID, err := h.resolveDomainID(domainStr)
	if err != nil {
		return ping.DefaultPingPolicy()
	}
	domain, err := h.clusterService.GetDomain(domainID)
	if err != nil {
		return ping.DefaultPingPolicy()
	}
	return domain.GetPingPolicy()
}

func (h *pingAPIHandler) getPingPolicy(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	domainStr := c.Param("domainId")
	if domainStr == "" {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "domainId is required"})
		return
	}
	domainID, err := h.resolveDomainID(domainStr)
	if err != nil {
		jsonMsg(c, "get ping policy", err)
		return
	}
	domain, err := h.clusterService.GetDomain(domainID)
	if err != nil {
		jsonMsg(c, "get ping policy", err)
		return
	}
	policy := domain.GetPingPolicy()
	jsonObj(c, policy, nil)
}

func (h *pingAPIHandler) putPingPolicy(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	domainStr := c.Param("domainId")
	if domainStr == "" {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "domainId is required"})
		return
	}
	domainID, err := h.resolveDomainID(domainStr)
	if err != nil {
		jsonMsg(c, "update ping policy", err)
		return
	}
	domain, err := h.clusterService.GetDomain(domainID)
	if err != nil {
		jsonMsg(c, "update ping policy", err)
		return
	}
	var policy ping.PingPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "invalid policy: " + err.Error()})
		return
	}
	if policy.Enabled && policy.Interval < 10 {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "interval must be at least 10 seconds"})
		return
	}
	if policy.Enabled && policy.MaxConcurrent < 1 {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "max_concurrent must be at least 1"})
		return
	}
	if err := domain.SetPingPolicy(policy); err != nil {
		jsonMsg(c, "update ping policy", err)
		return
	}
	if err := h.clusterService.SaveDomain(domain); err != nil {
		jsonMsg(c, "update ping policy", err)
		return
	}
	jsonObj(c, policy, nil)
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
