package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
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

type stubWebSSHSession struct {
	inputs    []string
	resizes   [][2]int
	inputErr  error
	resizeErr error
	messages  chan webSSHServerMessage
	resizeCh  chan struct{}
	closed    bool
	afterSend func()
}

func newStubWebSSHSession() *stubWebSSHSession {
	return &stubWebSSHSession{
		messages: make(chan webSSHServerMessage, 8),
		resizeCh: make(chan struct{}, 1),
	}
}

func (s *stubWebSSHSession) SendInput(input string) error {
	s.inputs = append(s.inputs, input)
	if s.inputErr != nil {
		return s.inputErr
	}
	s.messages <- webSSHServerMessage{Type: "output", Data: strings.ToUpper(input)}
	s.messages <- webSSHServerMessage{Type: "status", Data: "bridge-open"}
	if s.afterSend != nil {
		s.afterSend()
	}
	return nil
}

func (s *stubWebSSHSession) Messages() <-chan webSSHServerMessage {
	return s.messages
}

func (s *stubWebSSHSession) Resize(cols int, rows int) error {
	s.resizes = append(s.resizes, [2]int{cols, rows})
	select {
	case s.resizeCh <- struct{}{}:
	default:
	}
	if s.resizeErr != nil {
		return s.resizeErr
	}
	return nil
}

func (s *stubWebSSHSession) Close() error {
	if !s.closed {
		s.closed = true
		close(s.messages)
	}
	return nil
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
	router := newTestAPIRouter(nil, nil)

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
	}}, nil)
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
	}}, nil)
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
	}}, nil)
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

func TestAPIWebSSHRejectsNonAdminUsers(t *testing.T) {
	router := newTestAPIRouter(stubUserService{isFirstUser: func(username string) (bool, error) {
		return username == "admin", nil
	}}, nil)
	cookieHeader := loginCookie(t, router, "operator")
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, response, err := websocket.Dial(ctx, strings.Replace(server.URL, "http://", "ws://", 1)+"/api/webssh/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Cookie": []string{cookieHeader},
		},
	})
	if err == nil {
		t.Fatal("expected non-admin websocket request to be rejected")
	}
	if response == nil {
		t.Fatal("expected rejection response")
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.StatusCode)
	}
}

func TestAPIWebSSHForwardsBridgeMessagesForAdminUsers(t *testing.T) {
	session := newStubWebSSHSession()
	router := newTestAPIRouter(stubUserService{isFirstUser: func(username string) (bool, error) {
		return username == "admin", nil
	}}, func(context.Context) (webSSHSession, error) {
		return session, nil
	})
	cookieHeader := loginCookie(t, router, "admin")
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, strings.Replace(server.URL, "http://", "ws://", 1)+"/api/webssh/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Cookie": []string{cookieHeader},
		},
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	if err := wsjson.Write(ctx, conn, webSSHClientMessage{Type: "input", Data: "pwd\n"}); err != nil {
		t.Fatalf("write websocket message: %v", err)
	}

	var output webSSHServerMessage
	if err := wsjson.Read(ctx, conn, &output); err != nil {
		t.Fatalf("read output message: %v", err)
	}
	if output.Type != "output" {
		t.Fatalf("expected output message, got %#v", output)
	}
	if output.Data != "PWD\n" {
		t.Fatalf("expected transformed bridge output, got %q", output.Data)
	}

	var status webSSHServerMessage
	if err := wsjson.Read(ctx, conn, &status); err != nil {
		t.Fatalf("read status message: %v", err)
	}
	if status.Type != "status" {
		t.Fatalf("expected status message, got %#v", status)
	}
	if status.Data != "bridge-open" {
		t.Fatalf("expected bridge status, got %q", status.Data)
	}

	if len(session.inputs) != 1 || session.inputs[0] != "pwd\n" {
		t.Fatalf("expected bridge input to be forwarded, got %#v", session.inputs)
	}
}

