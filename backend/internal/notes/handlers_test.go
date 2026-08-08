package notes

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestValidateContent(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"valid", "buy milk", false},
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"too long", strings.Repeat("a", maxContentLength+1), true},
		{"exactly at limit", strings.Repeat("a", maxContentLength), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateContent(tc.content)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestHandler_Create_RejectsEmptyItems(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)

	body, _ := json.Marshal(createRequest{Items: []string{}})
	req := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandler_Create_RejectsBatchWithOneInvalidItem(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)

	body, _ := json.Marshal(createRequest{Items: []string{"valid note", "   "}})
	req := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	notes, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("expected nothing inserted on validation failure, got %d notes", len(notes))
	}
}

func TestHandler_CreateThenList(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)

	body, _ := json.Marshal(createRequest{Items: []string{"first note", "second note"}})
	req := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var created []Note
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 notes in response, got %d", len(created))
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	listRec := httptest.NewRecorder()
	handler.List(listRec, listReq)

	var listed []Note
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decoding list response: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 notes from List, got %d", len(listed))
	}
}

func TestHandler_Delete(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)
	ctx := context.Background()

	created, err := store.InsertBatch(ctx, []string{"to delete"})
	if err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}
	id := created[0].ID
	idStr := strconv.FormatInt(id, 10)

	req := httptest.NewRequest(http.MethodDelete, "/api/notes/"+idStr, nil)
	req.SetPathValue("id", idStr)
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestHandler_Delete_NotFound(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/notes/999999", nil)
	req.SetPathValue("id", "999999")
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
