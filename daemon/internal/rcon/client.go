// Package rcon implements the Source RCON protocol for sending commands to
// game servers (Minecraft, Palworld, etc.) over a plain TCP connection.
//
// Protocol reference: https://developer.valvesoftware.com/wiki/Source_RCON_Protocol
package rcon

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	typeAuth     int32 = 3 // SERVERDATA_AUTH
	typeCommand  int32 = 2 // SERVERDATA_EXECCOMMAND
	typeResponse int32 = 0 // SERVERDATA_RESPONSE_VALUE
	typeAuthResp int32 = 2 // SERVERDATA_AUTH_RESPONSE (same wire value as command)

	maxResponseBody = 4096 // bytes; protect against runaway reads
)

// Exec opens a TCP connection to addr, authenticates with password, sends
// command, reads the response, and closes the connection.  timeout applies to
// the entire operation (dial + auth + exec).
func Exec(addr, password, command string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return "", fmt.Errorf("connect %s: %w", addr, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)

	// Authenticate.
	if err := writePacket(conn, 1, typeAuth, password); err != nil {
		return "", fmt.Errorf("write auth: %w", err)
	}
	// The server sends an empty RESPONSE_VALUE first, then the AUTH_RESPONSE.
	// We read until we get a packet whose ID matches ours (1) or ID==-1 (fail).
	if err := readAuthResponse(conn); err != nil {
		return "", err
	}

	// Send command.
	if err := writePacket(conn, 2, typeCommand, command); err != nil {
		return "", fmt.Errorf("write command: %w", err)
	}
	body, err := readResponseBody(conn)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	return body, nil
}

// packet represents a decoded RCON packet.
type packet struct {
	id   int32
	typ  int32
	body string
}

// writePacket serialises and sends a single RCON packet.
func writePacket(w io.Writer, id, typ int32, body string) error {
	bodyBytes := []byte(body)
	// size = id(4) + type(4) + body + null + padding null
	size := int32(4 + 4 + len(bodyBytes) + 2)

	buf := make([]byte, 4+size)
	binary.LittleEndian.PutUint32(buf[0:], uint32(size))
	binary.LittleEndian.PutUint32(buf[4:], uint32(id))
	binary.LittleEndian.PutUint32(buf[8:], uint32(typ))
	copy(buf[12:], bodyBytes)
	// buf[12+len(bodyBytes)] and buf[12+len(bodyBytes)+1] are already 0 (null terminators)

	_, err := w.Write(buf)
	return err
}

// readPacket reads and deserialises a single RCON packet.
func readPacket(r io.Reader) (packet, error) {
	var sizeBuf [4]byte
	if _, err := io.ReadFull(r, sizeBuf[:]); err != nil {
		return packet{}, fmt.Errorf("read size: %w", err)
	}
	size := int32(binary.LittleEndian.Uint32(sizeBuf[:]))
	if size < 10 || size > maxResponseBody+10 {
		return packet{}, fmt.Errorf("invalid packet size %d", size)
	}

	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return packet{}, fmt.Errorf("read body: %w", err)
	}

	p := packet{
		id:  int32(binary.LittleEndian.Uint32(data[0:4])),
		typ: int32(binary.LittleEndian.Uint32(data[4:8])),
	}
	// Body is null-terminated; trim both trailing nulls.
	bodyEnd := len(data) - 2
	if bodyEnd > 8 {
		p.body = string(data[8:bodyEnd])
	}
	return p, nil
}

// readAuthResponse reads packets until the auth response arrives.
// Returns an error if the server rejects the password (ID == -1).
func readAuthResponse(r io.Reader) error {
	for {
		p, err := readPacket(r)
		if err != nil {
			return fmt.Errorf("auth response: %w", err)
		}
		if p.id == -1 {
			return fmt.Errorf("RCON authentication failed: wrong password")
		}
		if p.id == 1 {
			return nil // authenticated
		}
		// Otherwise it's the empty RESPONSE_VALUE before the auth response; keep reading.
	}
}

// readResponseBody reads a command response packet and returns its body.
func readResponseBody(r io.Reader) (string, error) {
	p, err := readPacket(r)
	if err != nil {
		return "", err
	}
	return p.body, nil
}
