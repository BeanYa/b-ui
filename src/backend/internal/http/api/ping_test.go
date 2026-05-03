package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/ping"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

type externalPingServiceStub struct {
	called  bool
	req     ping.ExternalRunRequest
	members []ping.MeshMember
	result  *ping.ExternalResultData
	err     error
}

func (s *externalPingServiceStub) Run(ctx context.Context, req ping.ExternalRunRequest, members []ping.MeshMember) (*ping.ExternalResultData, error) {
	s.called = true
	s.req = req
	s.members = members
	if s.result != nil || s.err != nil {
		return s.result, s.err
	}
	return &ping.ExternalResultData{TestedAt: 123, Results: []ping.ExternalTestResult{}}, nil
}

func newExternalPingTestRouter(externalSvc externalPingService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(sessions.Sessions("b-ui", cookie.NewStore([]byte("test-secret"))))
	handler := &pingAPIHandler{externalSvc: externalSvc}
	router.POST("/api/ping/external", handler.triggerExternalPing)
	router.GET("/__test/login/:username", func(c *gin.Context) {
		if err := SetLoginUser(c, c.Param("username"), 0); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})
	return router
}

func externalTargetFromHostForTest(t *testing.T, host string) (*ping.ExternalTargetRequest, error) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ping/external", nil)
	c.Request.Host = host
	return externalTargetFromRequest(c)
}

func TestTriggerExternalPingOutboundDoesNotRequireClusterMembers(t *testing.T) {
	stub := &externalPingServiceStub{}
	router := newExternalPingTestRouter(stub)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/ping/external",
		bytes.NewBufferString(`{"direction":"outbound","source_ids":["public_dns"],"methods":["tcp"]}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected outbound external ping status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	if !response.Success {
		t.Fatalf("expected outbound external ping success, got %#v", response)
	}
	if !stub.called {
		t.Fatal("expected external ping service to be called")
	}
	if stub.req.Direction != ping.DirectionOutbound {
		t.Fatalf("expected outbound direction, got %q", stub.req.Direction)
	}
	if len(stub.req.TargetNodeIDs) != 0 {
		t.Fatalf("expected no target_node_ids, got %#v", stub.req.TargetNodeIDs)
	}
	if stub.req.Target != nil {
		t.Fatalf("expected no outbound target, got %#v", stub.req.Target)
	}
	if len(stub.members) != 0 {
		t.Fatalf("expected no cluster members, got %#v", stub.members)
	}
}

func TestTriggerExternalPingInboundUsesExplicitTarget(t *testing.T) {
	stub := &externalPingServiceStub{}
	router := newExternalPingTestRouter(stub)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/ping/external",
		bytes.NewBufferString(`{"direction":"inbound","source_ids":["check_host"],"target":{"host":"panel.example.com","port":8443,"label":"Panel edge"},"methods":["tcp"]}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected inbound external ping status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	if !response.Success {
		t.Fatalf("expected inbound external ping success, got %#v", response)
	}
	if !stub.called {
		t.Fatal("expected external ping service to be called")
	}
	if stub.req.Direction != ping.DirectionInbound {
		t.Fatalf("expected inbound direction, got %q", stub.req.Direction)
	}
	if stub.req.Target == nil {
		t.Fatal("expected inbound target to be passed to external service")
	}
	if stub.req.Target.Host != "panel.example.com" {
		t.Fatalf("expected target host panel.example.com, got %q", stub.req.Target.Host)
	}
	if stub.req.Target.Port != 8443 {
		t.Fatalf("expected target port 8443, got %d", stub.req.Target.Port)
	}
	if stub.req.Target.Label != "Panel edge" {
		t.Fatalf("expected target label Panel edge, got %q", stub.req.Target.Label)
	}
	if len(stub.members) != 0 {
		t.Fatalf("expected no cluster members, got %#v", stub.members)
	}
}

func TestTriggerExternalPingLegacyInboundSourceDerivesTargetFromRequestHost(t *testing.T) {
	stub := &externalPingServiceStub{}
	router := newExternalPingTestRouter(stub)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/ping/external",
		bytes.NewBufferString(`{"source_ids":["check_host"]}`),
	)
	req.Host = "panel.example.com:8443"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected legacy inbound external ping status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	if !response.Success {
		t.Fatalf("expected legacy inbound external ping success, got %#v", response)
	}
	if !stub.called {
		t.Fatal("expected external ping service to be called")
	}
	if stub.req.Target == nil {
		t.Fatal("expected inbound target to be derived from request host")
	}
	if stub.req.Target.Host != "panel.example.com" {
		t.Fatalf("expected derived target host panel.example.com, got %q", stub.req.Target.Host)
	}
	if stub.req.Target.Port != 8443 {
		t.Fatalf("expected derived target port 8443, got %d", stub.req.Target.Port)
	}
	if stub.req.Target.Label != "panel.example.com" {
		t.Fatalf("expected derived target label panel.example.com, got %q", stub.req.Target.Label)
	}
}

