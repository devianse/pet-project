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
	r.Handle("/newnote ", "add a note", func(_ context.Context, args string) (string, error) {
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
	r.Handle("/notes", "list notes", func(_ context.Context, _ string) (string, error) {
		return "should not be called", nil
	})

	reply := r.Dispatch(context.Background(), "/banana")

	if reply != unknownCommandReply {
		t.Fatalf("expected %q, got %q", unknownCommandReply, reply)
	}
}

func TestRouter_Dispatch_HandlerErrorGetsGenericReply(t *testing.T) {
	r := NewRouter()
	r.Handle("/notes", "list notes", func(_ context.Context, _ string) (string, error) {
		return "", errors.New("db exploded")
	})

	reply := r.Dispatch(context.Background(), "/notes")

	if reply != "error handling command" {
		t.Fatalf("expected generic error reply, got %q", reply)
	}
}

func TestRouter_HelpText_ListsCommandsInRegistrationOrder(t *testing.T) {
	r := NewRouter()
	r.Handle("/notes", "list notes", func(_ context.Context, _ string) (string, error) { return "", nil })
	r.Handle("/newnote ", "add a note", func(_ context.Context, _ string) (string, error) { return "", nil })

	want := "/notes — list notes\n/newnote  — add a note"
	if got := r.HelpText(); got != want {
		t.Fatalf("expected help text %q, got %q", want, got)
	}
}

func TestRouter_Commands_ReturnsPrefixAndDescription(t *testing.T) {
	r := NewRouter()
	r.Handle("/notes", "list notes", func(_ context.Context, _ string) (string, error) { return "", nil })

	got := r.Commands()
	want := []Command{{Prefix: "/notes", Description: "list notes"}}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestRouter_Dispatch_FirstRegisteredMatchWins(t *testing.T) {
	r := NewRouter()
	r.Handle("/notes", "list notes", func(_ context.Context, _ string) (string, error) { return "first", nil })
	r.Handle("/notes", "list notes again", func(_ context.Context, _ string) (string, error) { return "second", nil })

	if reply := r.Dispatch(context.Background(), "/notes"); reply != "first" {
		t.Fatalf("expected first-registered handler to win, got %q", reply)
	}
}
