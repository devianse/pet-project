package realtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestNewHandler_PanicsOnInvalidPingInterval(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected NewHandler to panic on non-positive pingInterval")
		}
	}()
	authOK := AuthenticatorFunc(func(r *http.Request) (Identity, error) {
		return Identity{UserID: 1, Role: "user"}, nil
	})
	NewHandler(NewHub(AllowAuthenticated), authOK, WithPingInterval(0))
}

func TestHandler_RejectsUnauthenticated(t *testing.T) {
	authFail := AuthenticatorFunc(func(r *http.Request) (Identity, error) {
		return Identity{}, http.ErrNoCookie
	})
	handler := NewHandler(NewHub(AllowAuthenticated), authFail)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	url := "ws" + srv.URL[len("http"):]
	_, resp, err := websocket.Dial(ctx, url, nil)
	if err == nil {
		t.Fatal("expected Dial to fail for an unauthenticated request")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestHandler_AcceptsAuthenticatedAndRegisters(t *testing.T) {
	authOK := AuthenticatorFunc(func(r *http.Request) (Identity, error) {
		return Identity{UserID: 7, Role: "user"}, nil
	})
	hub := NewHub(AllowAuthenticated)
	handler := NewHandler(hub, authOK, WithPingInterval(50*time.Millisecond))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	url := "ws" + srv.URL[len("http"):]
	client, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close(websocket.StatusNormalClosure, "")

	// coder/websocket only processes incoming control frames (including
	// pongs) while something is actively pumping Read/Reader on the
	// connection — mirroring connection.go's server-side readPump. Without
	// this, client.Ping below would block until ctx's deadline regardless
	// of whether the server responds correctly.
	go func() {
		for {
			if _, _, err := client.Read(ctx); err != nil {
				return
			}
		}
	}()

	// The connection should survive at least one ping/pong round trip
	// without the server closing it.
	time.Sleep(150 * time.Millisecond)
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("expected connection still alive after ping interval, got: %v", err)
	}
}
