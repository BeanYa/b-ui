package api

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"
)

type stubWebSSHSettings struct {
	idleTimeout         time.Duration
	terminalIdleTimeout time.Duration
}

func (s stubWebSSHSettings) GetWebSSHIdleTimeout() (time.Duration, error) { return s.idleTimeout, nil }
func (s stubWebSSHSettings) GetWebTerminalIdleTimeout() (time.Duration, error) {
	return s.terminalIdleTimeout, nil
}

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

type blockingWebSSHProcess struct {
	stdin  io.WriteCloser
	stdout *io.PipeReader
	stderr *io.PipeReader
	waitCh chan struct{}
}

func newBlockingWebSSHProcess() *blockingWebSSHProcess {
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	return &blockingWebSSHProcess{
		stdin:  &recordingWriteCloser{},
		stdout: stdoutReader,
		stderr: stderrReader,
		waitCh: make(chan struct{}),
	}
}

func (p *blockingWebSSHProcess) InputPipe() (io.WriteCloser, error) { return p.stdin, nil }
func (p *blockingWebSSHProcess) OutputPipe() (io.ReadCloser, error) { return p.stdout, nil }
func (p *blockingWebSSHProcess) Start() error                       { return nil }
func (p *blockingWebSSHProcess) Wait() error {
	<-p.waitCh
	return nil
}
func (p *blockingWebSSHProcess) Kill() error {
	select {
	case <-p.waitCh:
	default:
		close(p.waitCh)
	}
	return nil
}

func TestLocalWebSSHSessionCloseUnblocksInFlightSender(t *testing.T) {
	session := &localWebSSHSession{
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

func TestLocalWebSSHSessionIgnoresStaleIdleTimeoutCallbackAfterActivity(t *testing.T) {
	session := &localWebSSHSession{
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

func TestLocalWebSSHSessionTimeoutClaimsTimerStateBeforeConcurrentReset(t *testing.T) {
	session := &localWebSSHSession{
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

func TestLocalWebSSHSessionRejectsInputAfterTimeoutClaimBeforeClose(t *testing.T) {
	stdin := &recordingWriteCloser{}
	session := &localWebSSHSession{
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

func TestLocalWebSSHSessionOutputDoesNotRefreshIdleTimeout(t *testing.T) {
	session := &localWebSSHSession{
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

func TestNewLocalWebSSHSessionFactoryStartsConfiguredShell(t *testing.T) {
	settings := stubWebSSHSettings{
		idleTimeout:         37 * time.Second,
		terminalIdleTimeout: 37 * time.Second,
	}

	called := false
	originalShellCommand := webSSHShellCommand
	originalStart := startWebSSHProcess
	webSSHShellCommand = func() (string, []string) {
		return "/bin/test-shell", []string{"-l", "-c"}
	}
	startWebSSHProcess = func(ctx context.Context, name string, args ...string) (webSSHProcess, error) {
		called = true
		if name != "/bin/test-shell" {
			t.Fatalf("expected shell path to be forwarded, got %q", name)
		}
		if len(args) != 2 || args[0] != "-l" || args[1] != "-c" {
			t.Fatalf("expected shell args to be forwarded, got %#v", args)
		}
		return nil, io.EOF
	}
	defer func() {
		webSSHShellCommand = originalShellCommand
		startWebSSHProcess = originalStart
	}()

	_, err := newLocalWebSSHSessionFactory(settings)(context.Background())
	if err != io.EOF {
		t.Fatalf("expected startup error to be returned, got %v", err)
	}
	if !called {
		t.Fatal("expected local shell starter to be invoked")
	}
}

func TestNewLocalWebSSHSessionFactoryCancelsStartWhenContextEnds(t *testing.T) {
	settings := stubWebSSHSettings{
		idleTimeout:         37 * time.Second,
		terminalIdleTimeout: 37 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	originalStart := startWebSSHProcess
	startWebSSHProcess = func(ctx context.Context, name string, args ...string) (webSSHProcess, error) {
		called = true
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1 * time.Second):
			t.Fatal("expected canceled context to stop shell startup promptly")
			return nil, nil
		}
	}
	defer func() {
		startWebSSHProcess = originalStart
	}()

	_, err := newLocalWebSSHSessionFactory(settings)(ctx)
	if err != context.Canceled {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
	if !called {
		t.Fatal("expected shell starter to be invoked")
	}
}

func TestNewLocalWebSSHSessionFactoryUsesTerminalIdleTimeoutSetting(t *testing.T) {
	settings := stubWebSSHSettings{
		idleTimeout:         11 * time.Second,
		terminalIdleTimeout: 29 * time.Second,
	}

	process := newBlockingWebSSHProcess()
	originalStart := startWebSSHProcess
	startWebSSHProcess = func(ctx context.Context, name string, args ...string) (webSSHProcess, error) {
		return process, nil
	}
	defer func() {
		startWebSSHProcess = originalStart
	}()

	rawSession, err := newLocalWebSSHSessionFactory(settings)(context.Background())
	if err != nil {
		t.Fatalf("expected session factory to succeed, got %v", err)
	}
	session, ok := rawSession.(*localWebSSHSession)
	if !ok {
		t.Fatalf("expected local session type, got %T", rawSession)
	}
	defer session.Close()

	if session.idleTimeout != 29*time.Second {
		t.Fatalf("expected terminal idle timeout to be used, got %v", session.idleTimeout)
	}
}

func TestNewLocalWebSSHSessionFactoryLaunchesTTYBackedShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty behavior test is unix-only")
	}

	settings := stubWebSSHSettings{terminalIdleTimeout: 5 * time.Second}
	originalShellCommand := webSSHShellCommand
	webSSHShellCommand = func() (string, []string) {
		return "/bin/sh", []string{"-c", "if test -t 0; then printf PTY-YES; else printf PTY-NO; fi"}
	}
	defer func() {
		webSSHShellCommand = originalShellCommand
	}()

	session, err := newLocalWebSSHSessionFactory(settings)(context.Background())
	if err != nil {
		t.Fatalf("expected session factory to succeed, got %v", err)
	}
	defer session.Close()

	var output strings.Builder
	deadline := time.After(3 * time.Second)
	for {
		select {
		case message, ok := <-session.Messages():
			if !ok {
				if strings.Contains(output.String(), "PTY-YES") {
					return
				}
				t.Fatalf("expected shell output to confirm tty-backed stdin, got %q", output.String())
			}
			if message.Type == "output" {
				output.WriteString(message.Data)
				if strings.Contains(output.String(), "PTY-YES") {
					return
				}
			}
		case <-deadline:
			t.Fatalf("timed out waiting for tty-backed shell output, got %q", output.String())
		}
	}
}
