package rcon

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

// buildPacket serialises an RCON packet into bytes for test assertions.
func buildPacket(id, typ int32, body string) []byte {
	bodyBytes := []byte(body)
	size := int32(4 + 4 + len(bodyBytes) + 2)
	buf := make([]byte, 4+size)
	binary.LittleEndian.PutUint32(buf[0:], uint32(size))
	binary.LittleEndian.PutUint32(buf[4:], uint32(id))
	binary.LittleEndian.PutUint32(buf[8:], uint32(typ))
	copy(buf[12:], bodyBytes)
	return buf
}

func TestWritePacket_ReadPacket_Roundtrip(t *testing.T) {
	var buf bytes.Buffer
	if err := writePacket(&buf, 42, typeCommand, "say hello"); err != nil {
		t.Fatalf("writePacket error: %v", err)
	}

	p, err := readPacket(&buf)
	if err != nil {
		t.Fatalf("readPacket error: %v", err)
	}
	if p.id != 42 {
		t.Errorf("id = %d, want 42", p.id)
	}
	if p.typ != typeCommand {
		t.Errorf("typ = %d, want %d", p.typ, typeCommand)
	}
	if p.body != "say hello" {
		t.Errorf("body = %q, want %q", p.body, "say hello")
	}
}

func TestWritePacket_EmptyBody(t *testing.T) {
	var buf bytes.Buffer
	if err := writePacket(&buf, 1, typeAuth, ""); err != nil {
		t.Fatalf("writePacket error: %v", err)
	}
	p, err := readPacket(&buf)
	if err != nil {
		t.Fatalf("readPacket error: %v", err)
	}
	if p.body != "" {
		t.Errorf("body = %q, want empty", p.body)
	}
}

func TestReadPacket_InvalidSize_TooSmall(t *testing.T) {
	// Write a packet with size < 10
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, int32(5)) // invalid size
	_, err := readPacket(&buf)
	if err == nil {
		t.Error("expected error for invalid packet size, got nil")
	}
}

func TestReadAuthResponse_AuthFailed(t *testing.T) {
	// Build a packet with id == -1 (auth rejected)
	pkt := buildPacket(-1, typeAuthResp, "")
	r := bytes.NewReader(pkt)
	err := readAuthResponse(r)
	if err == nil {
		t.Error("expected auth failure error, got nil")
	}
}

func TestReadAuthResponse_Success(t *testing.T) {
	// Build a packet with id == 1 (auth success)
	pkt := buildPacket(1, typeAuthResp, "")
	r := bytes.NewReader(pkt)
	if err := readAuthResponse(r); err != nil {
		t.Errorf("expected nil error for successful auth, got: %v", err)
	}
}

func TestReadAuthResponse_SkipsEmptyResponse(t *testing.T) {
	// First packet has id 0 (empty RESPONSE_VALUE before auth response), second has id 1 (auth ok)
	var buf bytes.Buffer
	buf.Write(buildPacket(0, typeResponse, ""))
	buf.Write(buildPacket(1, typeAuthResp, ""))
	if err := readAuthResponse(&buf); err != nil {
		t.Errorf("expected nil error after skipping empty response, got: %v", err)
	}
}

func TestExec_ConnectionRefused(t *testing.T) {
	_, err := Exec("127.0.0.1:19999", "password", "status", 500*time.Millisecond)
	if err == nil {
		t.Error("expected error when connecting to closed port, got nil")
	}
}

// TestExec_MockServer spins up a minimal RCON-like TCP server and verifies
// that Exec authenticates and returns the command response.
func TestExec_MockServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read auth packet
		if _, err := readPacket(conn); err != nil {
			return
		}
		// Send empty RESPONSE_VALUE then AUTH_RESPONSE
		writePacket(conn, 0, typeResponse, "")
		writePacket(conn, 1, typeAuthResp, "")

		// Read command packet
		if _, err := readPacket(conn); err != nil {
			return
		}
		// Send response
		writePacket(conn, 2, typeResponse, "pong")
	}()

	resp, err := Exec(ln.Addr().String(), "pass", "ping", 3*time.Second)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if resp != "pong" {
		t.Errorf("response = %q, want pong", resp)
	}
}

// TestReadResponseBody reads a single response packet.
func TestReadResponseBody(t *testing.T) {
	pkt := buildPacket(2, typeResponse, "server status ok")
	r := bytes.NewReader(pkt)
	body, err := readResponseBody(r)
	if err != nil {
		t.Fatalf("readResponseBody error: %v", err)
	}
	if body != "server status ok" {
		t.Errorf("body = %q, want %q", body, "server status ok")
	}
}

func TestReadPacket_EOF(t *testing.T) {
	r := bytes.NewReader([]byte{})
	_, err := readPacket(r)
	if err == nil {
		t.Error("expected error reading from empty reader")
	}
}

// writePacket helper so test server can respond — reuse production function via same package.
func init() {
	_ = io.Discard // avoid unused import
}
