package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestJsonObjErrorWithoutMessageUsesErrorText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		jsonObj(c, nil, errors.New("latest release unavailable"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	if response.Success {
		t.Fatalf("expected failure response, got %#v", response)
	}
	if response.Msg != "latest release unavailable" {
		t.Fatalf("expected bare error message, got %q", response.Msg)
	}
}

func TestJsonMsgErrorWithMessageKeepsMessagePrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		jsonMsg(c, "save", errors.New("disk full"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response Msg
	decodeResponse(t, recorder, &response)
	if response.Success {
		t.Fatalf("expected failure response, got %#v", response)
	}
	if response.Msg != "save: disk full" {
		t.Fatalf("expected prefixed error message, got %q", response.Msg)
	}
}
