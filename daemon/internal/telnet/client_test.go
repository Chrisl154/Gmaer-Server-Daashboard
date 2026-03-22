package telnet

import (
	"bufio"
	"fmt"
	"net"
	"testing"
	"time"
)

// mockNetError implements net.Error for testing isTimeout.
type mockNetError struct{ timeout bool }

func (e *mockNetError) Error() string    { return "mock net error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return false }

func TestStripTelnetBytes_PlainText(t *testing.T) {
	got := stripTelnetBytes("hello world\r\n")
	if got != "hello world\r\n" {
		t.Errorf("got %q, want plain text unchanged", got)
	}
}

func TestStripTelnetBytes_Empty(t *testing.T) {
	if got := stripTelnetBytes(""); got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestStripTelnetBytes_ThreeByteSequence(t *testing.T) {
	// IAC WILL ECHO = 0xFF 0xFB 0x01, followed by "hi"
	input := string([]byte{0xFF, 0xFB, 0x01}) + "hi"
	got := stripTelnetBytes(input)
	if got != "hi" {
		t.Errorf("got %q, want hi", got)
	}
}

func TestStripTelnetBytes_TwoByteSequence(t *testing.T) {
	// IAC + non-verb byte (0xF0), followed by "ok"
	input := string([]byte{0xFF, 0xF0}) + "ok"
	got := stripTelnetBytes(input)
	if got != "ok" {
		t.Errorf("got %q, want ok", got)
	}
}

func TestStripTelnetBytes_Mixed(t *testing.T) {
	// "he" + IAC WILL ECHO (3-byte: 0xFF 0xFB 0x01) + "llo"
	input := "he" + string([]byte{0xFF, 0xFB, 0x01}) + "llo"
	got := stripTelnetBytes(input)
	if got != "hello" {
		t.Errorf("got %q, want hello", got)
	}
}

func TestStripTelnetBytes_TrailingIAC(t *testing.T) {
	// Trailing 0xFF with no following byte
	input := "text" + string([]byte{0xFF})
	got := stripTelnetBytes(input)
	if got != "text" {
		t.Errorf("got %q, want text", got)
	}
}

func TestIsTimeout_Nil(t *testing.T) {
	if isTimeout(nil) {
		t.Error("isTimeout(nil) should be false")
	}
}

func TestIsTimeout_NetErrorTrue(t *testing.T) {
	if !isTimeout(&mockNetError{timeout: true}) {
		t.Error("expected isTimeout=true for timeout net.Error")
	}
}

func TestIsTimeout_NetErrorFalse(t *testing.T) {
	if isTimeout(&mockNetError{timeout: false}) {
		t.Error("expected isTimeout=false for non-timeout net.Error")
	}
}

func TestIsTimeout_PlainError(t *testing.T) {
	if isTimeout(fmt.Errorf("plain error")) {
		t.Error("expected isTimeout=false for non-net.Error")
	}
}

func TestExec_ConnectionRefused(t *testing.T) {
	_, err := Exec("127.0.0.1:1", "pass", "ping", time.Second)
	if err == nil {
		t.Error("expected connection error on port 1")
	}
}

func TestExec_MockServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)

		// Send password prompt
		fmt.Fprint(conn, "Please enter password\r\n")
		// Read the password line (discard)
		r.ReadString('\n') //nolint:errcheck
		// Send auth confirmation
		fmt.Fprint(conn, "Logon successful\r\n")
		// Read the command line (discard)
		r.ReadString('\n') //nolint:errcheck
		// Send response — closing the conn breaks the drain loop via EOF
		fmt.Fprint(conn, "pong\r\n")
	}()

	got, err := Exec(ln.Addr().String(), "testpass", "ping", 5*time.Second)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if got != "pong" {
		t.Errorf("got %q, want pong", got)
	}
}
