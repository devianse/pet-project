package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// echoServer accepts one WS connection, wires it into a fresh Hub as
// connection.go's Handler will in Task 6 (a minimal stand-in here so
// this test doesn't depend on Task 6 existing yet), and returns the Hub
// so the test can Broadcast into it.
func echoServer(t *testing.T) (*httptest.Server, *Hub) {
	t.Helper()
	hub := NewHub(AllowAuthenticated)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept: %v", err)
			return
		}
		conn := newConnection(newConnID(), 1, ws)
		if err := hub.Register(conn); err != nil {
			t.Errorf("Register: %v", err)
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		go conn.writePump(ctx)
		conn.readPump(ctx, hub, Identity{UserID: 1, Role: "user"})
		hub.Unregister(conn)
	}))
	t.Cleanup(srv.Close)
	return srv, hub
}

// waitForSubscriber polls hub's internal state until topic has at least
// one subscriber, or fails the test after 2s. client.Write(subscribe)
// returning only means the bytes reached the local TCP buffer — it says
// nothing about whether the server's readPump has processed the control
// message and called hub.Subscribe yet. Racing hub.Broadcast against that
// is exactly the bug: broadcasting before the subscription registers
// means nothing is delivered, and the test's single blocking client.Read
// then hangs with no way to retry. This closes that race without
// touching connection.go/hub.go or weakening the test's assertions.
func waitForSubscriber(t *testing.T, hub *Hub, topic string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hub.mu.Lock()
		n := len(hub.subscribers[topic])
		hub.mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for subscription to register")
}

func TestConnection_SubscribeThenReceivesBroadcast(t *testing.T) {
	srv, hub := echoServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "ws" + srv.URL[len("http"):]
	client, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close(websocket.StatusNormalClosure, "")

	sub := Envelope{Topic: "ops.health", Type: MessageTypeSubscribe}
	data, _ := json.Marshal(sub)
	if err := client.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("Write subscribe: %v", err)
	}
	waitForSubscriber(t, hub, "ops.health")

	// Give the server's readPump a moment to process the subscribe
	// before broadcasting — this test only needs "eventually", not a
	// specific latency bound.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hub.Broadcast(Envelope{Topic: "ops.health", Type: MessageTypeUpdate, Payload: json.RawMessage(`{"n":1}`)})

		_, msg, err := client.Read(ctx)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		var got Envelope
		if err := json.Unmarshal(msg, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if got.Topic == "ops.health" && got.Type == MessageTypeUpdate {
			return // success
		}
	}
	t.Fatal("never received the broadcast update within the deadline")
}
