package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
)

func TestClusterRegisterReturnsOperationStatus(t *testing.T) {
	router, cluster := newTestClusterRouter()
	registerBody := bytes.NewBufferString(`{"domain":"edge.example.com","hubUrl":"https://hub.example.com","token":"cluster-token","baseUrl":"https://panel.example.com/app/"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/cluster/register", registerBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	if !response.Success {
		t.Fatalf("expected success response, got %#v", response)
	}
	if cluster.registerCalls != 1 {
		t.Fatalf("expected one register call, got %d", cluster.registerCalls)
	}
	if cluster.registeredRequest.HubURL != "https://hub.example.com" {
		t.Fatalf("expected forwarded hub URL, got %q", cluster.registeredRequest.HubURL)
	}
	if cluster.registeredRequest.BaseURL != "https://panel.example.com/app/" {
		t.Fatalf("expected forwarded base URL, got %q", cluster.registeredRequest.BaseURL)
	}
}

func TestClusterRegisterAcceptsJoinURI(t *testing.T) {
	router, cluster := newTestClusterRouter()
	registerBody := bytes.NewBufferString(`{"joinUri":"buihub://hub.example.com/domain?id=edge.example.com&domain_token=cluster-token&hub_protocol=https","baseUrl":"https://panel.example.com/app/"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/cluster/register", registerBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.registerCalls != 1 {
		t.Fatalf("expected one register call, got %d", cluster.registerCalls)
	}
	if cluster.registeredRequest.HubURL != "https://hub.example.com" {
		t.Fatalf("expected parsed hub URL, got %q", cluster.registeredRequest.HubURL)
	}
	if cluster.registeredRequest.Domain != "edge.example.com" {
		t.Fatalf("expected parsed domain, got %q", cluster.registeredRequest.Domain)
	}
	if cluster.registeredRequest.Token != "cluster-token" {
		t.Fatalf("expected parsed token, got %q", cluster.registeredRequest.Token)
	}
	if cluster.registeredRequest.BaseURL != "https://panel.example.com/app/" {
		t.Fatalf("expected forwarded base URL, got %q", cluster.registeredRequest.BaseURL)
	}
}

func TestClusterRegisterAcceptsFormEncodedRequest(t *testing.T) {
	router, cluster := newTestClusterRouter()
	registerBody := bytes.NewBufferString("domain=edge.example.com&hubUrl=https%3A%2F%2Fhub.example.com&token=cluster-token&baseUrl=https%3A%2F%2Fpanel.example.com%2Fapp%2F")
	req := httptest.NewRequest(http.MethodPost, "/api/cluster/register", registerBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.registerCalls != 1 {
		t.Fatalf("expected one register call, got %d", cluster.registerCalls)
	}
	if cluster.registeredRequest.HubURL != "https://hub.example.com" {
		t.Fatalf("expected parsed hub URL, got %q", cluster.registeredRequest.HubURL)
	}
	if cluster.registeredRequest.Domain != "edge.example.com" {
		t.Fatalf("expected parsed domain, got %q", cluster.registeredRequest.Domain)
	}
	if cluster.registeredRequest.Token != "cluster-token" {
		t.Fatalf("expected parsed token, got %q", cluster.registeredRequest.Token)
	}
	if cluster.registeredRequest.BaseURL != "https://panel.example.com/app/" {
		t.Fatalf("expected parsed base URL, got %q", cluster.registeredRequest.BaseURL)
	}
}

func TestClusterAdminRoutesRequireFirstUserAdmin(t *testing.T) {
	router, cluster := newTestClusterRouterWithUserService(stubUserService{
		isFirstUser: func(username string) (bool, error) {
			return username == "admin", nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/cluster/domains", nil)
	req.Header.Set("Cookie", loginCookie(t, router, "alice"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
	if cluster.listDomainsCalls != 0 {
		t.Fatalf("expected no cluster service calls, got %d", cluster.listDomainsCalls)
	}
}

func TestClusterListsDomainsAndMembers(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cluster.domains = []service.ClusterDomainResponse{{ID: 1, Domain: "edge.example.com", LastVersion: 4, SupportedActions: service.ClusterCommunicationSupportedActions()}}
	cluster.domains[0].HubURL = "https://hub.example.com"
	cluster.members = []service.ClusterMemberResponse{{ID: 2, NodeID: "node-a", Name: "alpha", BaseURL: "https://node-a.example.com", LastVersion: 4}}
	cookie := loginCookie(t, router, "admin")

	domainsReq := httptest.NewRequest(http.MethodGet, "/api/cluster/domains", nil)
	domainsReq.Header.Set("Cookie", cookie)
	domainsRecorder := httptest.NewRecorder()
	router.ServeHTTP(domainsRecorder, domainsReq)

	membersReq := httptest.NewRequest(http.MethodGet, "/api/cluster/members", nil)
	membersReq.Header.Set("Cookie", cookie)
	membersRecorder := httptest.NewRecorder()
	router.ServeHTTP(membersRecorder, membersReq)

	if cluster.listDomainsCalls != 1 {
		t.Fatalf("expected one domains call, got %d", cluster.listDomainsCalls)
	}
	var domainsResponse Msg
	decodeResponse(t, domainsRecorder, &domainsResponse)
	domainsJSON, err := json.Marshal(domainsResponse.Obj)
	if err != nil {
		t.Fatalf("marshal domains response: %v", err)
	}
	if !bytes.Contains(domainsJSON, []byte(`"hubUrl":"https://hub.example.com"`)) {
		t.Fatalf("expected hub URL in domains response, got %s", domainsJSON)
	}
	if !bytes.Contains(domainsJSON, []byte(`"supportedActions":["domain.cluster.changed","events","heartbeat","ping","info","action","domain.panel.update.available"]`)) {
		t.Fatalf("expected supported actions in domains response, got %s", domainsJSON)
	}
	if cluster.listMembersCalls != 1 {
		t.Fatalf("expected one members call, got %d", cluster.listMembersCalls)
	}
}

func TestClusterMemberConnectionUsesNodeIDQuery(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cluster.memberConnection = &service.ClusterMemberConnectionResponse{
		NodeID:      "node-a",
		Name:        "alpha",
		DisplayName: "Alpha",
		BaseURL:     "https://node-a.example.com",
		Token:       "peer-token-a",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/cluster/member-connection?node_id=node-a", nil)
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.memberConnectionNodeID != "node-a" {
		t.Fatalf("expected node_id query to be forwarded, got %q", cluster.memberConnectionNodeID)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	connectionJSON, err := json.Marshal(response.Obj)
	if err != nil {
		t.Fatalf("marshal connection response: %v", err)
	}
	if !bytes.Contains(connectionJSON, []byte(`"token":"peer-token-a"`)) || !bytes.Contains(connectionJSON, []byte(`"baseUrl":"https://node-a.example.com"`)) {
		t.Fatalf("expected token and baseUrl in connection response, got %s", connectionJSON)
	}
}

func TestClusterManualSyncAndDeleteMemberUseService(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cookie := loginCookie(t, router, "admin")

	syncReq := httptest.NewRequest(http.MethodPost, "/api/cluster/sync", bytes.NewBufferString(`{}`))
	syncReq.Header.Set("Content-Type", "application/json")
	syncReq.Header.Set("Cookie", cookie)
	syncRecorder := httptest.NewRecorder()
	router.ServeHTTP(syncRecorder, syncReq)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/cluster/members/7", nil)
	deleteReq.Header.Set("Cookie", cookie)
	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, deleteReq)

	if cluster.manualSyncCalls != 1 {
		t.Fatalf("expected one manual sync call, got %d", cluster.manualSyncCalls)
	}
	if cluster.deletedMemberID != 7 {
		t.Fatalf("expected deleted member id 7, got %d", cluster.deletedMemberID)
	}
}

func TestClusterLeaveDomainUsesService(t *testing.T) {
	router, cluster := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/cluster/domains/9", nil)
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.leftDomainID != 9 {
		t.Fatalf("expected left domain id 9, got %d", cluster.leftDomainID)
	}
}

func TestClusterMessageRouteAcceptsLegacyEnvelope(t *testing.T) {
	router, cluster := newTestClusterRouter()
	body, err := json.Marshal(service.ClusterEnvelope{SchemaVersion: 1, MessageType: "sync.notify_version", SourceNodeID: "node-a", Domain: "edge.example.com", Version: 9, SentAt: 1700000000, Signature: "sig"})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/_cluster/v1/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cluster-Token", "cluster-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.receivedToken != "cluster-token" {
		t.Fatalf("expected forwarded token, got %q", cluster.receivedToken)
	}
	if cluster.receiveCalls != 1 {
		t.Fatalf("expected one receive call, got %d", cluster.receiveCalls)
	}
	if cluster.receivedEnvelope == nil || cluster.receivedEnvelope.SourceNodeID != "node-a" {
		t.Fatalf("expected forwarded envelope, got %#v", cluster.receivedEnvelope)
	}
}

func TestClusterMessageRouteAcceptsPeerMessage(t *testing.T) {
	router, cluster := newTestClusterRouter()
	body := bytes.NewBufferString(`{
			"messageId":"msg-1",
			"domainId":"edge.example.com",
			"membershipVersion":3,
			"sourceNodeId":"node-a",
			"sourceSeq":1,
			"category":"event",
			"action":"domain.cluster.changed",
			"protocolVersion":"v1",
			"schemaVersion":1,
			"route":{"mode":"broadcast"},
			"payloadHash":"hash",
			"payload":{"version":3},
			"signature":"sig"
		}`)
	req := httptest.NewRequest(http.MethodPost, "/_cluster/v1/events", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cluster-Token", "peer-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if cluster.receivedPeerMessage == nil || cluster.receivedPeerMessage.Action != "domain.cluster.changed" {
		t.Fatalf("expected peer message to be passed to service")
	}
	if cluster.receivedToken != "peer-token" {
		t.Fatalf("expected forwarded token, got %q", cluster.receivedToken)
	}
	if cluster.receiveCalls != 0 {
		t.Fatalf("expected legacy receive not to be called, got %d calls", cluster.receiveCalls)
	}
}

func TestClusterMessageRouteRejectsOversizedBody(t *testing.T) {
	router, cluster := newTestClusterRouter()
	body := bytes.NewBufferString(`{
			"messageId":"msg-oversized",
			"domainId":"edge.example.com",
			"membershipVersion":3,
			"sourceNodeId":"node-a",
			"sourceSeq":1,
			"category":"event",
			"action":"domain.cluster.changed",
			"protocolVersion":"v1",
			"schemaVersion":1,
			"route":{"mode":"broadcast"},
			"payloadHash":"hash",
			"payload":{"data":"` + strings.Repeat("x", maxClusterMessageBytes+1) + `"},
			"signature":"sig"
		}`)
	req := httptest.NewRequest(http.MethodPost, "/_cluster/v1/events", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	if cluster.receiveCalls != 0 {
		t.Fatalf("expected legacy receive not to be called, got %d calls", cluster.receiveCalls)
	}
	if cluster.receivedPeerMessage != nil {
		t.Fatalf("expected peer receive not to be called, got %#v", cluster.receivedPeerMessage)
	}
}

func TestClusterMessageReceiveReturnsNon200OnBindFailure(t *testing.T) {
	router, _ := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/_cluster/v1/events", bytes.NewBufferString(`{"schemaVersion":`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected non-200 status for bind failure, got %d", recorder.Code)
	}
}

func TestClusterMessageReceiveReturnsNon200OnServiceFailure(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cluster.receiveErr = errors.New("verification failed")
	body, err := json.Marshal(service.ClusterEnvelope{SchemaVersion: 1, MessageType: "sync.notify_version", SourceNodeID: "node-a", Domain: "edge.example.com", Version: 9, SentAt: 1700000000, Signature: "sig"})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/_cluster/v1/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cluster-Token", "cluster-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected non-200 status for service failure, got %d", recorder.Code)
	}
}

func TestClusterHeartbeatReturnsProtocolPayloadWithDomainContext(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cluster.heartbeatResponse = &service.ClusterPeerStatus{
		Status: "processed",
		Code:   "ok",
		NodeID: "node-local",
		Details: map[string]any{
			"domainId":          "edge.example.com",
			"membershipVersion": float64(9),
			"observedAt":        float64(1700000000),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/_cluster/v1/heartbeat", nil)
	req.Header.Set("X-Cluster-Token", "peer-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected heartbeat status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response map[string]any
	decodeResponse(t, recorder, &response)
	if response["status"] != "processed" || response["code"] != "ok" {
		t.Fatalf("expected processed/ok heartbeat, got %#v", response)
	}
}

func TestClusterHeartbeatReturnsRejectedCodeForUnknownToken(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cluster.heartbeatResponse = &service.ClusterPeerStatus{
		Status: "rejected",
		Code:   "invalid_token",
	}

	req := httptest.NewRequest(http.MethodGet, "/_cluster/v1/heartbeat", nil)
	req.Header.Set("X-Cluster-Token", "wrong-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected heartbeat status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response map[string]any
	decodeResponse(t, recorder, &response)
	if response["code"] != "invalid_token" {
		t.Fatalf("expected invalid_token code, got %#v", response)
	}
}

func newTestClusterRouter() (*gin.Engine, *stubClusterAPIService) {
	return newTestClusterRouterWithUserService(stubUserService{
		isFirstUser: func(username string) (bool, error) {
			return username == "admin", nil
		},
	})
}

func newTestClusterRouterWithUserService(userService apiUserService) (*gin.Engine, *stubClusterAPIService) {
	gin.SetMode(gin.TestMode)
	logger.InitLogger(logging.ERROR)
	router := gin.New()
	router.Use(sessions.Sessions("b-ui", cookie.NewStore([]byte("test-secret"))))
	cluster := &stubClusterAPIService{}
	handler := &APIHandler{clusterService: cluster}
	handler.ApiService.userService = userService
	handler.initRouter(router.Group("/api"))
	RegisterClusterMessageRoute(router, cluster)
	router.GET("/__test/login/:username", func(c *gin.Context) {
		if err := SetLoginUser(c, c.Param("username"), 0); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})
	return router, cluster
}

type stubClusterAPIService struct {
	registerCalls          int
	listDomainsCalls       int
	listMembersCalls       int
	manualSyncCalls        int
	receiveCalls           int
	deletedMemberID        uint
	leftDomainID           uint
	receivedToken          string
	receivedEnvelope       *service.ClusterEnvelope
	receivedPeerMessage    *service.PeerMessage
	receiveErr             error
	domains                []service.ClusterDomainResponse
	members                []service.ClusterMemberResponse
	registeredRequest      service.ClusterRegisterRequest
	heartbeatResponse      *service.ClusterPeerStatus
	pingResponse           *service.ClusterPeerStatus
	memberConnection       *service.ClusterMemberConnectionResponse
	memberConnectionNodeID string
}

func (s *stubClusterAPIService) Register(request service.ClusterRegisterRequest) (*service.ClusterOperationStatus, error) {
	s.registerCalls++
	s.registeredRequest = request
	return &service.ClusterOperationStatus{ID: "op-register", State: "completed"}, nil
}

func (s *stubClusterAPIService) GetOperation(string) (*service.ClusterOperationStatus, error) {
	return &service.ClusterOperationStatus{ID: "op-register", State: "completed"}, nil
}

func (s *stubClusterAPIService) ListDomains() ([]service.ClusterDomainResponse, error) {
	s.listDomainsCalls++
	return s.domains, nil
}

func (s *stubClusterAPIService) ListMembers() ([]service.ClusterMemberResponse, error) {
	s.listMembersCalls++
	return s.members, nil
}

func (s *stubClusterAPIService) GetMemberConnection(nodeID string) (*service.ClusterMemberConnectionResponse, error) {
	s.memberConnectionNodeID = nodeID
	if s.memberConnection != nil {
		return s.memberConnection, nil
	}
	return &service.ClusterMemberConnectionResponse{NodeID: nodeID, BaseURL: "https://node.example.com", Token: "peer-token"}, nil
}

func (s *stubClusterAPIService) ManualSync() (*service.ClusterOperationStatus, error) {
	s.manualSyncCalls++
	return &service.ClusterOperationStatus{ID: "op-sync", State: "completed"}, nil
}

func (s *stubClusterAPIService) DeleteMember(id uint) error {
	s.deletedMemberID = id
	return nil
}

func (s *stubClusterAPIService) LeaveDomain(id uint) error {
	s.leftDomainID = id
	return nil
}

func (s *stubClusterAPIService) ReceiveMessage(envelope *service.ClusterEnvelope, token string) error {
	s.receiveCalls++
	if s.receiveErr != nil {
		return s.receiveErr
	}
	s.receivedToken = token
	copy := *envelope
	s.receivedEnvelope = &copy
	return nil
}

func (s *stubClusterAPIService) ReceivePeerMessage(message *service.PeerMessage, token string) error {
	if s.receiveErr != nil {
		return s.receiveErr
	}
	s.receivedToken = token
	copy := *message
	s.receivedPeerMessage = &copy
	return nil
}

func (s *stubClusterAPIService) Heartbeat(string) (*service.ClusterPeerStatus, error) {
	if s.heartbeatResponse != nil {
		return s.heartbeatResponse, nil
	}
	return &service.ClusterPeerStatus{Status: "processed", Code: "ok", NodeID: "node-local"}, nil
}

func (s *stubClusterAPIService) Ping(string) (*service.ClusterPeerStatus, error) {
	if s.pingResponse != nil {
		return s.pingResponse, nil
	}
	return &service.ClusterPeerStatus{Status: "processed", Code: "ok", NodeID: "node-local"}, nil
}

func (s *stubClusterAPIService) HandleAction(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "invalid json"})
		return
	}
	action, _ := body["action"].(string)
	if action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "missing action"})
		return
	}
	if action != "known.action" {
		c.JSON(http.StatusOK, gin.H{"status": "unsupported", "action": action})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stub", "action": action})
}

func (s *stubClusterAPIService) Info(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"actions": []string{}})
}

func TestClusterInfoEndpoint(t *testing.T) {
	router, _ := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodGet, "/_cluster/v1/info", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response map[string]any
	decodeResponse(t, recorder, &response)
	actions, ok := response["actions"]
	if !ok {
		t.Fatalf("expected 'actions' key in response, got %#v", response)
	}
	_, ok = actions.([]any)
	if !ok {
		t.Fatalf("expected 'actions' to be an array, got %T", actions)
	}
}

func TestClusterActionEndpoint_UnsupportedAction(t *testing.T) {
	router, _ := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/_cluster/v1/action", bytes.NewBufferString(`{"action":"unknown.action"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response map[string]any
	decodeResponse(t, recorder, &response)
	if response["status"] != "unsupported" {
		t.Fatalf("expected status 'unsupported', got %v", response["status"])
	}
	if response["action"] != "unknown.action" {
		t.Fatalf("expected action 'unknown.action', got %v", response["action"])
	}
}

func TestClusterActionEndpoint_InvalidJSON(t *testing.T) {
	router, _ := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/_cluster/v1/action", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

var _ = model.ClusterMember{}
