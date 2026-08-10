// Package platform holds small helpers shared across handler packages
// (auth, notes, watchlist, ...) — kept intentionally tiny so it doesn't
// grow into a dumping ground for domain logic.
package platform

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// WriteJSON writes v as a JSON response body with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// IDParam parses the "id" path value from r as an int64, e.g. for routes
// registered as "DELETE /things/{id}". ok is false if the value is
// missing or not a valid integer — callers should respond 400 in that
// case rather than treating it as a not-found.
func IDParam(r *http.Request) (id int64, ok bool) {
	parsed, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
