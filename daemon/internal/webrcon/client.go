// Package webrcon implements Rust's WebSocket-based RCON protocol.
//
// Rust dedicated servers expose an HTTP/WebSocket RCON endpoint when launched
// with +rcon.web 1.  The wire format is a simple JSON envelope:
//
//	Send:    {"Identifier":<id>,"Message":"<command>","Name":"WebRcon"}
//	Receive: {"Identifier":<id>,"Message":"<response>","Type":"Generic","Stacktrace":""}
//
// Reference: https://wiki.facepunch.com/rust/Creating-a-server#rcon
package webrcon

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

type outbound struct {
	Identifier int    `json:"Identifier"`
	Message    string `json:"Message"`
	Name       string `json:"Name"`
}

type inbound struct {
	Identifier int    `json:"Identifier"`
	Message    string `json:"Message"`
	Type       string `json:"Type"`
	Stacktrace string `json:"Stacktrace"`
}

// Exec connects to addr (host:port) via WebSocket, authenticates with
// password, sends command, reads the response, and closes the connection.
// timeout applies to the entire operation.
func Exec(addr, password, command string, timeout time.Duration) (string, error) {
	url := fmt.Sprintf("ws://%s/%s", addr, password)

	dialer := websocket.Dialer{HandshakeTimeout: timeout}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		// Do not wrap the underlying dialer error — the URL contains the
		// RCON password and gorilla/websocket includes the full URL in its
		// error messages.
		return "", fmt.Errorf("webrcon connect %s: connection failed", addr)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))

	// Send the command.
	msg := outbound{Identifier: 1, Message: command, Name: "WebRcon"}
	payload, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return "", fmt.Errorf("webrcon write: %w", err)
	}

	// Drain messages until we find the one matching our Identifier.
	// Rust may send informational messages before the command response.
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(deadline)
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return "", fmt.Errorf("webrcon read: %w", err)
		}

		var resp inbound
		if err := json.Unmarshal(raw, &resp); err != nil {
			continue // skip non-JSON or unexpected frames
		}
		if resp.Identifier == 1 {
			return resp.Message, nil
		}
	}

	return "", fmt.Errorf("webrcon: timed out waiting for response")
}
