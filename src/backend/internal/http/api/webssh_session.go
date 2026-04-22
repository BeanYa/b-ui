package api

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/creack/pty"
)

var webSSHShellCommand = defaultWebSSHShellCommand
var startWebSSHProcess = newExecWebSSHProcess

type webSSHSettings interface {
	GetWebTerminalIdleTimeout() (time.Duration, error)
}

type webSSHProcess interface {
	InputPipe() (io.WriteCloser, error)
	OutputPipe() (io.ReadCloser, error)
	Start() error
	Resize(cols int, rows int) error
	Wait() error
	Kill() error
}

type execWebSSHProcess struct {
	cmd    *exec.Cmd
	input  io.WriteCloser
	output io.ReadCloser
	pty    *os.File
}

type localWebSSHSession struct {
	process                 webSSHProcess
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

func newLocalWebSSHSessionFactory(settings webSSHSettings) webSSHSessionFactory {
	return func(ctx context.Context) (webSSHSession, error) {
		idleTimeout, err := settings.GetWebTerminalIdleTimeout()
		if err != nil {
			return nil, err
		}

		shellName, shellArgs := webSSHShellCommand()
		process, err := startWebSSHProcess(ctx, shellName, shellArgs...)
		if err != nil {
			return nil, err
		}

		if err := process.Start(); err != nil {
			return nil, err
		}

		stdin, err := process.InputPipe()
		if err != nil {
			_ = process.Kill()
			return nil, err
		}
		output, err := process.OutputPipe()
		if err != nil {
			_ = stdin.Close()
			_ = process.Kill()
			return nil, err
		}

		session := &localWebSSHSession{
			process:     process,
			stdin:       stdin,
			messages:    make(chan webSSHServerMessage, 32),
			idleTimeout: idleTimeout,
			done:        make(chan struct{}),
		}
		session.startIdleTimer()
		session.sendMessage(webSSHServerMessage{Type: "status", Data: "connected"})

		go session.forwardOutput(output)
		go session.waitForExit()
		go func() {
			<-ctx.Done()
			session.Close()
		}()

		return session, nil
	}
}

func defaultWebSSHShellCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", nil
	}
	return "/bin/sh", nil
}

func newExecWebSSHProcess(ctx context.Context, name string, args ...string) (webSSHProcess, error) {
	return &execWebSSHProcess{cmd: exec.CommandContext(ctx, name, args...)}, nil
}

func (p *execWebSSHProcess) InputPipe() (io.WriteCloser, error) {
	if p.input == nil {
		return nil, errors.New("web terminal input unavailable")
	}
	return p.input, nil
}

func (p *execWebSSHProcess) OutputPipe() (io.ReadCloser, error) {
	if p.output == nil {
		return nil, errors.New("web terminal output unavailable")
	}
	return p.output, nil
}

func (p *execWebSSHProcess) Start() error {
	if runtime.GOOS == "windows" {
		stdin, err := p.cmd.StdinPipe()
		if err != nil {
			return err
		}
		stdout, err := p.cmd.StdoutPipe()
		if err != nil {
			_ = stdin.Close()
			return err
		}
		stderr, err := p.cmd.StderrPipe()
		if err != nil {
			_ = stdin.Close()
			_ = stdout.Close()
			return err
		}
		if err := p.cmd.Start(); err != nil {
			_ = stdin.Close()
			_ = stdout.Close()
			_ = stderr.Close()
			return err
		}
		p.input = stdin
		p.output = &mergedReadCloser{readers: []io.ReadCloser{stdout, stderr}}
		return nil
	}

	terminal, err := pty.Start(p.cmd)
	if err != nil {
		return err
	}
	p.input = terminal
	p.output = terminal
	p.pty = terminal
	return nil
}

