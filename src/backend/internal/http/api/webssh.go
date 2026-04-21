package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
)

type webSSHClientMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
}

type webSSHServerMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
}

type webSSHSession interface {
	SendInput(input string) error
	Messages() <-chan webSSHServerMessage
	Close() error
}

type webSSHSessionFactory func(ctx context.Context) (webSSHSession, error)

func (a *APIHandler) handleWebSSH(c *gin.Context) {
	username := GetLoginUser(c)
	isAdmin, err := a.ApiService.getUserService().IsFirstUser(username)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, Msg{Success: false, Msg: err.Error()})
		return
	}
	if !isAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, Msg{Success: false, Msg: "admin access required"})
		return
	}

	conn, err := websocket.Accept(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx := c.Request.Context()
	session, err := a.getWebSSHSessionFactory()(ctx)
	if err != nil {
		conn.Close(websocket.StatusInternalError, err.Error())
		return
	}
	defer session.Close()

	readErr := make(chan error, 1)
	go a.readWebSSHMessages(ctx, conn, session, readErr)

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-readErr:
			if err == nil || websocket.CloseStatus(err) == websocket.StatusNormalClosure || websocket.CloseStatus(err) == websocket.StatusGoingAway {
				return
			}
			_ = wsjson.Write(ctx, conn, webSSHServerMessage{Type: "status", Data: err.Error()})
			return
		case message, ok := <-session.Messages():
			if !ok {
				return
			}
			if err := wsjson.Write(ctx, conn, message); err != nil {
				return
			}
		}
	}
}

func (a *APIHandler) readWebSSHMessages(ctx context.Context, conn *websocket.Conn, session webSSHSession, readErr chan<- error) {
	for {
		var message webSSHClientMessage
		if err := wsjson.Read(ctx, conn, &message); err != nil {
			readErr <- err
			return
		}

		switch message.Type {
		case "input":
			if err := session.SendInput(message.Data); err != nil {
				readErr <- err
				return
			}
		default:
			readErr <- errors.New("unsupported webssh message type")
			return
		}
	}
}

func (a *APIHandler) getWebSSHSessionFactory() webSSHSessionFactory {
	if a.webSSHSessionFactory != nil {
		return a.webSSHSessionFactory
	}

	return newLocalWebSSHSessionFactory(&a.ApiService.SettingService)
}