func TestTriggerExternalPingDisabledInboundSourceDoesNotDeriveTarget(t *testing.T) {
	stub := &externalPingServiceStub{}
	router := newExternalPingTestRouter(stub)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/ping/external",
		bytes.NewBufferString(`{"source_ids":["globalping"]}`),
	)
	req.Host = "panel.example.com:8443"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected disabled inbound source status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	if !response.Success {
		t.Fatalf("expected disabled inbound source request success from stub, got %#v", response)
	}
	if !stub.called {
		t.Fatal("expected external ping service to be called")
	}
	if stub.req.Target != nil {
		t.Fatalf("expected disabled inbound source not to derive target, got %#v", stub.req.Target)
	}
}

func TestExternalTargetFromRequestParsesBracketedIPv6WithoutPort(t *testing.T) {
	target, err := externalTargetFromHostForTest(t, "[2001:db8::1]")
	if err != nil {
		t.Fatalf("expected bracketed IPv6 without port to parse, got error %v", err)
	}
	if target.Host != "2001:db8::1" {
		t.Fatalf("expected IPv6 host without brackets, got %q", target.Host)
	}
	if target.Port != 0 {
		t.Fatalf("expected no IPv6 target port, got %d", target.Port)
	}
	if target.Label != "2001:db8::1" {
		t.Fatalf("expected IPv6 label without brackets, got %q", target.Label)
	}
}

func TestExternalTargetFromRequestParsesBracketedIPv6WithPort(t *testing.T) {
	target, err := externalTargetFromHostForTest(t, "[2001:db8::1]:8443")
	if err != nil {
		t.Fatalf("expected bracketed IPv6 with port to parse, got error %v", err)
	}
	if target.Host != "2001:db8::1" {
		t.Fatalf("expected IPv6 host without brackets, got %q", target.Host)
	}
	if target.Port != 8443 {
		t.Fatalf("expected IPv6 target port 8443, got %d", target.Port)
	}
	if target.Label != "2001:db8::1" {
		t.Fatalf("expected IPv6 label without brackets, got %q", target.Label)
	}
}

func TestExternalTargetFromRequestRejectsMalformedMultiColonHost(t *testing.T) {
	target, err := externalTargetFromHostForTest(t, "bad:host:8443")
	if err == nil {
		t.Fatalf("expected malformed multi-colon host error, got target %#v", target)
	}
}

func TestGetMeshPingMissingResultReturnsEmptySuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("b-ui", cookie.NewStore([]byte("test-secret"))))
	handler := &pingAPIHandler{store: ping.NewStore()}
	router.GET("/api/ping/mesh/:domainId", handler.getMeshPing)
	router.GET("/__test/login/:username", func(c *gin.Context) {
		if err := SetLoginUser(c, c.Param("username"), 0); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/ping/mesh/missing-edge-test.invalid", nil)
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected missing cached mesh result to return %d, got %d", http.StatusOK, recorder.Code)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	if !response.Success {
		t.Fatalf("expected success response for empty mesh cache, got %#v", response)
	}
	if response.Obj != nil {
		t.Fatalf("expected empty mesh cache object to be nil, got %#v", response.Obj)
	}
}

func TestGetExternalResultsMissingResultReturnsEmptySuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("b-ui", cookie.NewStore([]byte("test-secret"))))
	handler := &pingAPIHandler{store: ping.NewStore()}
	router.GET("/api/ping/external/results", handler.getExternalResults)
	router.GET("/__test/login/:username", func(c *gin.Context) {
		if err := SetLoginUser(c, c.Param("username"), 0); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/ping/external/results", nil)
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected missing external result to return %d, got %d", http.StatusOK, recorder.Code)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	if !response.Success {
		t.Fatalf("expected success response for empty external cache, got %#v", response)
	}
	if response.Obj == nil {
		t.Fatal("expected empty external cache object, got nil")
	}
}
