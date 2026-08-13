package datenight

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/devianse/pet-project/backend/internal/auth"
)

func TestHandler_CreateActivity_RejectsEmptyName(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)

	body, _ := json.Marshal(createActivityRequest{Name: "  ", Category: CategoryFood})
	req := httptest.NewRequest(http.MethodPost, "/api/datenight/activities", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.CreateActivity(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandler_CreateActivity_RejectsInvalidCategory(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)

	body, _ := json.Marshal(createActivityRequest{Name: "Movie Night", Category: Category("not-a-real-category")})
	req := httptest.NewRequest(http.MethodPost, "/api/datenight/activities", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.CreateActivity(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandler_CreateActivityThenListActivities(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)

	body, _ := json.Marshal(createActivityRequest{Name: "Movie Night", Category: CategoryCozy})
	req := httptest.NewRequest(http.MethodPost, "/api/datenight/activities", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.CreateActivity(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/datenight/activities", nil)
	listRec := httptest.NewRecorder()
	handler.ListActivities(listRec, listReq)

	var listed []Activity
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decoding list response: %v", err)
	}
	if len(listed) != 1 || listed[0].Name != "Movie Night" {
		t.Fatalf("expected 1 activity named Movie Night, got %+v", listed)
	}
}

func TestHandler_DeleteActivity(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)
	ctx := context.Background()

	created, err := store.CreateActivity(ctx, "To delete", nil, CategoryAdventure)
	if err != nil {
		t.Fatalf("CreateActivity: %v", err)
	}
	idStr := strconv.FormatInt(created.ID, 10)

	req := httptest.NewRequest(http.MethodDelete, "/api/datenight/activities/"+idStr, nil)
	req.SetPathValue("id", idStr)
	rec := httptest.NewRecorder()

	handler.DeleteActivity(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestHandler_DeleteActivity_NotFound(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/datenight/activities/999999", nil)
	req.SetPathValue("id", "999999")
	rec := httptest.NewRecorder()

	handler.DeleteActivity(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

var testSecret = []byte("test-secret")

// requestAs builds a request carrying a real, validly-signed session
// cookie for (userID, username), then runs it through auth.Require so the
// handler under test sees the same populated context production code
// would. userID is explicit because accept/decline turn on WHO the caller
// is: the store rejects a response from the proposal's own author.
func requestAs(t *testing.T, userID int64, username, method, target string, body []byte) *http.Request {
	t.Helper()
	token, err := auth.SignToken(testSecret, userID, username, "user")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	return req
}

func serveWithAuth(handlerFunc http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	auth.Require(testSecret)(handlerFunc).ServeHTTP(rec, req)
	return rec
}

func TestHandler_CreateProposalThenListProposals(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)
	activity := mustCreateActivity(t, store)

	body, _ := json.Marshal(map[string]any{
		"activity_id":  activity.ID,
		"date":         "2026-08-20",
		"time_slot":    "evening",
		"energy_level": "casual",
		"moods":        []string{"romantic", "playful"},
	})
	req := requestAs(t, 1, "alice", http.MethodPost, "/api/datenight/proposals", body)
	rec := serveWithAuth(handler.CreateProposal, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var created Proposal
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if created.Status != StatusPending {
		t.Fatalf("expected pending, got %q", created.Status)
	}

	listReq := requestAs(t, 1, "alice", http.MethodGet, "/api/datenight/proposals", nil)
	listRec := serveWithAuth(handler.ListProposals, listReq)

	var resp proposalsResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding list response: %v", err)
	}
	if resp.Current == nil || resp.Current.ID != created.ID {
		t.Fatalf("expected current to be the created proposal, got %+v", resp.Current)
	}
	if len(resp.History) != 0 {
		t.Fatalf("expected empty history for a single proposal, got %+v", resp.History)
	}
}

func TestHandler_CreateProposal_RejectsBadDate(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)
	activity := mustCreateActivity(t, store)

	body, _ := json.Marshal(map[string]any{
		"activity_id": activity.ID, "date": "not-a-date", "time_slot": "evening",
		"energy_level": "casual", "moods": []string{"chill"},
	})
	req := requestAs(t, 1, "alice", http.MethodPost, "/api/datenight/proposals", body)
	rec := serveWithAuth(handler.CreateProposal, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandler_CreateProposal_RejectsNoMoods(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)
	activity := mustCreateActivity(t, store)

	body, _ := json.Marshal(map[string]any{
		"activity_id": activity.ID, "date": "2026-08-20", "time_slot": "evening",
		"energy_level": "casual", "moods": []string{},
	})
	req := requestAs(t, 1, "alice", http.MethodPost, "/api/datenight/proposals", body)
	rec := serveWithAuth(handler.CreateProposal, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandler_CreateProposal_RejectsUnknownActivity(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)

	body, _ := json.Marshal(map[string]any{
		"activity_id": 999999, "date": "2026-08-20", "time_slot": "evening",
		"energy_level": "casual", "moods": []string{"chill"},
	})
	req := requestAs(t, 1, "alice", http.MethodPost, "/api/datenight/proposals", body)
	rec := serveWithAuth(handler.CreateProposal, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_AcceptProposal(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)
	activity := mustCreateActivity(t, store)
	created, err := store.CreateProposal(context.Background(), activity.ID, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), TimeSlotEvening, EnergyCasual, []Mood{MoodChill}, 1)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	idStr := strconv.FormatInt(created.ID, 10)

	req := requestAs(t, 2, "bob", http.MethodPost, "/api/datenight/proposals/"+idStr+"/accept", nil)
	req.SetPathValue("id", idStr)
	rec := serveWithAuth(handler.AcceptProposal, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var updated Proposal
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if updated.Status != StatusAccepted {
		t.Fatalf("expected accepted, got %q", updated.Status)
	}
}

// The proposer can't answer their own invitation — same 409 as any other
// non-actionable target, since the store decides all of those together.
func TestHandler_AcceptProposal_RejectsSelfResponse(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)
	activity := mustCreateActivity(t, store)
	created, err := store.CreateProposal(context.Background(), activity.ID, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), TimeSlotEvening, EnergyCasual, []Mood{MoodChill}, 1)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	idStr := strconv.FormatInt(created.ID, 10)

	req := requestAs(t, 1, "alice", http.MethodPost, "/api/datenight/proposals/"+idStr+"/accept", nil)
	req.SetPathValue("id", idStr)
	rec := serveWithAuth(handler.AcceptProposal, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_DeclineProposal_Conflict(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)

	req := requestAs(t, 2, "bob", http.MethodPost, "/api/datenight/proposals/999999/decline", nil)
	req.SetPathValue("id", "999999")
	rec := serveWithAuth(handler.DeclineProposal, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestHandler_CreateProposal_IncludesProposedByUsername(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)
	activity := mustCreateActivity(t, store)
	mustCreateUser(t, store, "alice")
	aliceID := int64(1) // placeholder, overwritten below
	// requestAs signs a token for whatever (userID, username) is passed —
	// it doesn't require a matching `users` row — but the join in Task 1
	// reads the real `users` table, so the row must exist and its id must
	// match what's signed into the token.
	row := store.conn.QueryRowContext(context.Background(), `SELECT id FROM users WHERE username = 'alice'`)
	if err := row.Scan(&aliceID); err != nil {
		t.Fatalf("looking up alice's id: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"activity_id":  activity.ID,
		"date":         "2026-08-20",
		"time_slot":    "evening",
		"energy_level": "casual",
		"moods":        []string{"romantic", "playful"},
	})
	req := requestAs(t, aliceID, "alice", http.MethodPost, "/api/datenight/proposals", body)
	rec := serveWithAuth(handler.CreateProposal, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var created Proposal
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if created.ProposedByUsername != "alice" {
		t.Fatalf("expected proposed_by_username %q in JSON response, got %q", "alice", created.ProposedByUsername)
	}
}
