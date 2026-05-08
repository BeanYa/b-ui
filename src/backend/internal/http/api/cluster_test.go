package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
)

func TestClusterRegisterReturnsOperationStatus(t *testing.T) {
	router, cluster := newTestClusterRouter()
	registerBody := bytes.NewBufferString(`{"domain":"edge.example.com","hubUrl":"https://hub.example.com","token":"cluster-token","baseUrl":"https://panel.example.com/app/","address":"203.0.113.10"}`)
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
	if cluster.registeredRequest.Address != "203.0.113.10" {
		t.Fatalf("expected forwarded node address, got %q", cluster.registeredRequest.Address)
	}
}

func TestNewAPIHandlerUsesProvidedClusterService(t *testing.T) {
	router := gin.New()
	clusterSvc := service.NewClusterService()

	handler := NewAPIHandler(router.Group("/api"), nil, clusterSvc)

	if handler.clusterService != clusterSvc {
		t.Fatal("expected API handler to use the provided cluster service")
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
	registerBody := bytes.NewBufferString("domain=edge.example.com&hubUrl=https%3A%2F%2Fhub.example.com&token=cluster-token&baseUrl=https%3A%2F%2Fpanel.example.com%2Fapp%2F&address=203.0.113.10")
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
	if cluster.registeredRequest.Address != "203.0.113.10" {
		t.Fatalf("expected parsed node address, got %q", cluster.registeredRequest.Address)
	}
}

func TestClusterDomainPanelUpdateCheckRoute(t *testing.T) {
	router, cluster := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/cluster/domains/7/update-check", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.checkUpdateCalls != 1 || cluster.checkedDomainID != 7 {
		t.Fatalf("expected update check for domain 7, got calls=%d id=%d", cluster.checkUpdateCalls, cluster.checkedDomainID)
	}
}

func TestClusterMemberPanelUpdateRoute(t *testing.T) {
	router, cluster := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/cluster/members/8/panel-update", bytes.NewBufferString(`{"targetVersion":"v999.0.0"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.memberPanelUpdateCalls != 1 || cluster.updatedMemberID != 8 || cluster.updatedTargetVersion != "v999.0.0" {
		t.Fatalf("expected panel update for member 8, got calls=%d id=%d target=%q", cluster.memberPanelUpdateCalls, cluster.updatedMemberID, cluster.updatedTargetVersion)
	}
}

func TestClusterDomainInboundCreateRouteUsesDomainResourceCoordinator(t *testing.T) {
	router, cluster := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/cluster/domains/7/resources/inbounds", bytes.NewBufferString(`{"group_id":"group-1","tag_seed":"edge-main","prefix":"edge","suffix":"prod","inbound":{"type":"vless","tag":"main","listen_port":{"LocalProvided":"DomainInboundListenPort"}},"tls_template":"standard","tls":{"name":"edge-main-tls","server":{"enabled":true,"server_name":{"LocalProvided":"DomainName"},"certificate":{"LocalProvided":"GeneratedTLSCertificate"},"key":{"LocalProvided":"GeneratedTLSKey"}},"client":{"insecure":true}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if cluster.createdDomainInboundID != 7 {
		t.Fatalf("expected domain id 7, got %d", cluster.createdDomainInboundID)
	}
	if cluster.createdDomainInboundInput.GroupID != "group-1" {
		t.Fatalf("expected group id to be forwarded, got %#v", cluster.createdDomainInboundInput)
	}
	if cluster.createdDomainInboundInput.TagSeed != "edge-main" || cluster.createdDomainInboundInput.Prefix != "edge" || cluster.createdDomainInboundInput.Suffix != "prod" {
		t.Fatalf("expected tag metadata to be forwarded, got %#v", cluster.createdDomainInboundInput)
	}
	if cluster.createdDomainInboundInput.Inbound["tag"] != "main" || cluster.createdDomainInboundInput.Inbound["type"] != "vless" {
		t.Fatalf("expected inbound payload to be forwarded, got %#v", cluster.createdDomainInboundInput.Inbound)
	}
	if cluster.createdDomainInboundInput.TLSTemplate != "standard" || cluster.createdDomainInboundInput.TLS == nil {
		t.Fatalf("expected tls payload to be forwarded, got %#v", cluster.createdDomainInboundInput)
	}
	if string(cluster.createdDomainInboundInput.TLS.Server) == "" || string(cluster.createdDomainInboundInput.TLS.Client) == "" {
		t.Fatalf("expected tls server/client raw payloads to be forwarded, got %#v", cluster.createdDomainInboundInput.TLS)
	}
}

