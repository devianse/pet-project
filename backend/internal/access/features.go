// backend/internal/access/features.go
package access

// Feature is one gate-able page/capability. Key is what's stored in the
// feature_access table and what the frontend's FeatureKey union must stay
// in sync with (frontend/src/shared/api.ts) — there's no single source of
// truth shared across the language boundary, so both sides are hand-kept,
// same as route paths already are between cmd/api/main.go and App.tsx.
type Feature struct {
	Key   string
	Label string
}

// KnownFeatures is the fixed set of gate-able features. Adding a new one
// is "add a line here" — EnsureSchema seeds it into the features table on
// next startup, no manual DB step.
var KnownFeatures = []Feature{
	{Key: "notes", Label: "Notes"},
	{Key: "watchlist", Label: "Watchlist"},
	{Key: "date-night", Label: "Date Night"},
	{Key: "shopping-list", Label: "Shopping List"},
	{Key: "image-processing", Label: "Image Processing"},
}

// IsKnownFeature reports whether key is one of KnownFeatures — used to
// reject an invalid key with a clear error before it ever reaches the DB
// (grantaccess's -grant/-revoke validation).
func IsKnownFeature(key string) bool {
	for _, f := range KnownFeatures {
		if f.Key == key {
			return true
		}
	}
	return false
}
