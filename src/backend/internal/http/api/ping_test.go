package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/ping"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

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
