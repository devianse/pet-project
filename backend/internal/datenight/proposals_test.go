package datenight

import (
	"context"
	"errors"
	"testing"
	"time"
)

func mustCreateActivity(t *testing.T, store *Store) Activity {
	t.Helper()
	a, err := store.CreateActivity(context.Background(), "Movie Night", nil, CategoryCozy)
	if err != nil {
		t.Fatalf("CreateActivity: %v", err)
	}
	return a
}

func TestStore_CreateProposalThenListProposals(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	activity := mustCreateActivity(t, store)

	date := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	moods := []Mood{MoodRomantic, MoodPlayful}
	created, err := store.CreateProposal(ctx, activity.ID, date, TimeSlotEvening, EnergyCasual, moods, 1)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	if created.Status != StatusPending {
		t.Fatalf("expected new proposal to be pending, got %q", created.Status)
	}
	if len(created.Moods) != 2 || created.Moods[0] != MoodRomantic || created.Moods[1] != MoodPlayful {
		t.Fatalf("expected moods to round-trip in order, got %+v", created.Moods)
	}

	listed, err := store.ListProposals(ctx)
	if err != nil {
		t.Fatalf("ListProposals: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("expected 1 proposal matching created, got %+v", listed)
	}
}

func TestStore_ListProposals_NewestFirst(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	activity := mustCreateActivity(t, store)
	date := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	first, err := store.CreateProposal(ctx, activity.ID, date, TimeSlotEvening, EnergyCasual, []Mood{MoodChill}, 1)
	if err != nil {
		t.Fatalf("CreateProposal (first): %v", err)
	}
	second, err := store.CreateProposal(ctx, activity.ID, date, TimeSlotMorning, EnergyUnstoppable, []Mood{MoodExcited}, 2)
	if err != nil {
		t.Fatalf("CreateProposal (second): %v", err)
	}

	listed, err := store.ListProposals(ctx)
	if err != nil {
		t.Fatalf("ListProposals: %v", err)
	}
	if len(listed) != 2 || listed[0].ID != second.ID || listed[1].ID != first.ID {
		t.Fatalf("expected newest-first order [%d, %d], got %+v", second.ID, first.ID, listed)
	}
}

func TestStore_SetProposalStatus_AcceptsCurrentPending(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	activity := mustCreateActivity(t, store)
	date := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	created, err := store.CreateProposal(ctx, activity.ID, date, TimeSlotEvening, EnergyCasual, []Mood{MoodChill}, 1)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}

	// Proposed by user 1, answered by user 2 — the pair's other member.
	updated, err := store.SetProposalStatus(ctx, created.ID, StatusAccepted, 2)
	if err != nil {
		t.Fatalf("SetProposalStatus: %v", err)
	}
	if updated.Status != StatusAccepted {
		t.Fatalf("expected status accepted, got %q", updated.Status)
	}
}

func TestStore_SetProposalStatus_RejectsSelfResponse(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	activity := mustCreateActivity(t, store)
	date := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	created, err := store.CreateProposal(ctx, activity.ID, date, TimeSlotEvening, EnergyCasual, []Mood{MoodChill}, 1)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}

	// Same user id that proposed it — the invitation is for the other person.
	_, err = store.SetProposalStatus(ctx, created.ID, StatusAccepted, 1)
	if !errors.Is(err, ErrProposalNotActionable) {
		t.Fatalf("expected ErrProposalNotActionable for a self-response, got %v", err)
	}
}

func TestStore_CreateProposal_UnknownActivity(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	date := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	_, err := store.CreateProposal(ctx, 999999, date, TimeSlotEvening, EnergyCasual, []Mood{MoodChill}, 1)
	if !errors.Is(err, ErrUnknownActivity) {
		t.Fatalf("expected ErrUnknownActivity, got %v", err)
	}
}

func TestStore_DeleteActivity_InUse(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	activity := mustCreateActivity(t, store)
	date := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	if _, err := store.CreateProposal(ctx, activity.ID, date, TimeSlotEvening, EnergyCasual, []Mood{MoodChill}, 1); err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}

	// Task 2 wrote DeleteActivity's ErrActivityInUse branch, but nothing
	// could reach it until proposals existed to hold the FK.
	if _, err := store.DeleteActivity(ctx, activity.ID); !errors.Is(err, ErrActivityInUse) {
		t.Fatalf("expected ErrActivityInUse, got %v", err)
	}
}

func TestStore_SetProposalStatus_RejectsNonCurrentProposal(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	activity := mustCreateActivity(t, store)
	date := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	older, err := store.CreateProposal(ctx, activity.ID, date, TimeSlotEvening, EnergyCasual, []Mood{MoodChill}, 1)
	if err != nil {
		t.Fatalf("CreateProposal (older): %v", err)
	}
	// A second proposal supersedes "current" without touching the first
	// row's status — see the design spec's "no explicit reset" note.
	if _, err := store.CreateProposal(ctx, activity.ID, date, TimeSlotMorning, EnergyCasual, []Mood{MoodChill}, 2); err != nil {
		t.Fatalf("CreateProposal (newer): %v", err)
	}

	_, err = store.SetProposalStatus(ctx, older.ID, StatusAccepted, 2)
	if !errors.Is(err, ErrProposalNotActionable) {
		t.Fatalf("expected ErrProposalNotActionable, got %v", err)
	}
}

func TestStore_SetProposalStatus_RejectsAlreadyDecided(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	activity := mustCreateActivity(t, store)
	date := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	created, err := store.CreateProposal(ctx, activity.ID, date, TimeSlotEvening, EnergyCasual, []Mood{MoodChill}, 1)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	if _, err := store.SetProposalStatus(ctx, created.ID, StatusAccepted, 2); err != nil {
		t.Fatalf("SetProposalStatus (accept): %v", err)
	}

	_, err = store.SetProposalStatus(ctx, created.ID, StatusDeclined, 2)
	if !errors.Is(err, ErrProposalNotActionable) {
		t.Fatalf("expected ErrProposalNotActionable for an already-decided proposal, got %v", err)
	}
}
