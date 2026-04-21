package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const defaultWebSSHConnectTimeout = 10 * time.Second

var sshDialContextClient = dialSSHClientContext

type webSSHSettings interface {
	GetWebSSHHost() (string, error)
	GetWebSSHPort() (int, error)
	GetWebSSHUsername() (string, error)
	GetWebSSHPassword() (string, error)
	GetWebSSHIdleTimeout() (time.Duration, error)
}

type sshWebSSHSession struct {
	client                  *ssh.Client
	sshSession              *ssh.Session
	stdin                   io.WriteCloser
	messages                chan webSSHServerMessage
	idleTimeout             time.Duration
	timer                   *time.Timer
	timerMu                 sync.Mutex
	timerGen                uint64
	stateMu                 sync.RWMutex
	expired                 bool
	beforeIdleTimeoutUnlock func()
	beforeIdleTimeoutClose  func()
	messagesMu              sync.RWMutex
	closeOnce               sync.Once
	done                    chan struct{}
}

func newSSHWebSSHSessionFactory(settings webSSHSettings) webSSHSessionFactory {
	return func(ctx context.Context) (webSSHSession, error) {
		host, err := settings.GetWebSSHHost()
		if err != nil {
			return nil, err
		}
		port, err := settings.GetWebSSHPort()
		if err != nil {
			return nil, err
		}
		username, err := settings.GetWebSSHUsername()
		if err != nil {
			return nil, err
		}
		password, err := settings.GetWebSSHPassword()
		if err != nil {
			return nil, err
		}
		idleTimeout, err := settings.GetWebSSHIdleTimeout()
		if err != nil {
			return nil, err
		}

		if host == "" || username == "" || password == "" {
			return nil, fmt.Errorf("webssh settings are incomplete")
		}

		sshConfig := newSSHClientConfig(username, password)

		client, err := sshDialContextClient(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)), sshConfig)
		if err != nil {
			return nil, err
		}

		sshSession, err := client.NewSession()
		if err != nil {
			client.Close()
			return nil, err
		}

		stdin, err := sshSession.StdinPipe()
		if err != nil {
			sshSession.Close()
			client.Close()
			return nil, err
		}
		stdout, err := sshSession.StdoutPipe()
		if err != nil {
			sshSession.Close()
			client.Close()
			return nil, err
		}
		stderr, err := sshSession.StderrPipe()
		if err != nil {
			sshSession.Close()
			client.Close()
			return nil, err
		}

		if err := sshSession.RequestPty("xterm", 40, 120, ssh.TerminalModes{}); err != nil {
			sshSession.Close()
			client.Close()
			return nil, err
		}
		if err := sshSession.Shell(); err != nil {
			sshSession.Close()
			client.Close()
			return nil, err
		}

		session := &sshWebSSHSession{
			client:      client,
			sshSession:  sshSession,
			stdin:       stdin,
			messages:    make(chan webSSHServerMessage, 32),
			idleTimeout: idleTimeout,
			done:        make(chan struct{}),
		}
		session.startIdleTimer()
		session.sendMessage(webSSHServerMessage{Type: "status", Data: "connected"})

		go session.forwardOutput(stdout)
		go session.forwardOutput(stderr)
		go session.waitForExit()
		go func() {
			<-ctx.Done()
			session.Close()
		}()

		return session, nil
	}
}

func newSSHClientConfig(username string, password string) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         defaultWebSSHConnectTimeout,
	}
}

func dialSSHClientContext(ctx context.Context, network string, address string, config *ssh.ClientConfig) (*ssh.Client, error) {
	dialer := &net.Dialer{Timeout: config.Timeout}
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return ssh.NewClient(clientConn, chans, reqs), nil
}

func (s *sshWebSSHSession) SendInput(input string) error {
	if !s.canDeliverIO() {
		return errors.New("webssh session closed")
	}
	s.resetIdleTimer()
	if !s.canDeliverIO() {
		return errors.New("webssh session closed")
	}
	_, err := io.WriteString(s.stdin, input)
	return err
}

func (s *sshWebSSHSession) Messages() <-chan webSSHServerMessage {
	return s.messages
}

func (s *sshWebSSHSession) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.markExpired()
		close(s.done)
		s.timerMu.Lock()
		s.timerGen++
		if s.timer != nil {
			s.timer.Stop()
			s.timer = nil
		}
		s.timerMu.Unlock()
		if s.stdin != nil {
			if closeErr := s.stdin.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}
		if s.sshSession != nil {
			if closeErr := s.sshSession.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}
		if s.client != nil {
			if closeErr := s.client.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}
		s.messagesMu.Lock()
		defer s.messagesMu.Unlock()
		if s.messages != nil {
			close(s.messages)
		}
	})
	return err
}

func (s *sshWebSSHSession) waitForExit() {
	if s.sshSession == nil {
		s.sendMessage(webSSHServerMessage{Type: "status", Data: "closed"})
		_ = s.Close()
		return
	}
	err := s.sshSession.Wait()
	if err != nil {
		s.sendMessage(webSSHServerMessage{Type: "status", Data: err.Error()})
	} else {
		s.sendMessage(webSSHServerMessage{Type: "status", Data: "closed"})
	}
	_ = s.Close()
}

func (s *sshWebSSHSession) forwardOutput(reader io.Reader) {
	buffer := make([]byte, 4096)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			if !s.canDeliverIO() {
				return
			}
			s.sendMessage(webSSHServerMessage{Type: "output", Data: string(buffer[:n])})
		}
		if err != nil {
			if err != io.EOF {
				s.sendMessage(webSSHServerMessage{Type: "status", Data: err.Error()})
			}
			return
		}
	}
}

func (s *sshWebSSHSession) startIdleTimer() uint64 {
	if s.idleTimeout <= 0 {
		return 0
	}
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if s.timer == nil {
		return s.scheduleIdleTimerLocked()
	}
	return s.timerGen
}

func (s *sshWebSSHSession) resetIdleTimer() uint64 {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if s.timer == nil {
		return 0
	}
	_ = s.timer.Stop()
	return s.scheduleIdleTimerLocked()
}

func (s *sshWebSSHSession) scheduleIdleTimerLocked() uint64 {
	s.timerGen++
	generation := s.timerGen
	s.timer = time.AfterFunc(s.idleTimeout, func() {
		s.handleIdleTimeout(generation)
	})
	return generation
}

func (s *sshWebSSHSession) handleIdleTimeout(generation uint64) {
	s.timerMu.Lock()
	if generation != s.timerGen {
		s.timerMu.Unlock()
		return
	}
	s.timerGen++
	s.timer = nil
	s.markExpiredLocked()
	if s.beforeIdleTimeoutUnlock != nil {
		s.beforeIdleTimeoutUnlock()
	}
	s.timerMu.Unlock()
	if s.beforeIdleTimeoutClose != nil {
		s.beforeIdleTimeoutClose()
	}

	s.sendMessage(webSSHServerMessage{Type: "status", Data: "idle-timeout"})
	_ = s.Close()
}

func (s *sshWebSSHSession) canDeliverIO() bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return !s.expired
}

func (s *sshWebSSHSession) markExpired() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.expired = true
}

func (s *sshWebSSHSession) markExpiredLocked() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.expired = true
}

func (s *sshWebSSHSession) sendMessage(message webSSHServerMessage) {
	s.messagesMu.RLock()
	defer s.messagesMu.RUnlock()
	if s.messages == nil {
		return
	}
	select {
	case <-s.done:
		return
	case s.messages <- message:
	}
}
