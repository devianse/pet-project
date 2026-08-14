// backend/internal/telegram/router_test.go
package telegram

import (
	"context"
	"errors"
	"testing"
)

func TestRouter_Dispatch_MatchesRegisteredPrefix(t *testing.T) {
	r := NewRouter()
	var gotArgs string
	r.Handle("/newnote ", func(_ context.Context, args string) (string, error) {
		gotArgs = args
		return "note added", nil
	})

	reply := r.Dispatch(context.Background(), "/newnote buy milk")

	if reply != "note added" {
		t.Fatalf("expected %q, got %q", "note added", reply)
	}
	if gotArgs != "buy milk" {
		t.Fatalf("expected args %q, got %q", "buy milk", gotArgs)
	}
}

func TestRouter_Dispatch_UnmatchedTextGetsUnknownCommandReply(t *testing.T) {
	r := NewRouter()
	r.Handle("/notes", func(_ context.Context, _ string) (string, error) {
		return "should not be called", nil
	})

	reply := r.Dispatch(context.Background(), "/banana")

	if reply != "unknown command" {
		t.Fatalf("expected %q, got %q", "unknown command", reply)
	}
}

func TestRouter_Dispatch_HandlerErrorGetsGenericReply(t *testing.T) {
	r := NewRouter()
	r.Handle("/notes", func(_ context.Context, _ string) (string, error) {
		return "", errors.New("db exploded")
	})

	reply := r.Dispatch(context.Background(), "/notes")

	if reply != "error handling command" {
		t.Fatalf("expected generic error reply, got %q", reply)
	}
}

func TestRouter_Dispatch_FirstRegisteredMatchWins(t *testing.T) {
	r := NewRouter()
	r.Handle("/notes", func(_ context.Context, _ string) (string, error) { return "first", nil })
	r.Handle("/notes", func(_ context.Context, _ string) (string, error) { return "second", nil })

	if reply := r.Dispatch(context.Background(), "/notes"); reply != "first" {
		t.Fatalf("expected first-registered handler to win, got %q", reply)
	}
}