func TestClusterDomainUserCreateRouteUsesDomainResourceCoordinator(t *testing.T) {
	router, cluster := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/cluster/domains/7/resources/users", bytes.NewBufferString(`{"user":{"uuid":"user-1","name":"Alice","enable":true,"config":{"level":1},"bound_inbound_group_ids":["group-1"]},"bound_inbound_group_ids":["group-1"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if cluster.createdDomainUserID != 7 {
		t.Fatalf("expected domain id 7, got %d", cluster.createdDomainUserID)
	}
	if cluster.createdDomainUserInput.User.UUID != "user-1" || cluster.createdDomainUserInput.User.Name != "Alice" || !cluster.createdDomainUserInput.User.Enable {
		t.Fatalf("expected user payload to be forwarded, got %#v", cluster.createdDomainUserInput.User)
	}
	if string(cluster.createdDomainUserInput.User.Config) != `{"level":1}` {
		t.Fatalf("expected user config to be forwarded, got %s", cluster.createdDomainUserInput.User.Config)
	}
	if len(cluster.createdDomainUserInput.BoundInboundGroupIDs) != 1 || cluster.createdDomainUserInput.BoundInboundGroupIDs[0] != "group-1" {
		t.Fatalf("expected bound inbound group ids to be forwarded, got %#v", cluster.createdDomainUserInput.BoundInboundGroupIDs)
	}
	if len(cluster.createdDomainUserInput.User.BoundInboundGroupIDs) != 1 || cluster.createdDomainUserInput.User.BoundInboundGroupIDs[0] != "group-1" {
		t.Fatalf("expected user bound inbound group ids to be forwarded, got %#v", cluster.createdDomainUserInput.User.BoundInboundGroupIDs)
	}
}

func TestClusterDomainResourcesRouteListsReadModel(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cluster.domainResources = service.ClusterHubDomainResources{
		Inbounds: []service.ClusterHubDomainResourceInbound{{GroupID: "group-1", Type: "vless", Status: "active"}},
		Users: []service.ClusterHubDomainResourceUser{{
			UUID:                 "user-1",
			Name:                 "Alice",
			Enable:               true,
			SubToken:             "stable-token",
			BoundInboundGroupIDs: []string{"group-1"},
		}},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/cluster/domains/7/resources", nil)
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if cluster.listDomainResourcesID != 7 {
		t.Fatalf("expected resources request for domain 7, got %d", cluster.listDomainResourcesID)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	body, err := json.Marshal(response.Obj)
	if err != nil {
		t.Fatalf("marshal resources response: %v", err)
	}
	if !bytes.Contains(body, []byte(`"group_id":"group-1"`)) || !bytes.Contains(body, []byte(`"bound_inbound_group_ids":["group-1"]`)) {
		t.Fatalf("expected resource read model in response, got %s", body)
	}
}

func TestClusterDomainResourceCrudRoutesUseCoordinator(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cookie := loginCookie(t, router, "admin")

	requests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPut, "/api/cluster/domains/7/resources/inbounds/group-1", `{"inbound":{"tag":"updated"}}`},
		{http.MethodDelete, "/api/cluster/domains/7/resources/inbounds/group-1", `{}`},
		{http.MethodPut, "/api/cluster/domains/7/resources/users/user-1", `{"user":{"name":"Alice Updated","enable":true,"config":{"level":2}},"inbounds":["group-1"]}`},
		{http.MethodDelete, "/api/cluster/domains/7/resources/users/user-1", `{}`},
	}
	for _, item := range requests {
		req := httptest.NewRequest(item.method, item.path, bytes.NewBufferString(item.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", cookie)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s %s expected status %d, got %d: %s", item.method, item.path, http.StatusOK, recorder.Code, recorder.Body.String())
		}
	}

	if cluster.updatedDomainInboundID != 7 || cluster.updatedDomainInboundGroupID != "group-1" || cluster.updatedDomainInboundInput.Inbound["tag"] != "updated" {
		t.Fatalf("expected inbound update to be forwarded, got id=%d group=%q input=%#v", cluster.updatedDomainInboundID, cluster.updatedDomainInboundGroupID, cluster.updatedDomainInboundInput)
	}
	if cluster.deletedDomainInboundID != 7 || cluster.deletedDomainInboundGroupID != "group-1" {
		t.Fatalf("expected inbound delete to be forwarded, got id=%d group=%q", cluster.deletedDomainInboundID, cluster.deletedDomainInboundGroupID)
	}
	if cluster.updatedDomainUserID != 7 || cluster.updatedDomainUserUUID != "user-1" || cluster.updatedDomainUserInput.User.Name != "Alice Updated" {
		t.Fatalf("expected user update to be forwarded, got id=%d uuid=%q input=%#v", cluster.updatedDomainUserID, cluster.updatedDomainUserUUID, cluster.updatedDomainUserInput)
	}
	if cluster.deletedDomainUserID != 7 || cluster.deletedDomainUserUUID != "user-1" {
		t.Fatalf("expected user delete to be forwarded, got id=%d uuid=%q", cluster.deletedDomainUserID, cluster.deletedDomainUserUUID)
	}
}

