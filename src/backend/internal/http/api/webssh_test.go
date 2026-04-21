package api

import (
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

type stubUserService struct {
	isFirstUser func(username string) (bool, error)
}

func (s stubUserService) GetUsers() (*[]model.User, error) { return nil, nil }
func (s stubUserService) Login(username string, password string, remoteIP string) (string, error) {
	return "", nil
}
func (s stubUserService) ChangePass(id string, oldPass string, newUser string, newPass string) error {
	return nil
}
func (s stubUserService) LoadTokens() ([]byte, error)                            { return nil, nil }
func (s stubUserService) GetUserTokens(username string) (*[]model.Tokens, error) { return nil, nil }
func (s stubUserService) AddToken(username string, expiry int64, desc string) (string, error) {
	return "", nil
}
func (s stubUserService) DeleteToken(id string) error { return nil }
func (s stubUserService) IsFirstUser(username string) (bool, error) {
	if s.isFirstUser == nil {
		return false, nil
	}
	return s.isFirstUser(username)
}

type authStateResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	Obj     struct {
		Username string `json:"username"`
		IsAdmin  bool   `json:"isAdmin"`
	} `json:"obj"`
}

func TestUserServiceIsFirstUserReturnsFalseForEmptyUsername(t *testing.T) {
	isAdmin, err := (&service.UserService{}).IsFirstUser("")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if isAdmin {
		t.Fatal("expected empty username to not be first user")
	}
}

func TestAPIAuthStateRequiresLogin(t *testing.T) {
	router := newTestAPIRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/authState", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	var response authStateResponse
	decodeResponse(t, recorder, &response)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if response.Success {
		t.Fatal("expected request without session to fail")
	}
	if response.Msg != "Invalid login" {
		t.Fatalf("expected invalid login message, got %q", response.Msg)
	}
}

func TestAPIAuthStateReturnsAdminCapabilityForFirstUser(t *testing.T) {
	router := newTestAPIRouter(stubUserService{isFirstUser: func(username string) (bool, error) {
		return username == "admin", nil
	}})
	cookieHeader := loginCookie(t, router, "admin")

	req := httptest.NewRequest(http.MethodGet, "/api/authState", nil)
	req.Header.Set("Cookie", cookieHeader)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	var response authStateResponse
	decodeResponse(t, recorder, &response)

	if !response.Success {
		t.Fatalf("expected success, got message %q", response.Msg)
	}
	if response.Obj.Username != "admin" {
		t.Fatalf("expected username admin, got %q", response.Obj.Username)
	}
	if !response.Obj.IsAdmin {
		t.Fatal("expected first user to be admin")
	}
}

func TestAPIAuthStateReturnsNonAdminForLaterUser(t *testing.T) {
	router := newTestAPIRouter(stubUserService{isFirstUser: func(username string) (bool, error) {
		return username == "admin", nil
	}})
	cookieHeader := loginCookie(t, router, "operator")

	req := httptest.NewRequest(http.MethodGet, "/api/authState", nil)
	req.Header.Set("Cookie", cookieHeader)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	var response authStateResponse
	decodeResponse(t, recorder, &response)

	if !response.Success {
		t.Fatalf("expected success, got message %q", response.Msg)
	}
	if response.Obj.Username != "operator" {
		t.Fatalf("expected username operator, got %q", response.Obj.Username)
	}
	if response.Obj.IsAdmin {
		t.Fatal("expected later user to not be admin")
	}
}

func TestAPIAuthStateReturnsUserServiceError(t *testing.T) {
	router := newTestAPIRouter(stubUserService{isFirstUser: func(username string) (bool, error) {
		return false, errors.New("boom")
	}})
	cookieHeader := loginCookie(t, router, "admin")

	req := httptest.NewRequest(http.MethodGet, "/api/authState", nil)
	req.Header.Set("Cookie", cookieHeader)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	var response authStateResponse
	decodeResponse(t, recorder, &response)

	if response.Success {
		t.Fatal("expected user service error to fail")
	}
	if response.Msg != ": boom" {
		t.Fatalf("expected propagated user service error, got %q", response.Msg)
	}
}

func newTestAPIRouter(userService apiUserService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger.InitLogger(logging.ERROR)
	router := gin.New()
	router.Use(sessions.Sessions("s-ui", cookie.NewStore([]byte("test-secret"))))
	handler := &APIHandler{}
	handler.ApiService.userService = userService
	handler.initRouter(router.Group("/api"))
	router.GET("/__test/login/:username", func(c *gin.Context) {
		if err := SetLoginUser(c, c.Param("username"), 0); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})
	return router
}

func loginCookie(t *testing.T, router *gin.Engine, username string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/__test/login/"+username, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected login helper status %d, got %d", http.StatusOK, recorder.Code)
	}
	return recorder.Header().Get("Set-Cookie")
}

func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
