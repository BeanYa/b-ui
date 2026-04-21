package api

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type stubWebSSHSettings struct {
	host        string
	port        int
	username    string
	password    string
	idleTimeout time.Duration
}

func (s stubWebSSHSettings) GetWebSSHHost() (string, error)               { return s.host, nil }
func (s stubWebSSHSettings) GetWebSSHPort() (int, error)                  { return s.port, nil }
func (s stubWebSSHSettings) GetWebSSHUsername() (string, error)           { return s.username, nil }
func (s stubWebSSHSettings) GetWebSSHPassword() (string, error)           { return s.password, nil }
func (s stubWebSSHSettings) GetWebSSHIdleTimeout() (time.Duration, error) { return s.idleTimeout, nil }

type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (nopWriteCloser) Close() error                { return nil }

type recordingWriteCloser struct {
	writes [][]byte
	closed bool
}

func (r *recordingWriteCloser) Write(p []byte) (int, error) {
	clone := append([]byte(nil), p...)
	r.writes = append(r.writes, clone)
	return len(p), nil
}

func (r *recordingWriteCloser) Close() error {
	r.closed = true
	return nil
}

func TestSSHWebSSHSessionCloseUnblocksInFlightSender(t *testing.T) {
	session := &sshWebSSHSession{
		stdin:    nopWriteCloser{},
		messages: make(chan webSSHServerMessage),
		done:     make(chan struct{}),
	}

	senderDone := make(chan interface{}, 1)
	go func() {
		defer func() {
			senderDone <- recover()
		}()
		session.sendMessage(webSSHServerMessage{Type: "status", Data: "late"})
	}()

	time.Sleep(20 * time.Millisecond)

	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)
		_ = session.Close()
	}()

	select {
	case <-closeDone:
	case <-time.After(1 * time.Second):
		t.Fatal("expected Close to unblock in-flight sender")
	}

	if recovered := <-senderDone; recovered != nil {
		t.Fatalf("expected no panic from in-flight sender, got %v", recovered)
	}

	select {
	case _, ok := <-session.Messages():
		if ok {
			t.Fatal("expected messages channel to be closed after teardown")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("expected messages channel to close after teardown")
	}
}

func TestSSHWebSSHSessionIgnoresStaleIdleTimeoutCallbackAfterActivity(t *testing.T) {
	session := &sshWebSSHSession{
		stdin:       nopWriteCloser{},
		messages:    make(chan webSSHServerMessage, 2),
		done:        make(chan struct{}),
		idleTimeout: time.Second,
	}

	firstGeneration := session.startIdleTimer()
	secondGeneration := session.resetIdleTimer()
	if secondGeneration == firstGeneration {
		t.Fatal("expected activity to advance idle timer generation")
	}

	session.handleIdleTimeout(firstGeneration)

	select {
	case <-session.done:
		t.Fatal("expected stale idle-timeout callback to leave session active")
	default:
	}

	select {
	case message := <-session.Messages():
		t.Fatalf("expected no stale idle-timeout message, got %#v", message)
	default:
	}

	_ = session.Close()
}