func TestAPIWebSSHReturnsBridgeErrorStatus(t *testing.T) {
	session := newStubWebSSHSession()
	session.inputErr = errors.New("bridge write failed")
	router := newTestAPIRouter(stubUserService{isFirstUser: func(username string) (bool, error) {
		return username == "admin", nil
	}}, func(context.Context) (webSSHSession, error) {
		return session, nil
	})
	cookieHeader := loginCookie(t, router, "admin")
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, strings.Replace(server.URL, "http://", "ws://", 1)+"/api/webssh/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Cookie": []string{cookieHeader},
		},
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	if err := wsjson.Write(ctx, conn, webSSHClientMessage{Type: "input", Data: "pwd\n"}); err != nil {
		t.Fatalf("write websocket message: %v", err)
	}

	var status webSSHServerMessage
	if err := wsjson.Read(ctx, conn, &status); err != nil {
		t.Fatalf("read status message: %v", err)
	}
	if status.Type != "status" {
		t.Fatalf("expected status message, got %#v", status)
	}
	if status.Data != "bridge write failed" {
		t.Fatalf("expected bridge error status, got %q", status.Data)
	}
}

func TestAPIWebSSHForwardsResizeMessagesForAdminUsers(t *testing.T) {
	session := newStubWebSSHSession()
	router := newTestAPIRouter(stubUserService{isFirstUser: func(username string) (bool, error) {
		return username == "admin", nil
	}}, func(context.Context) (webSSHSession, error) {
		return session, nil
	})
	cookieHeader := loginCookie(t, router, "admin")
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, strings.Replace(server.URL, "http://", "ws://", 1)+"/api/webssh/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Cookie": []string{cookieHeader},
		},
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	if err := wsjson.Write(ctx, conn, webSSHClientMessage{Type: "resize", Cols: 120, Rows: 36}); err != nil {
		t.Fatalf("write resize websocket message: %v", err)
	}

	select {
	case <-session.resizeCh:
	case <-time.After(1 * time.Second):
		t.Fatal("expected resize message to reach bridge")
	}

	if len(session.resizes) != 1 {
		t.Fatalf("expected one resize forwarded, got %#v", session.resizes)
	}
	if session.resizes[0][0] != 120 || session.resizes[0][1] != 36 {
		t.Fatalf("expected resize dimensions 120x36, got %#v", session.resizes[0])
	}
}

func TestAPIWebSSHReturnsIdleTimeoutStatusBeforeClosing(t *testing.T) {
	session := newStubWebSSHSession()
	session.afterSend = func() {
		session.messages <- webSSHServerMessage{Type: "status", Data: "idle-timeout"}
		_ = session.Close()
	}
	router := newTestAPIRouter(stubUserService{isFirstUser: func(username string) (bool, error) {
		return username == "admin", nil
	}}, func(context.Context) (webSSHSession, error) {
		return session, nil
	})
	cookieHeader := loginCookie(t, router, "admin")
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, strings.Replace(server.URL, "http://", "ws://", 1)+"/api/webssh/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Cookie": []string{cookieHeader},
		},
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	if err := wsjson.Write(ctx, conn, webSSHClientMessage{Type: "input", Data: "pwd\n"}); err != nil {
		t.Fatalf("write websocket message: %v", err)
	}

	var output webSSHServerMessage
	if err := wsjson.Read(ctx, conn, &output); err != nil {
		t.Fatalf("read output message: %v", err)
	}
	if output.Type != "output" {
		t.Fatalf("expected output message, got %#v", output)
	}

	var status webSSHServerMessage
	if err := wsjson.Read(ctx, conn, &status); err != nil {
		t.Fatalf("read bridge-open status: %v", err)
	}
	if status.Type != "status" || status.Data != "bridge-open" {
		t.Fatalf("expected initial bridge-open status, got %#v", status)
	}

	if err := wsjson.Read(ctx, conn, &status); err != nil {
		t.Fatalf("read idle-timeout status: %v", err)
	}
	if status.Type != "status" || status.Data != "idle-timeout" {
		t.Fatalf("expected idle-timeout status, got %#v", status)
	}

	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("expected websocket to close after idle-timeout")
	}
}

func newTestAPIRouter(userService apiUserService, webSSHFactory webSSHSessionFactory) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger.InitLogger(logging.ERROR)
	router := gin.New()
	router.Use(sessions.Sessions("b-ui", cookie.NewStore([]byte("test-secret"))))
	handler := &APIHandler{}
	handler.ApiService.userService = userService
	handler.webSSHSessionFactory = webSSHFactory
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
