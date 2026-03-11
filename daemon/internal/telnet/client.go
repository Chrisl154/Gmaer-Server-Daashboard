// Package telnet implements a minimal Telnet client used for game server
// consoles that expose a Telnet interface (e.g. 7 Days to Die).
package telnet

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

// Exec connects to a Telnet server, authenticates with password, sends
// command, reads the response, and closes the connection.
// timeout applies to the entire operation.
func Exec(addr, password, command string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return "", fmt.Errorf("telnet connect %s: %w", addr, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)

	r := bufio.NewReader(conn)

	// Consume Telnet negotiation bytes and read until the password prompt.
	if err := waitFor(r, conn, []string{"Please enter password", "password:", "Password:"}, 5*time.Second); err != nil {
		return "", fmt.Errorf("waiting for password prompt: %w", err)
	}

	// Send password.
	if _, err := fmt.Fprintf(conn, "%s\r\n", password); err != nil {
		return "", fmt.Errorf("send password: %w", err)
	}

	// Wait for authentication confirmation.
	if err := waitFor(r, conn, []string{"Logon successful", "session started", "Connected"}, 5*time.Second); err != nil {
		return "", fmt.Errorf("authentication failed or timed out: %w", err)
	}

	// Send the command.
	if _, err := fmt.Fprintf(conn, "%s\r\n", command); err != nil {
		return "", fmt.Errorf("send command: %w", err)
	}

	// Drain output for up to 2 seconds.
	var sb strings.Builder
	readDeadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(readDeadline) {
		_ = conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		line, err := r.ReadString('\n')
		if line != "" {
			sb.WriteString(stripTelnetBytes(line))
		}
		if err != nil {
			break // timeout or EOF — done
		}
	}

	return strings.TrimSpace(sb.String()), nil
}

// waitFor reads lines until one contains any of the triggers, or until the
// local deadline fires.  It returns an error if the deadline expires first.
func waitFor(r *bufio.Reader, conn net.Conn, triggers []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		line, err := r.ReadString('\n')
		line = stripTelnetBytes(line)
		for _, t := range triggers {
			if strings.Contains(line, t) {
				return nil
			}
		}
		if err != nil && !isTimeout(err) {
			return err
		}
	}
	return fmt.Errorf("timed out waiting for %v", triggers)
}

// stripTelnetBytes removes IAC (0xFF) control sequences from a line.
func stripTelnetBytes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0xFF { // IAC — skip 2-byte or 3-byte sequences
			if i+1 < len(s) {
				next := s[i+1]
				if next == 0xFB || next == 0xFC || next == 0xFD || next == 0xFE {
					i += 2 // 3-byte sequence: IAC <verb> <opt>
				} else {
					i++ // 2-byte sequence: IAC <cmd>
				}
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// isTimeout returns true if err is a network timeout.
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}
