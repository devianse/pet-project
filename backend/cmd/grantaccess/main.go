// backend/cmd/grantaccess/main.go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/devianse/pet-project/backend/internal/access"
	"github.com/devianse/pet-project/backend/internal/auth"
	"github.com/devianse/pet-project/backend/internal/db"
	"github.com/joho/godotenv"
)

func main() {
	username := flag.String("username", "", "username to grant/revoke/list access for")
	grant := flag.String("grant", "", "feature key to grant")
	revoke := flag.String("revoke", "", "feature key to revoke")
	list := flag.Bool("list", false, "list the resolved feature set for -username")
	flag.Parse()

	if *username == "" {
		log.Fatal("-username is required")
	}
	if err := validateMode(*grant, *revoke, *list); err != nil {
		log.Fatal(err)
	}

	_ = godotenv.Load()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	conn, err := db.Open(databaseURL)
	if err != nil {
		log.Fatalf("opening db: %v", err)
	}
	defer conn.Close()

	authStore := auth.NewStore(conn)
	if err := authStore.EnsureSchema(context.Background()); err != nil {
		log.Fatalf("ensuring auth schema: %v", err)
	}
	accessStore := access.NewStore(conn)
	if err := accessStore.EnsureSchema(context.Background()); err != nil {
		log.Fatalf("ensuring access schema: %v", err)
	}

	out, err := run(context.Background(), authStore, accessStore, *username, *grant, *revoke, *list)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}

// validateMode enforces exactly one of grant/revoke/list.
func validateMode(grant, revoke string, list bool) error {
	set := 0
	if grant != "" {
		set++
	}
	if revoke != "" {
		set++
	}
	if list {
		set++
	}
	if set != 1 {
		return errors.New("exactly one of -grant, -revoke, or -list is required")
	}
	return nil
}

// knownFeatureKeys renders KnownFeatures as a comma-joined string for
// error messages, e.g. "notes, watchlist, date-night, shopping-list, image-processing".
func knownFeatureKeys() string {
	keys := make([]string, len(access.KnownFeatures))
	for i, f := range access.KnownFeatures {
		keys[i] = f.Key
	}
	return strings.Join(keys, ", ")
}

// run performs the requested grant/revoke/list against real stores and
// returns the message to print — split from main so tests can call it
// directly against a live test DB without spawning a process.
func run(ctx context.Context, authStore *auth.Store, accessStore *access.Store, username, grant, revoke string, list bool) (string, error) {
	user, err := authStore.FindByUsername(ctx, username)
	if err != nil {
		return "", fmt.Errorf("%q: %w", username, err)
	}
	// FindByUsername returns (nil, nil) when no row matches, rather than
	// an error, so an unknown username must be checked separately.
	if user == nil {
		return "", fmt.Errorf("no such user %q", username)
	}

	switch {
	case grant != "":
		if !access.IsKnownFeature(grant) {
			return "", fmt.Errorf("unknown feature key %q (known: %s)", grant, knownFeatureKeys())
		}
		if err := accessStore.Grant(ctx, user.ID, grant); err != nil {
			return "", fmt.Errorf("granting %q to %q: %w", grant, username, err)
		}
		return fmt.Sprintf("granted %q to %q", grant, username), nil

	case revoke != "":
		if !access.IsKnownFeature(revoke) {
			return "", fmt.Errorf("unknown feature key %q (known: %s)", revoke, knownFeatureKeys())
		}
		revoked, err := accessStore.Revoke(ctx, user.ID, revoke)
		if err != nil {
			return "", fmt.Errorf("revoking %q from %q: %w", revoke, username, err)
		}
		if !revoked {
			return fmt.Sprintf("%q did not have %q granted", username, revoke), nil
		}
		return fmt.Sprintf("revoked %q from %q", revoke, username), nil

	case list:
		features, err := accessStore.ListAllForUser(ctx, user.ID, user.Role)
		if err != nil {
			return "", fmt.Errorf("listing features for %q: %w", username, err)
		}
		if len(features) == 0 {
			return fmt.Sprintf("%q (role %s) has no features granted", username, user.Role), nil
		}
		return fmt.Sprintf("%q (role %s): %s", username, user.Role, strings.Join(features, ", ")), nil
	}

	// unreachable: validateMode already guarantees exactly one branch above ran.
	return "", errors.New("no mode selected")
}
