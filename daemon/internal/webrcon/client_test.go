package webrcon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// startMockServer spins up an httptest.Server that upgrades to WebSocket
// and hands the connection to handler.
func startMockServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handler(conn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// serverAddr strips the "http://" scheme so it can be used as the addr
// argument to Exec (which prepends "ws://").
func serverAddr(srv *httptest.Server) string {
	return strings.TrimPrefix(srv.URL, "http://")
}

func TestExec_ConnectionRefused(t *testing.T) {
	_, err := Exec("127.0.0.1:1", "pass", "ping", time.Second)
	if err == nil {
		t.Error("expected connection error on port 1")
	}
}

func TestExec_ReceivesResponse(t *testing.T) {
	srv := startMockServer(t, func(conn *websocket.Conn) {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg outbound
		json.Unmarshal(raw, &msg) //nolint:errcheck

		resp := inbound{Identifier: msg.Identifier, Message: "pong", Type: "Generic"}
		payload, _ := json.Marshal(resp)
		conn.WriteMessage(websocket.TextMessage, payload) //nolint:errcheck
	})

	got, err := Exec(serverAddr(srv), "testpass", "ping", 5*time.Second)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if got != "pong" {
		t.Errorf("got %q, want pong", got)
	}
}

func TestExec_SkipsNonMatchingMessages(t *testing.T) {
	srv := startMockServer(t, func(conn *websocket.Conn) {
		_, raw, _ := conn.ReadMessage()
		var msg outbound
		json.Unmarshal(raw, &msg) //nolint:errcheck

		// Send an informational message with a different Identifier first
		info := inbound{Identifier: 0, Message: "server info", Type: "Chat"}
		infoBytes, _ := json.Marshal(info)
		conn.WriteMessage(websocket.TextMessage, infoBytes) //nolint:errcheck

		// Then send the matching response
		resp := inbound{Identifier: msg.Identifier, Message: "command result", Type: "Generic"}
		payload, _ := json.Marshal(resp)
		conn.WriteMessage(websocket.TextMessage, payload) //nolint:errcheck
	})

	got, err := Exec(serverAddr(srv), "pass", "status", 5*time.Second)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if got != "command result" {
		t.Errorf("got %q, want command result", got)
	}
}

func TestExec_URLIncludesPassword(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, raw, _ := conn.ReadMessage()
		var msg outbound
		json.Unmarshal(raw, &msg) //nolint:errcheck
		resp := inbound{Identifier: msg.Identifier, Message: "ok"}
		payload, _ := json.Marshal(resp)
		conn.WriteMessage(websocket.TextMessage, payload) //nolint:errcheck
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	Exec(addr, "mypassword", "cmd", 5*time.Second) //nolint:errcheck
	if capturedPath != "/mypassword" {
		t.Errorf("path = %q, want /mypassword", capturedPath)
	}
}
