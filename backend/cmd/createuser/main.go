package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/devianse/pet-project/backend/internal/auth"
	"github.com/devianse/pet-project/backend/internal/db"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/joho/godotenv"
	"golang.org/x/term"
)

func main() {
	username := flag.String("username", "", "username for the new account (required)")
	role := flag.String("role", "user", `account role: "admin" or "user"`)
	displayName := flag.String("display-name", "", "optional display name shown in the UI instead of the username")
	flag.Parse()

	if *username == "" {
		fmt.Fprintln(os.Stderr, "error: -username is required")
		os.Exit(1)
	}
	if *role != "admin" && *role != "user" {
		fmt.Fprintf(os.Stderr, "error: -role must be \"admin\" or \"user\", got %q\n", *role)
		os.Exit(1)
	}

	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, using process env")
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "error: DATABASE_URL is not set")
		os.Exit(1)
	}

	password, err := readPassword()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading password: %v\n", err)
		os.Exit(1)
	}

	conn, err := db.Open(databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	store := auth.NewStore(conn)
	if err := store.EnsureSchema(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error ensuring users schema: %v\n", err)
		os.Exit(1)
	}

	if err := createUser(context.Background(), store, *username, password, *role, *displayName); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("created user %q with role %q\n", *username, *role)
}

// createUser hashes the password and inserts the row, translating a
// unique-constraint violation into a clear message instead of a raw
// pgconn error. Split out from main so it's testable without stdin.
// displayName is optional — an empty string stores NULL, same as
// omitting it from the self-service PATCH /api/me endpoint.
func createUser(ctx context.Context, store *auth.Store, username, password, role, displayName string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	var displayNamePtr *string
	if trimmed := strings.TrimSpace(displayName); trimmed != "" {
		displayNamePtr = &trimmed
	}

	if _, err := store.CreateUserWithDisplayName(ctx, username, hash, role, displayNamePtr); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("username %q already exists", username)
		}
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

// readPassword prompts on stdout and reads from stdin without echoing —
// a -password flag would land the password in shell history instead.
func readPassword() (string, error) {
	fmt.Print("Password: ")
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", err
	}
	password := strings.TrimSpace(string(bytePassword))
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	return password, nil
}
