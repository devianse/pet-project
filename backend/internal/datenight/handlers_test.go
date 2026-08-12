package datenight

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
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