func (p *execWebSSHProcess) Resize(cols int, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	if p.pty == nil {
		return nil
	}
	return pty.Setsize(p.pty, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (p *execWebSSHProcess) Wait() error {
	return p.cmd.Wait()
}

func (p *execWebSSHProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

type mergedReadCloser struct {
	readers   []io.ReadCloser
	reader    *io.PipeReader
	writer    *io.PipeWriter
	initOnce  sync.Once
	closeOnce sync.Once
	writeMu   sync.Mutex
	errMu     sync.Mutex
	wg        sync.WaitGroup
	firstErr  error
}

func (r *mergedReadCloser) Read(p []byte) (int, error) {
	r.startFanIn()
	return r.reader.Read(p)
}

func (r *mergedReadCloser) Close() error {
	var firstErr error
	r.closeOnce.Do(func() {
		for _, reader := range r.readers {
			if reader == nil {
				continue
			}
			if err := reader.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if r.writer != nil {
			_ = r.writer.Close()
		}
		if r.reader != nil {
			_ = r.reader.Close()
		}
	})
	return firstErr
}

func (r *mergedReadCloser) startFanIn() {
	r.initOnce.Do(func() {
		r.reader, r.writer = io.Pipe()
		for _, reader := range r.readers {
			if reader == nil {
				continue
			}

			r.wg.Add(1)
			go func(reader io.ReadCloser) {
				defer r.wg.Done()
				defer reader.Close()

				buffer := make([]byte, 4096)
				for {
					n, err := reader.Read(buffer)
					if n > 0 {
						r.writeMu.Lock()
						_, writeErr := r.writer.Write(buffer[:n])
						r.writeMu.Unlock()
						if writeErr != nil {
							return
						}
					}
					if err != nil {
						if err != io.EOF {
							r.errMu.Lock()
							if r.firstErr == nil {
								r.firstErr = err
							}
							r.errMu.Unlock()
						}
						return
					}
				}
			}(reader)
		}

		go func() {
			r.wg.Wait()
			r.errMu.Lock()
			defer r.errMu.Unlock()
			_ = r.writer.CloseWithError(r.firstErr)
		}()
	})
}

func (s *localWebSSHSession) SendInput(input string) error {
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

func (s *localWebSSHSession) Resize(cols int, rows int) error {
	if !s.canDeliverIO() {
		return errors.New("webssh session closed")
	}
	if cols <= 0 || rows <= 0 {
		return nil
	}
	s.resetIdleTimer()
	if !s.canDeliverIO() {
		return errors.New("webssh session closed")
	}
	if s.process == nil {
		return errors.New("webssh session closed")
	}
	return s.process.Resize(cols, rows)
}

func (s *localWebSSHSession) Messages() <-chan webSSHServerMessage {
	return s.messages
}

func (s *localWebSSHSession) Close() error {
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
		if s.process != nil {
			if killErr := s.process.Kill(); killErr != nil && err == nil {
				err = killErr
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

func (s *localWebSSHSession) waitForExit() {
	if s.process == nil {
		s.sendMessage(webSSHServerMessage{Type: "status", Data: "closed"})
		_ = s.Close()
		return
	}
	if err := s.process.Wait(); err != nil {
		if s.canDeliverIO() {
			s.sendMessage(webSSHServerMessage{Type: "status", Data: err.Error()})
		}
	} else if s.canDeliverIO() {
		s.sendMessage(webSSHServerMessage{Type: "status", Data: "closed"})
	}
	_ = s.Close()
}

func (s *localWebSSHSession) forwardOutput(reader io.Reader) {
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
			if err != io.EOF && s.canDeliverIO() {
				s.sendMessage(webSSHServerMessage{Type: "status", Data: err.Error()})
			}
			return
		}
	}
}

func (s *localWebSSHSession) startIdleTimer() uint64 {
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

func (s *localWebSSHSession) resetIdleTimer() uint64 {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if s.timer == nil {
		return 0
	}
	_ = s.timer.Stop()
	return s.scheduleIdleTimerLocked()
}

func (s *localWebSSHSession) scheduleIdleTimerLocked() uint64 {
	s.timerGen++
	generation := s.timerGen
	s.timer = time.AfterFunc(s.idleTimeout, func() {
		s.handleIdleTimeout(generation)
	})
	return generation
}

func (s *localWebSSHSession) handleIdleTimeout(generation uint64) {
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

func (s *localWebSSHSession) canDeliverIO() bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return !s.expired
}

func (s *localWebSSHSession) markExpired() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.expired = true
}

func (s *localWebSSHSession) markExpiredLocked() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.expired = true
}

func (s *localWebSSHSession) sendMessage(message webSSHServerMessage) {
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
