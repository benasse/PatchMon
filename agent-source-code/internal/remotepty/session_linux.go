//go:build linux

package remotepty

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

type Session struct {
	cmd    *exec.Cmd
	pty    *os.File
	once   sync.Once
	mu     sync.Mutex
	waitCh chan error
}

func Start(username, terminal string, cols, rows int) (*Session, error) {
	account, err := user.Lookup(username)
	if err != nil {
		return nil, fmt.Errorf("lookup Linux account: %w", err)
	}
	if account.Uid == "0" || username == "root" {
		return nil, errors.New("root login is disabled")
	}
	shell := passwdShell(username)
	if shell == "" {
		shell = "/bin/sh"
	}
	if stat, err := os.Stat(shell); err != nil || stat.IsDir() || stat.Mode()&0111 == 0 {
		return nil, fmt.Errorf("login shell %q is not executable", shell)
	}
	if err := validateLoginShell(shell); err != nil {
		return nil, err
	}
	if terminal == "" {
		terminal = "xterm-256color"
	}
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	suPath, err := findSU()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(suPath, "-", username)
	cmd.Env = []string{
		"TERM=" + terminal,
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}
	sysProcAttr := &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}
	if os.Geteuid() == 0 {
		sysProcAttr.Credential = &syscall.Credential{Uid: 65534, Gid: 65534}
	}
	cmd.SysProcAttr = sysProcAttr
	file, err := pty.StartWithAttrs(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}, sysProcAttr)
	if err != nil {
		return nil, fmt.Errorf("start account PTY: %w", err)
	}
	session := &Session{cmd: cmd, pty: file, waitCh: make(chan error, 1)}
	go func() { session.waitCh <- cmd.Wait() }()
	return session, nil
}

func (s *Session) Output() io.Reader { return s.pty }

// WriteInput returns the terminal ECHO state observed immediately before the
// input was written. Callers must only record the input when echo is true.
func (s *Session) WriteInput(data []byte) (echo bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	termios, ioctlErr := unix.IoctlGetTermios(int(s.pty.Fd()), unix.TCGETS)
	if ioctlErr != nil {
		return false, fmt.Errorf("read PTY echo state: %w", ioctlErr)
	}
	echo = termios.Lflag&unix.ECHO != 0
	_, err = s.pty.Write(data)
	return echo, err
}

func (s *Session) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 || cols > 1000 || rows > 1000 {
		return errors.New("invalid terminal dimensions")
	}
	return pty.Setsize(s.pty, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (s *Session) Signal(name string) error {
	var signal os.Signal
	switch name {
	case "INT":
		signal = os.Interrupt
	case "TERM":
		signal = syscall.SIGTERM
	case "HUP":
		signal = syscall.SIGHUP
	default:
		return errors.New("unsupported signal")
	}
	if s.cmd.Process == nil {
		return errors.New("session process is not running")
	}
	return s.cmd.Process.Signal(signal)
}

func (s *Session) Wait() error { return <-s.waitCh }

func (s *Session) Close() error {
	var closeErr error
	s.once.Do(func() {
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Signal(syscall.SIGHUP)
		}
		closeErr = s.pty.Close()
	})
	return closeErr
}

func passwdShell(username string) string {
	file, err := os.Open("/etc/passwd")
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) >= 7 && fields[0] == username {
			return strings.TrimSpace(fields[6])
		}
	}
	return ""
}

func findSU() (string, error) {
	for _, candidate := range []string{"/bin/su", "/usr/bin/su"} {
		stat, err := os.Stat(candidate)
		if err == nil && !stat.IsDir() && stat.Mode()&0111 != 0 {
			return candidate, nil
		}
	}
	path, err := exec.LookPath("su")
	if err != nil {
		return "", errors.New("su is required for local Linux password authentication")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve su path: %w", err)
	}
	return path, nil
}

func validateLoginShell(shell string) error {
	file, err := os.Open("/etc/shells")
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == shell {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read /etc/shells: %w", err)
	}
	return fmt.Errorf("login shell %q is not listed in /etc/shells", shell)
}
