// backend/internal/telegram/client_test.go
package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestRealClient_Post_TransportErrorDoesNotLeakToken(t *testing.T) {
	const secretToken = "super-secret-token"

	// Start and immediately close a server so the request fails at the
	// transport level (connection refused) rather than getting an HTTP
	// response — this is what triggers httpClient.Do's err path, as
	// opposed to a non-2xx or malformed-body response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	client := newRealClient(server.URL, secretToken)
	_, err := client.GetUpdates(context.Background(), 0, 30)
	if err == nil {
		t.Fatal("expected error when server is unreachable, got nil")
	}
	if strings.Contains(err.Error(), secretToken) {
		t.Fatalf("error message leaked the bot token: %q", err.Error())
	}
}

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

// TestRealClient_GetUpdates_SkipsMalformedElementButKeepsValidOnes covers
// finding 4: one malformed element (a type mismatch on "message") must not
// fail the whole batch. The valid element must still come back, and the
// malformed element's update_id must still be returned (Message nil) so
// Poller's offset-advancement logic doesn't refetch it forever.
func TestRealClient_GetUpdates_SkipsMalformedElementButKeepsValidOnes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// update_id 201's "message" is a string instead of an object —
		// a type mismatch that fails to unmarshal into Message. 202 is
		// well-formed.
		w.Write([]byte(`{"ok":true,"result":[
			{"update_id":201,"message":"not-an-object"},
			{"update_id":202,"message":{"chat":{"id":555},"text":"/notes"}}
		]}`))
	}))
	defer server.Close()

	client := newRealClient(server.URL, "test-token")
	updates, err := client.GetUpdates(context.Background(), 201, 30)
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}

	if len(updates) != 2 {
		t.Fatalf("expected 2 updates (malformed one included with nil Message so offset can advance), got %d: %+v", len(updates), updates)
	}
	if updates[0].UpdateID != 201 || updates[0].Message != nil {
		t.Fatalf("expected malformed update_id 201 with nil Message, got %+v", updates[0])
	}
	if updates[1].UpdateID != 202 || updates[1].Message == nil || updates[1].Message.Text != "/notes" {
		t.Fatalf("expected valid update_id 202 to still decode, got %+v", updates[1])
	}

}

// TestPoller_Run_AdvancesOffsetPastMalformedElement is an end-to-end check
// (real RealClient + httptest server + real Poller) that a malformed
// element doesn't wedge the poller: it must not be refetched on the next
// poll. The server records every offset it's called with; a second call
// reusing offset 201 (the malformed update's id) would mean the poller got
// stuck re-requesting the same batch forever.
func TestPoller_Run_AdvancesOffsetPastMalformedElement(t *testing.T) {
	var gotOffsets []float64
	var mu sync.Mutex
	ctx, cancel := context.WithCancel(context.Background())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		gotOffsets = append(gotOffsets, body["offset"].(float64))
		call := len(gotOffsets)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			w.Write([]byte(`{"ok":true,"result":[
				{"update_id":201,"message":"not-an-object"},
				{"update_id":202,"message":{"chat":{"id":555},"text":"/notes"}}
			]}`))
			return
		}
		// Second (and any further) poll: no new updates. Cancel so Run
		// stops instead of long-polling again.
		cancel()
		w.Write([]byte(`{"ok":true,"result":[]}`))
	}))
	defer server.Close()

	client := newRealClient(server.URL, "test-token")
	router := NewRouter()
	router.Handle("/notes", func(_ context.Context, _ string) (string, error) { return "", nil })
	p := NewPoller(client, 555, router)
	p.Run(ctx)

	mu.Lock()
	defer mu.Unlock()
	if len(gotOffsets) < 2 {
		t.Fatalf("expected at least 2 polls, got %d", len(gotOffsets))
	}
	if gotOffsets[0] != 0 {
		t.Fatalf("expected first poll offset 0, got %v", gotOffsets[0])
	}
	if gotOffsets[1] != 203 {
		t.Fatalf("expected second poll offset 203 (past both update 201 and 202), got %v — malformed update 201 was refetched", gotOffsets[1])
	}
}
