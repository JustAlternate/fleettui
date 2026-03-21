package tui

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/justalternate/fleettui/internal/domain"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshSession holds the components for an interactive SSH session.
type sshSession struct {
	client  *ssh.Client
	session *ssh.Session
	stdinW  *os.File // write end — user input goes here
	stdoutR *os.File // read end — SSH output comes from here
}

type logsStreamSession struct {
	client  *ssh.Client
	session *ssh.Session
	stdoutR *os.File
}

// connectSSH opens an interactive SSH session to the given node with PTY
// allocation. It returns pipes for stdin/stdout and the SSH client/session
// for lifecycle management.
func connectSSH(node *domain.Node, cols, rows int) (*sshSession, error) {
	config, err := buildSSHConfig(node)
	if err != nil {
		return nil, fmt.Errorf("ssh config: %w", err)
	}

	host, port := parseHostPort(node.IP)
	addr := host + ":22"
	if port != "" {
		addr = host + ":" + port
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("ssh session: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("request pty: %w", err)
	}

	// Create pipes: one end goes to SSH, the other to the terminal emulator.
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		stdinR.Close()
		stdinW.Close()
		session.Close()
		client.Close()
		return nil, err
	}

	session.Stdin = stdinR
	session.Stdout = stdoutW
	session.Stderr = stdoutW

	if err := session.Shell(); err != nil {
		stdinR.Close()
		stdinW.Close()
		stdoutR.Close()
		stdoutW.Close()
		session.Close()
		client.Close()
		return nil, fmt.Errorf("start shell: %w", err)
	}

	return &sshSession{
		client:  client,
		session: session,
		stdinW:  stdinW,
		stdoutR: stdoutR,
	}, nil
}

// sendWindowChange sends a window-change request to the remote side.
func sendWindowChange(session *ssh.Session, width, height int) error {
	req := struct {
		Width, Height, WidthPx, HeightPx uint32
	}{uint32(width), uint32(height), 0, 0}
	_, err := session.SendRequest("window-change", true, ssh.Marshal(&req))
	return err
}

// closeSSH cleans up an SSH session.
func closeSSH(s *sshSession) {
	if s == nil {
		return
	}
	s.stdinW.Close()
	s.stdoutR.Close()
	s.session.Close()
	s.client.Close()
}

func connectLogsStream(node *domain.Node, command string) (*logsStreamSession, error) {
	config, err := buildSSHConfig(node)
	if err != nil {
		return nil, fmt.Errorf("ssh config: %w", err)
	}

	host, port := parseHostPort(node.IP)
	addr := host + ":22"
	if port != "" {
		addr = host + ":" + port
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("ssh session: %w", err)
	}

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}

	session.Stdout = stdoutW
	session.Stderr = stdoutW

	if err := session.Start(command); err != nil {
		stdoutR.Close()
		stdoutW.Close()
		session.Close()
		client.Close()
		return nil, fmt.Errorf("start logs command: %w", err)
	}

	return &logsStreamSession{
		client:  client,
		session: session,
		stdoutR: stdoutR,
	}, nil
}

func closeLogsStream(s *logsStreamSession) {
	if s == nil {
		return
	}
	s.stdoutR.Close()
	s.session.Close()
	s.client.Close()
}

// buildSSHConfig builds an ssh.ClientConfig from node fields plus TOFU host
// key verification.
func buildSSHConfig(node *domain.Node) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	keyPath := node.SSHKeyPath
	if keyPath != "" {
		if strings.HasPrefix(keyPath, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				keyPath = filepath.Join(home, keyPath[2:])
			}
		}
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("read SSH key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse SSH key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
		if agentConn, err := net.Dial("unix", sshAuthSock); err == nil {
			agentClient := agent.NewClient(agentConn)
			authMethods = append(authMethods, ssh.PublicKeysCallback(agentClient.Signers))
		}
	}

	hostKeyCB, err := tofuHostKeyCallback()
	if err != nil {
		hostKeyCB = ssh.InsecureIgnoreHostKey()
	}

	user := node.User
	if user == "" {
		user = "root"
	}

	return &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCB,
		Timeout:         15 * time.Second,
	}, nil
}

// tofuHostKeyCallback returns a host key callback that auto-accepts unknown
// hosts (Trust on First Use) and rejects key mismatches.
func tofuHostKeyCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")

	sshDir := filepath.Join(home, ".ssh")
	if _, serr := os.Stat(sshDir); os.IsNotExist(serr) {
		if derr := os.MkdirAll(sshDir, 0700); derr != nil {
			return nil, derr
		}
	}

	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		if f, ferr := os.OpenFile(knownHostsPath, os.O_CREATE|os.O_WRONLY, 0600); ferr == nil {
			f.Close()
		}
		callback, err = knownhosts.New(knownHostsPath)
		if err != nil {
			return nil, err
		}
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := callback(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
			f, ferr := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if ferr != nil {
				return ferr
			}
			defer f.Close()

			host := hostname
			if strings.Contains(host, ":") {
				host = "[" + host + "]"
			}
			line := fmt.Sprintf("%s %s\n", host, string(ssh.MarshalAuthorizedKey(key)))
			_, werr := f.WriteString(line)
			return werr
		}

		return err
	}, nil
}
