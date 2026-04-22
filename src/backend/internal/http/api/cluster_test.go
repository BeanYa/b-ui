package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	service "github.com/alireza0/s-ui/src/backend/internal/domain/services"
	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
	logger "github.com/alireza0/s-ui/src/backend/internal/infra/logging"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
)

func TestClusterRegisterReturnsOperationStatus(t *testing.T) {
	router, cluster := newTestClusterRouter()
	registerBody := bytes.NewBufferString(`{"domain":"edge.example.com","token":"cluster-token"}`)
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
	cluster.domains = []service.ClusterDomainResponse{{ID: 1, Domain: "edge.example.com", LastVersion: 4}}
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
	if cluster.listMembersCalls != 1 {
		t.Fatalf("expected one members call, got %d", cluster.listMembersCalls)
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

func TestClusterMessageReceiveBypassesSessionAndForwardsToken(t *testing.T) {
	router, cluster := newTestClusterRouter()
	body, err := json.Marshal(service.ClusterEnvelope{SchemaVersion: 1, MessageType: "sync.notify_version", SourceNodeID: "node-a", Domain: "edge.example.com", Version: 9, SentAt: 1700000000, Signature: "sig"})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/cluster/message", bytes.NewReader(body))
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

func TestClusterMessageReceiveReturnsNon200OnBindFailure(t *testing.T) {
	router, _ := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/cluster/message", bytes.NewBufferString(`{"schemaVersion":`))
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
	req := httptest.NewRequest(http.MethodPost, "/cluster/message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cluster-Token", "cluster-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected non-200 status for service failure, got %d", recorder.Code)
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
	router.Use(sessions.Sessions("s-ui", cookie.NewStore([]byte("test-secret"))))
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
	registerCalls    int
	listDomainsCalls int
	listMembersCalls int
	manualSyncCalls  int
	receiveCalls     int
	deletedMemberID  uint
	receivedToken    string
	receivedEnvelope *service.ClusterEnvelope
	receiveErr       error
	domains          []service.ClusterDomainResponse
	members          []service.ClusterMemberResponse
}

func (s *stubClusterAPIService) Register(request service.ClusterRegisterRequest) (*service.ClusterOperationStatus, error) {
	s.registerCalls++
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

func (s *stubClusterAPIService) ManualSync() (*service.ClusterOperationStatus, error) {
	s.manualSyncCalls++
	return &service.ClusterOperationStatus{ID: "op-sync", State: "completed"}, nil
}

func (s *stubClusterAPIService) DeleteMember(id uint) error {
	s.deletedMemberID = id
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

var _ = model.ClusterMember{}
