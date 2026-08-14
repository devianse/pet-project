// backend/internal/telegram/client_test.go
package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRealClient_SendMessage_PostsToCorrectPath(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	client := newRealClient(server.URL, "test-token")
	if err := client.SendMessage(context.Background(), 12345, "hello"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if !strings.HasSuffix(gotPath, "/bottest-token/sendMessage") {
		t.Fatalf("expected path ending in /bottest-token/sendMessage, got %q", gotPath)
	}
	if gotBody["chat_id"] != float64(12345) {
		t.Fatalf("expected chat_id 12345, got %v", gotBody["chat_id"])
	}
	if gotBody["text"] != "hello" {
		t.Fatalf("expected text %q, got %v", "hello", gotBody["text"])
	}
}

func TestRealClient_SendMessage_ReturnsErrorOnAPIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":false,"description":"chat not found"}`))
	}))
	defer server.Close()

	client := newRealClient(server.URL, "test-token")
	if err := client.SendMessage(context.Background(), 12345, "hello"); err == nil {
		t.Fatal("expected error when telegram API returns ok:false, got nil")
	}
}

func TestRealClient_GetUpdates_ParsesResultAndPassesOffset(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"result":[
			{"update_id":101,"message":{"chat":{"id":555},"text":"/notes"}},
			{"update_id":102}
		]}`))
	}))
	defer server.Close()

	client := newRealClient(server.URL, "test-token")
	updates, err := client.GetUpdates(context.Background(), 101, 30)
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}

	if gotBody["offset"] != float64(101) {
		t.Fatalf("expected offset 101 sent, got %v", gotBody["offset"])
	}
	if gotBody["timeout"] != float64(30) {
		t.Fatalf("expected timeout 30 sent, got %v", gotBody["timeout"])
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
	if updates[0].UpdateID != 101 || updates[0].Message == nil || updates[0].Message.Chat.ID != 555 || updates[0].Message.Text != "/notes" {
		t.Fatalf("unexpected first update: %+v", updates[0])
	}
	if updates[1].UpdateID != 102 || updates[1].Message != nil {
		t.Fatalf("expected second update to have a nil Message, got %+v", updates[1])
	}
}