func TestSSHWebSSHSessionTimeoutClaimsTimerStateBeforeConcurrentReset(t *testing.T) {
	session := &sshWebSSHSession{
		stdin:       nopWriteCloser{},
		messages:    make(chan webSSHServerMessage, 2),
		done:        make(chan struct{}),
		idleTimeout: time.Second,
	}

	generation := session.startIdleTimer()
	resetResult := make(chan uint64, 1)
	session.beforeIdleTimeoutUnlock = func() {
		go func() {
			resetResult <- session.resetIdleTimer()
		}()

		select {
		case result := <-resetResult:
			t.Fatalf("expected concurrent reset to block until timeout claims timer state, got %d", result)
		default:
		}
	}

	session.handleIdleTimeout(generation)

	select {
	case result := <-resetResult:
		if result != 0 {
			t.Fatalf("expected reset after claimed timeout to observe inactive timer, got %d", result)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("expected blocked reset to complete after timeout handling")
	}

	select {
	case <-session.done:
	default:
		t.Fatal("expected timeout callback to close session")
	}

	select {
	case message := <-session.Messages():
		if message.Type != "status" || message.Data != "idle-timeout" {
			t.Fatalf("expected idle-timeout status message, got %#v", message)
		}
	default:
		t.Fatal("expected idle-timeout status message")
	}
}

func TestSSHWebSSHSessionRejectsInputAfterTimeoutClaimBeforeClose(t *testing.T) {
	stdin := &recordingWriteCloser{}
	session := &sshWebSSHSession{
		stdin:       stdin,
		messages:    make(chan webSSHServerMessage, 2),
		done:        make(chan struct{}),
		idleTimeout: time.Second,
	}

	generation := session.startIdleTimer()
	inputResult := make(chan error, 1)
	session.beforeIdleTimeoutClose = func() {
		go func() {
			inputResult <- session.SendInput("pwd\n")
		}()
	}

	session.handleIdleTimeout(generation)

	select {
	case err := <-inputResult:
		if err == nil {
			t.Fatal("expected input after timeout claim to be rejected")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("expected concurrent input attempt to complete")
	}

	if len(stdin.writes) != 0 {
		t.Fatalf("expected no stdin writes after timeout claim, got %#v", stdin.writes)
	}
}

func TestSSHWebSSHSessionOutputDoesNotRefreshIdleTimeout(t *testing.T) {
	session := &sshWebSSHSession{
		stdin:       nopWriteCloser{},
		messages:    make(chan webSSHServerMessage, 4),
		done:        make(chan struct{}),
		idleTimeout: time.Second,
	}

	generation := session.startIdleTimer()
	session.forwardOutput(bytes.NewBufferString("remote output"))
	session.handleIdleTimeout(generation)

	select {
	case <-session.done:
	default:
		t.Fatal("expected idle timeout to still close session after server-only output")
	}

	message := <-session.Messages()
	if message.Type != "output" || message.Data != "remote output" {
		t.Fatalf("expected forwarded output message, got %#v", message)
	}

	message = <-session.Messages()
	if message.Type != "status" || message.Data != "idle-timeout" {
		t.Fatalf("expected idle-timeout status after output-only activity, got %#v", message)
	}
}

func TestNewSSHWebSSHSessionFactoryUsesDedicatedConnectTimeout(t *testing.T) {
	settings := stubWebSSHSettings{
		host:        "127.0.0.1",
		port:        22,
		username:    "root",
		password:    "secret",
		idleTimeout: 37 * time.Second,
	}

	called := false
	originalDial := sshDialContextClient
	sshDialContextClient = func(ctx context.Context, network string, address string, config *ssh.ClientConfig) (*ssh.Client, error) {
		called = true
		if network != "tcp" {
			t.Fatalf("expected tcp network, got %q", network)
		}
		if address != "127.0.0.1:22" {
			t.Fatalf("expected SSH address, got %q", address)
		}
		if config.Timeout != defaultWebSSHConnectTimeout {
			t.Fatalf("expected dedicated connect timeout %v, got %v", defaultWebSSHConnectTimeout, config.Timeout)
		}
		return nil, io.EOF
	}
	defer func() {
		sshDialContextClient = originalDial
	}()

	_, err := newSSHWebSSHSessionFactory(settings)(context.Background())
	if err != io.EOF {
		t.Fatalf("expected dial error to be returned, got %v", err)
	}
	if !called {
		t.Fatal("expected SSH dialer to be invoked")
	}
}

func TestNewSSHWebSSHSessionFactoryCancelsDialWhenContextEnds(t *testing.T) {
	settings := stubWebSSHSettings{
		host:        "127.0.0.1",
		port:        22,
		username:    "root",
		password:    "secret",
		idleTimeout: 37 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	originalDial := sshDialContextClient
	sshDialContextClient = func(ctx context.Context, network string, address string, config *ssh.ClientConfig) (*ssh.Client, error) {
		called = true
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1 * time.Second):
			t.Fatal("expected canceled context to stop SSH dial promptly")
			return nil, nil
		}
	}
	defer func() {
		sshDialContextClient = originalDial
	}()

	_, err := newSSHWebSSHSessionFactory(settings)(ctx)
	if err != context.Canceled {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
	if !called {
		t.Fatal("expected SSH dialer to be invoked")
	}
}