func TestClusterDomainOperationRetryRouteUsesCoordinator(t *testing.T) {
	router, cluster := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/cluster/domain-operations/op-1/retry", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if cluster.retriedDomainOperationID != "op-1" {
		t.Fatalf("expected operation id op-1, got %q", cluster.retriedDomainOperationID)
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
	expectedActionsJSON, err := json.Marshal(service.ClusterCommunicationSupportedActions())
	if err != nil {
		t.Fatalf("marshal supported actions: %v", err)
	}
	expectedSupportedActions := append([]byte(`"supportedActions":`), expectedActionsJSON...)
	if !bytes.Contains(domainsJSON, expectedSupportedActions) {
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
	if bytes.Contains(connectionJSON, []byte(`"token"`)) {
		t.Fatalf("expected peer token to be omitted from browser-facing connection response, got %s", connectionJSON)
	}
	if !bytes.Contains(connectionJSON, []byte(`"baseUrl":"https://node-a.example.com"`)) {
		t.Fatalf("expected baseUrl in connection response, got %s", connectionJSON)
	}
}

func TestClusterMemberInfoUsesServerSideProxy(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cluster.memberInfo = &clustertypes.InfoResponse{Actions: []string{"inbound.list"}}
	req := httptest.NewRequest(http.MethodGet, "/api/cluster/member-info?node_id=node-a", nil)
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.memberInfoNodeID != "node-a" {
		t.Fatalf("expected node_id query to be forwarded, got %q", cluster.memberInfoNodeID)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	infoJSON, err := json.Marshal(response.Obj)
	if err != nil {
		t.Fatalf("marshal info response: %v", err)
	}
	if !bytes.Contains(infoJSON, []byte(`"actions":["inbound.list"]`)) {
		t.Fatalf("expected proxied actions in response, got %s", infoJSON)
	}
}

func TestClusterMemberActionUsesServerSideProxy(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cluster.memberAction = &clustertypes.ActionResponse{Status: "success", Action: "inbound.list"}
	req := httptest.NewRequest(http.MethodPost, "/api/cluster/member-action", bytes.NewBufferString(`{"node_id":"node-a","request":{"schema_version":1,"action":"inbound.list","payload":{"page":1,"page_size":10}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.memberActionNodeID != "node-a" {
		t.Fatalf("expected node_id to be forwarded, got %q", cluster.memberActionNodeID)
	}
	if cluster.memberActionRequest.Action != "inbound.list" {
		t.Fatalf("expected action request to be forwarded, got %#v", cluster.memberActionRequest)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	actionJSON, err := json.Marshal(response.Obj)
	if err != nil {
		t.Fatalf("marshal action response: %v", err)
	}
	if !bytes.Contains(actionJSON, []byte(`"status":"success"`)) || !bytes.Contains(actionJSON, []byte(`"action":"inbound.list"`)) {
		t.Fatalf("expected proxied action response, got %s", actionJSON)
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

func TestClusterDeleteMemberForwardsForceDeleteFlag(t *testing.T) {
	router, cluster := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/cluster/members/7?force=true", nil)
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.deletedMemberID != 7 || !cluster.deletedMemberForce {
		t.Fatalf("expected force delete for member 7, got id=%d force=%v", cluster.deletedMemberID, cluster.deletedMemberForce)
	}
}

func TestClusterLeaveDomainForwardsForceDeleteFlag(t *testing.T) {
	router, cluster := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/cluster/domains/9?force=1", nil)
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.leftDomainID != 9 || !cluster.leftDomainForce {
		t.Fatalf("expected force leave for domain 9, got id=%d force=%v", cluster.leftDomainID, cluster.leftDomainForce)
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

func TestClusterMessageRouteAcceptsDomainInboundDeleteBroadcast(t *testing.T) {
	router, cluster := newTestClusterRouter()
	body := bytes.NewBufferString(`{
			"messageId":"msg-domain-inbound-delete",
			"domainId":"edge.example.com",
			"membershipVersion":3,
			"sourceNodeId":"node-a",
			"sourceSeq":1,
			"category":"command",
			"action":"domain.inbound.delete",
			"protocolVersion":"v1",
			"schemaVersion":1,
			"route":{"mode":"broadcast"},
			"payloadHash":"hash",
			"payload":{"request_id":"req-delete","domain_id":"edge.example.com","group_id":"group-1"},
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
	var response Msg
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || response.Msg != "cluster message received" {
		t.Fatalf("expected cluster success response, got %#v", response)
	}
	if cluster.receivedPeerMessage == nil {
		t.Fatal("expected domain inbound delete broadcast to be passed to service")
	}
	if cluster.receivedPeerMessage.Action != "domain.inbound.delete" || cluster.receivedPeerMessage.Route.Mode != "broadcast" {
		t.Fatalf("expected domain inbound delete broadcast, got %#v", cluster.receivedPeerMessage)
	}
	if cluster.receivedToken != "peer-token" {
		t.Fatalf("expected forwarded token, got %q", cluster.receivedToken)
	}
}

func TestClusterMessageRouteReturnsPeerCommandResultEnvelope(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cluster.receivedPeerResult = &clustertypes.DomainResourceCommandResult{
		Status:          "applied",
		OperationID:     "op-1",
		NodeID:          "node-local",
		MemberID:        "node-a",
		ResourceKind:    "domain_inbound",
		ResourceID:      "group-1",
		LocalResourceID: 99,
		TargetTag:       "main-node-local",
		Revision:        7,
	}
	body := bytes.NewBufferString(`{
			"messageId":"msg-domain-inbound-create",
			"domainId":"edge.example.com",
			"membershipVersion":3,
			"sourceNodeId":"node-a",
			"sourceSeq":1,
			"category":"command",
			"action":"domain.inbound.create",
			"protocolVersion":"v1",
			"schemaVersion":1,
			"route":{"mode":"broadcast"},
			"payloadHash":"hash",
			"payload":{"request_id":"req-create","domain_id":"edge.example.com","group_id":"group-1"},
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
	var response struct {
		Success bool                                      `json:"success"`
		Msg     string                                    `json:"msg"`
		Result  *clustertypes.DomainResourceCommandResult `json:"result"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || response.Msg != "cluster message received" {
		t.Fatalf("expected cluster success response, got %#v", response)
	}
	if response.Result == nil {
		t.Fatal("expected peer result")
	}
	if response.Result.Status != cluster.receivedPeerResult.Status ||
		response.Result.OperationID != cluster.receivedPeerResult.OperationID ||
		response.Result.NodeID != cluster.receivedPeerResult.NodeID ||
		response.Result.ResourceKind != cluster.receivedPeerResult.ResourceKind ||
		response.Result.ResourceID != cluster.receivedPeerResult.ResourceID ||
		response.Result.Revision != cluster.receivedPeerResult.Revision {
		t.Fatalf("expected peer result %#v, got %#v", cluster.receivedPeerResult, response.Result)
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

func TestClusterMessageReceiveReturnsProtocolBodyOnServiceFailure(t *testing.T) {
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

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d for protocol service failure, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response struct {
		Status  string `json:"status"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	decodeResponse(t, recorder, &response)
	if response.Status != "rejected" || response.Code != "request_rejected" || response.Message != "cluster message: verification failed" {
		t.Fatalf("expected protocol rejection body, got %#v", response)
	}
}

func TestClusterHeartbeatReturnsProtocolBodyOnServiceFailure(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cluster.heartbeatErr = errors.New("heartbeat rejected")
	req := httptest.NewRequest(http.MethodGet, "/_cluster/v1/heartbeat?node_id=node-a", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d for heartbeat business failure, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response service.ClusterPeerStatus
	decodeResponse(t, recorder, &response)
	if response.Status != "failed" || response.Code != "internal_error" || response.Message != "heartbeat rejected" {
		t.Fatalf("expected heartbeat protocol failure body, got %#v", response)
	}
}

func TestClusterPingReturnsProtocolBodyOnServiceFailure(t *testing.T) {
	router, cluster := newTestClusterRouter()
	cluster.pingErr = errors.New("ping rejected")
	req := httptest.NewRequest(http.MethodGet, "/_cluster/v1/ping?node_id=node-a", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d for ping business failure, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response service.ClusterPeerStatus
	decodeResponse(t, recorder, &response)
	if response.Status != "failed" || response.Code != "internal_error" || response.Message != "ping rejected" {
		t.Fatalf("expected ping protocol failure body, got %#v", response)
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

	req := httptest.NewRequest(http.MethodGet, "/_cluster/v1/heartbeat?node_id=node-remote", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/_cluster/v1/heartbeat?node_id=node-remote", nil)
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
	registerCalls               int
	listDomainsCalls            int
	listMembersCalls            int
	manualSyncCalls             int
	checkUpdateCalls            int
	memberPanelUpdateCalls      int
	receiveCalls                int
	deletedMemberID             uint
	deletedMemberForce          bool
	leftDomainID                uint
	leftDomainForce             bool
	checkedDomainID             uint
	updatedMemberID             uint
	updatedTargetVersion        string
	receivedToken               string
	receivedEnvelope            *service.ClusterEnvelope
	receivedPeerMessage         *service.PeerMessage
	receivedPeerResult          *clustertypes.DomainResourceCommandResult
	receiveErr                  error
	domains                     []service.ClusterDomainResponse
	members                     []service.ClusterMemberResponse
	registeredRequest           service.ClusterRegisterRequest
	heartbeatResponse           *service.ClusterPeerStatus
	heartbeatErr                error
	pingResponse                *service.ClusterPeerStatus
	pingErr                     error
	memberConnection            *service.ClusterMemberConnectionResponse
	memberConnectionNodeID      string
	memberInfo                  *clustertypes.InfoResponse
	memberInfoNodeID            string
	memberAction                *clustertypes.ActionResponse
	memberActionNodeID          string
	memberActionRequest         clustertypes.ActionRequest
	createdDomainInboundID      uint
	createdDomainInboundInput   service.ClusterDomainInboundCommandInput
	updatedDomainInboundID      uint
	updatedDomainInboundGroupID string
	updatedDomainInboundInput   service.ClusterDomainInboundCommandInput
	deletedDomainInboundID      uint
	deletedDomainInboundGroupID string
	createdDomainUserID         uint
	createdDomainUserInput      service.ClusterDomainUserCommandInput
	updatedDomainUserID         uint
	updatedDomainUserUUID       string
	updatedDomainUserInput      service.ClusterDomainUserCommandInput
	deletedDomainUserID         uint
	deletedDomainUserUUID       string
	retriedDomainOperationID    string
	domainResources             service.ClusterHubDomainResources
	listDomainResourcesID       uint
	scatterTasks                []service.TaskSummary
	scatterTaskResult           *service.TaskResultDetail
	scatterDomainID             string
	scatterTaskID               string
	scatterTaskType             string
	scatterTaskScope            string
	scatterTaskParams           map[string]any
	scatterToken                string
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

func (s *stubClusterAPIService) GetMemberInfo(nodeID string) (*clustertypes.InfoResponse, error) {
	s.memberInfoNodeID = nodeID
	if s.memberInfo != nil {
		return s.memberInfo, nil
	}
	return &clustertypes.InfoResponse{Actions: []string{}}, nil
}

func (s *stubClusterAPIService) SendMemberAction(nodeID string, req clustertypes.ActionRequest) (*clustertypes.ActionResponse, error) {
	s.memberActionNodeID = nodeID
	s.memberActionRequest = req
	if s.memberAction != nil {
		return s.memberAction, nil
	}
	return &clustertypes.ActionResponse{Status: "success", Action: req.Action}, nil
}

func (s *stubClusterAPIService) ManualSync() (*service.ClusterOperationStatus, error) {
	s.manualSyncCalls++
	return &service.ClusterOperationStatus{ID: "op-sync", State: "completed"}, nil
}

func (s *stubClusterAPIService) CheckDomainPanelUpdate(id uint) (*service.ClusterPanelUpdateCheckResult, error) {
	s.checkUpdateCalls++
	s.checkedDomainID = id
	return &service.ClusterPanelUpdateCheckResult{
		CurrentVersion:  "v1.0.0",
		LatestVersion:   "v1.0.1",
		Comparison:      "older",
		UpdateAvailable: true,
		UpdatePolicy:    "manual",
	}, nil
}

func (s *stubClusterAPIService) RequestMemberPanelUpdate(id uint, targetVersion string) (*service.ClusterPanelMemberUpdateResult, error) {
	s.memberPanelUpdateCalls++
	s.updatedMemberID = id
	s.updatedTargetVersion = targetVersion
	return &service.ClusterPanelMemberUpdateResult{
		NodeID:        "node-peer",
		TargetVersion: targetVersion,
		Status:        "updating",
		UpdateStarted: true,
	}, nil
}

func (s *stubClusterAPIService) CreateDomainInboundResource(_ context.Context, domainID uint, input service.ClusterDomainInboundCommandInput) (*service.ClusterDomainOperationView, error) {
	s.createdDomainInboundID = domainID
	s.createdDomainInboundInput = input
	return &service.ClusterDomainOperationView{
		OperationID:  "op-domain-inbound",
		DomainID:     domainID,
		ResourceKind: service.ClusterDomainResourceInbound,
		ResourceID:   input.GroupID,
		Action:       service.ClusterDomainOperationCreate,
		Status:       service.ClusterDomainOperationApplied,
	}, nil
}

func (s *stubClusterAPIService) UpdateDomainInboundResource(_ context.Context, domainID uint, groupID string, input service.ClusterDomainInboundCommandInput) (*service.ClusterDomainOperationView, error) {
	s.updatedDomainInboundID = domainID
	s.updatedDomainInboundGroupID = groupID
	s.updatedDomainInboundInput = input
	return &service.ClusterDomainOperationView{
		OperationID:  "op-domain-inbound-update",
		DomainID:     domainID,
		ResourceKind: service.ClusterDomainResourceInbound,
		ResourceID:   groupID,
		Action:       service.ClusterDomainOperationUpdate,
		Status:       service.ClusterDomainOperationApplied,
	}, nil
}

func (s *stubClusterAPIService) DeleteDomainInboundResource(_ context.Context, domainID uint, groupID string) (*service.ClusterDomainOperationView, error) {
	s.deletedDomainInboundID = domainID
	s.deletedDomainInboundGroupID = groupID
	return &service.ClusterDomainOperationView{
		OperationID:  "op-domain-inbound-delete",
		DomainID:     domainID,
		ResourceKind: service.ClusterDomainResourceInbound,
		ResourceID:   groupID,
		Action:       service.ClusterDomainOperationDelete,
		Status:       service.ClusterDomainOperationApplied,
	}, nil
}

func (s *stubClusterAPIService) CreateDomainUserResource(_ context.Context, domainID uint, input service.ClusterDomainUserCommandInput) (*service.ClusterDomainOperationView, error) {
	s.createdDomainUserID = domainID
	s.createdDomainUserInput = input
	return &service.ClusterDomainOperationView{
		OperationID:  "op-domain-user",
		DomainID:     domainID,
		ResourceKind: service.ClusterDomainResourceUser,
		ResourceID:   input.User.UUID,
		Action:       service.ClusterDomainOperationCreate,
		Status:       service.ClusterDomainOperationApplied,
	}, nil
}

func (s *stubClusterAPIService) UpdateDomainUserResource(_ context.Context, domainID uint, userUUID string, input service.ClusterDomainUserCommandInput) (*service.ClusterDomainOperationView, error) {
	s.updatedDomainUserID = domainID
	s.updatedDomainUserUUID = userUUID
	s.updatedDomainUserInput = input
	return &service.ClusterDomainOperationView{
		OperationID:  "op-domain-user-update",
		DomainID:     domainID,
		ResourceKind: service.ClusterDomainResourceUser,
		ResourceID:   userUUID,
		Action:       service.ClusterDomainOperationUpdate,
		Status:       service.ClusterDomainOperationApplied,
	}, nil
}

func (s *stubClusterAPIService) DeleteDomainUserResource(_ context.Context, domainID uint, userUUID string) (*service.ClusterDomainOperationView, error) {
	s.deletedDomainUserID = domainID
	s.deletedDomainUserUUID = userUUID
	return &service.ClusterDomainOperationView{
		OperationID:  "op-domain-user-delete",
		DomainID:     domainID,
		ResourceKind: service.ClusterDomainResourceUser,
		ResourceID:   userUUID,
		Action:       service.ClusterDomainOperationDelete,
		Status:       service.ClusterDomainOperationApplied,
	}, nil
}

func (s *stubClusterAPIService) RetryDomainOperation(_ context.Context, operationID string) (*service.ClusterDomainOperationView, error) {
	s.retriedDomainOperationID = operationID
	return &service.ClusterDomainOperationView{
		OperationID: operationID,
		Status:      service.ClusterDomainOperationApplied,
	}, nil
}

func (s *stubClusterAPIService) ListDomainResources(_ context.Context, domainID uint) (service.ClusterHubDomainResources, error) {
	s.listDomainResourcesID = domainID
	return s.domainResources, nil
}

func (s *stubClusterAPIService) DeleteMember(id uint, force bool) error {
	s.deletedMemberID = id
	s.deletedMemberForce = force
	return nil
}

func (s *stubClusterAPIService) LeaveDomain(id uint, force bool) error {
	s.leftDomainID = id
	s.leftDomainForce = force
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

func (s *stubClusterAPIService) ReceivePeerMessageWithResult(message *service.PeerMessage, token string) (*clustertypes.DomainResourceCommandResult, error) {
	if err := s.ReceivePeerMessage(message, token); err != nil {
		return nil, err
	}
	return s.receivedPeerResult, nil
}

func (s *stubClusterAPIService) Heartbeat(remoteNodeID string, token string) (*service.ClusterPeerStatus, error) {
	if s.heartbeatErr != nil {
		return nil, s.heartbeatErr
	}
	if s.heartbeatResponse != nil {
		return s.heartbeatResponse, nil
	}
	return &service.ClusterPeerStatus{Status: "processed", Code: "ok", NodeID: "node-local"}, nil
}

func (s *stubClusterAPIService) Ping(remoteNodeID string, token string) (*service.ClusterPeerStatus, error) {
	if s.pingErr != nil {
		return nil, s.pingErr
	}
	if s.pingResponse != nil {
		return s.pingResponse, nil
	}
	return &service.ClusterPeerStatus{Status: "processed", Code: "ok", NodeID: "node-local"}, nil
}

func (s *stubClusterAPIService) ListScatterTasks(domainID string, token string) ([]service.TaskSummary, error) {
	s.scatterDomainID = domainID
	s.scatterToken = token
	return s.scatterTasks, nil
}

func (s *stubClusterAPIService) ListScatterTasksForAdmin(domainID string) ([]service.TaskSummary, error) {
	s.scatterDomainID = domainID
	s.scatterToken = ""
	return s.scatterTasks, nil
}

func (s *stubClusterAPIService) CreateScatterTask(domainID string, token string, taskType string, scope string, params map[string]any) (*service.TaskSummary, error) {
	s.scatterDomainID = domainID
	s.scatterToken = token
	s.scatterTaskType = taskType
	s.scatterTaskScope = scope
	s.scatterTaskParams = params
	return &service.TaskSummary{
		TaskID:   "task-stub",
		TaskType: taskType,
		Status:   "queued",
		Scope:    scope,
	}, nil
}

func (s *stubClusterAPIService) CreateScatterTaskForAdmin(domainID string, taskType string, scope string, params map[string]any) (*service.TaskSummary, error) {
	return s.CreateScatterTask(domainID, "", taskType, scope, params)
}

func (s *stubClusterAPIService) GetScatterTaskResult(domainID string, token string, taskID string) (*service.TaskResultDetail, error) {
	s.scatterDomainID = domainID
	s.scatterToken = token
	s.scatterTaskID = taskID
	if s.scatterTaskResult != nil {
		return s.scatterTaskResult, nil
	}
	return &service.TaskResultDetail{
		TaskID: taskID,
		Status: "completed",
	}, nil
}

func (s *stubClusterAPIService) GetScatterTaskResultForAdmin(domainID string, taskID string) (*service.TaskResultDetail, error) {
	return s.GetScatterTaskResult(domainID, "", taskID)
}

func (s *stubClusterAPIService) HandleAction(c *gin.Context) {
	if c.GetHeader("X-Cluster-Token") == "" {
		c.JSON(http.StatusOK, gin.H{"status": "error", "error_message": "cluster token is required"})
		return
	}
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
	if c.GetHeader("X-Cluster-Token") == "" {
		c.JSON(http.StatusOK, gin.H{"status": "rejected", "code": "invalid_token", "message": "cluster token is required"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"actions": []string{}})
}

func TestClusterInfoEndpoint(t *testing.T) {
	router, _ := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodGet, "/_cluster/v1/info", nil)
	req.Header.Set("X-Cluster-Token", "peer-token")
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

func TestClusterInfoEndpointRejectsMissingToken(t *testing.T) {
	router, _ := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodGet, "/_cluster/v1/info", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response map[string]any
	decodeResponse(t, recorder, &response)
	if response["status"] != "rejected" || response["code"] != "invalid_token" {
		t.Fatalf("expected invalid token rejection, got %#v", response)
	}
}

func TestClusterActionEndpoint_UnsupportedAction(t *testing.T) {
	router, _ := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/_cluster/v1/action", bytes.NewBufferString(`{"domain":"edge.example.com","action":"unknown.action"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cluster-Token", "peer-token")
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

func TestClusterActionEndpointRejectsMissingToken(t *testing.T) {
	router, _ := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/_cluster/v1/action", bytes.NewBufferString(`{"domain":"edge.example.com","action":"domain.cleanup"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response map[string]any
	decodeResponse(t, recorder, &response)
	if response["status"] != "error" {
		t.Fatalf("expected action token rejection, got %#v", response)
	}
	if !strings.Contains(response["error_message"].(string), "token") {
		t.Fatalf("expected token error message, got %#v", response)
	}
}

func TestClusterActionEndpoint_InvalidJSON(t *testing.T) {
	router, _ := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/_cluster/v1/action", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cluster-Token", "peer-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestClusterScatterTaskRoutesForwardClusterToken(t *testing.T) {
	router, cluster := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodGet, "/_cluster/v1/domains/edge.example.com/tasks", nil)
	req.Header.Set("X-Cluster-Token", "peer-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.scatterDomainID != "edge.example.com" || cluster.scatterToken != "peer-token" {
		t.Fatalf("expected task list token/domain forwarded, got domain=%q token=%q", cluster.scatterDomainID, cluster.scatterToken)
	}
}

func TestClusterScatterTaskAdminRoutesUseSessionAuth(t *testing.T) {
	router, cluster := newTestClusterRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/cluster/domains/edge.example.com/tasks", bytes.NewBufferString(`{"taskType":"mesh.latency","scope":"domain","params":{"sample":true}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if cluster.scatterDomainID != "edge.example.com" || cluster.scatterToken != "" {
		t.Fatalf("expected admin task route to avoid peer token, got domain=%q token=%q", cluster.scatterDomainID, cluster.scatterToken)
	}
	if cluster.scatterTaskType != "mesh.latency" || cluster.scatterTaskScope != "domain" {
		t.Fatalf("expected task request forwarded, got type=%q scope=%q", cluster.scatterTaskType, cluster.scatterTaskScope)
	}
}

var _ = model.ClusterMember{}
