package datenight

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type TimeSlot string

const (
	TimeSlotMorning   TimeSlot = "morning"
	TimeSlotAfternoon TimeSlot = "afternoon"
	TimeSlotEvening   TimeSlot = "evening"
	TimeSlotNight     TimeSlot = "night"
)

var validTimeSlots = map[TimeSlot]bool{
	TimeSlotMorning: true, TimeSlotAfternoon: true, TimeSlotEvening: true, TimeSlotNight: true,
}

func IsValidTimeSlot(t TimeSlot) bool { return validTimeSlots[t] }

type EnergyLevel string

const (
	EnergyCouchPotato EnergyLevel = "couch_potato"
	EnergyCasual      EnergyLevel = "casual"
	EnergyAdventurous EnergyLevel = "adventurous"
	EnergyUnstoppable EnergyLevel = "unstoppable"
)

var validEnergyLevels = map[EnergyLevel]bool{
	EnergyCouchPotato: true, EnergyCasual: true, EnergyAdventurous: true, EnergyUnstoppable: true,
}

func IsValidEnergyLevel(e EnergyLevel) bool { return validEnergyLevels[e] }

type Mood string

const (
	MoodRomantic    Mood = "romantic"
	MoodPlayful     Mood = "playful"
	MoodNostalgic   Mood = "nostalgic"
	MoodCozy        Mood = "cozy"
	MoodExcited     Mood = "excited"
	MoodChill       Mood = "chill"
	MoodSentimental Mood = "sentimental"
	MoodSilly       Mood = "silly"
)

var validMoods = map[Mood]bool{
	MoodRomantic: true, MoodPlayful: true, MoodNostalgic: true, MoodCozy: true,
	MoodExcited: true, MoodChill: true, MoodSentimental: true, MoodSilly: true,
}

func IsValidMood(m Mood) bool { return validMoods[m] }

type ProposalStatus string

const (
	StatusPending  ProposalStatus = "pending"
	StatusAccepted ProposalStatus = "accepted"
	StatusDeclined ProposalStatus = "declined"
)

// DateOnly is a calendar date with no time-of-day component. A Postgres
// DATE column scans into a time.Time, which would marshal as
// "2026-08-20T00:00:00Z" — and the frontend renders this value straight
// into the proposal card, so the wire format is the display format.
// Scanning and both JSON directions are implemented here so nothing
// downstream has to remember to reformat.
type DateOnly time.Time

const dateLayout = "2006-01-02"

func (d *DateOnly) Scan(src any) error {
	t, ok := src.(time.Time)
	if !ok {
		return fmt.Errorf("scanning date: expected time.Time, got %T", src)
	}
	*d = DateOnly(t)
	return nil
}

func (d DateOnly) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(time.Time(d).Format(dateLayout))), nil
}

// UnmarshalJSON exists for the handler tests, which decode a response
// body back into a Proposal.
func (d *DateOnly) UnmarshalJSON(data []byte) error {
	raw, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}
	t, err := time.Parse(dateLayout, raw)
	if err != nil {
		return err
	}
	*d = DateOnly(t)
	return nil
}

type Proposal struct {
	ID                 int64          `json:"id"`
	ActivityID         int64          `json:"activity_id"`
	Date               DateOnly       `json:"date"`
	TimeSlot           TimeSlot       `json:"time_slot"`
	EnergyLevel        EnergyLevel    `json:"energy_level"`
	Moods              []Mood         `json:"moods"`
	Status             ProposalStatus `json:"status"`
	ProposedByUserID   int64          `json:"proposed_by_user_id"`
	ProposedByUsername string         `json:"proposed_by_username"`
	CreatedAt          time.Time      `json:"created_at"`
}

// ErrProposalNotActionable means the caller can't act on the target
// proposal: it isn't the current (most recent) one, its status is no
// longer "pending", or the caller is the person who proposed it — see the
// design spec's "Current proposal" definition and its "the other person
// accepts or declines" framing. One error for all three because the
// UPDATE decides all three in one atomic WHERE clause.
var ErrProposalNotActionable = errors.New("proposal is not the current pending proposal, or the caller proposed it")

// ErrUnknownActivity means activity_id doesn't exist. Detected from the
// FK violation rather than a pre-flight SELECT, so a concurrent activity
// delete can't slip between the check and the insert.
var ErrUnknownActivity = errors.New("unknown activity id")

func joinMoods(moods []Mood) string {
	parts := make([]string, len(moods))
	for i, m := range moods {
		parts[i] = string(m)
	}
	return strings.Join(parts, ",")
}

func parseMoods(raw string) []Mood {
	if raw == "" {
		return []Mood{}
	}
	parts := strings.Split(raw, ",")
	moods := make([]Mood, len(parts))
	for i, part := range parts {
		moods[i] = Mood(part)
	}
	return moods
}

func (s *Store) ListProposals(ctx context.Context) ([]Proposal, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT p.id, p.activity_id, p.date, p.time_slot, p.energy_level, p.moods, p.status,
		       p.proposed_by_user_id, COALESCE(u.username, 'someone'), p.created_at
		FROM date_night_proposals p
		LEFT JOIN users u ON u.id = p.proposed_by_user_id
		ORDER BY p.created_at DESC, p.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	proposals := []Proposal{}
	for rows.Next() {
		var p Proposal
		var moodsRaw string
		if err := rows.Scan(&p.ID, &p.ActivityID, &p.Date, &p.TimeSlot, &p.EnergyLevel, &moodsRaw, &p.Status, &p.ProposedByUserID, &p.ProposedByUsername, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.Moods = parseMoods(moodsRaw)
		proposals = append(proposals, p)
	}
	return proposals, rows.Err()
}

func (s *Store) CreateProposal(ctx context.Context, activityID int64, date time.Time, slot TimeSlot, energy EnergyLevel, moods []Mood, proposedByUserID int64) (Proposal, error) {
	var p Proposal
	var moodsRaw string
	err := s.conn.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO date_night_proposals (activity_id, date, time_slot, energy_level, moods, status, proposed_by_user_id)
			VALUES ($1, $2, $3, $4, $5, 'pending', $6)
			RETURNING id, activity_id, date, time_slot, energy_level, moods, status, proposed_by_user_id, created_at
		)
		SELECT i.id, i.activity_id, i.date, i.time_slot, i.energy_level, i.moods, i.status,
		       i.proposed_by_user_id, COALESCE(u.username, 'someone'), i.created_at
		FROM inserted i
		LEFT JOIN users u ON u.id = i.proposed_by_user_id
	`, activityID, date, slot, energy, joinMoods(moods), proposedByUserID).
		Scan(&p.ID, &p.ActivityID, &p.Date, &p.TimeSlot, &p.EnergyLevel, &moodsRaw, &p.Status, &p.ProposedByUserID, &p.ProposedByUsername, &p.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return Proposal{}, ErrUnknownActivity
		}
		return Proposal{}, err
	}
	p.Moods = parseMoods(moodsRaw)
	return p, nil
}

// SetProposalStatus transitions id to status, but only if id is the
// current (most recent) proposal, is still pending, and wasn't proposed
// by respondingUserID — all three atomically, via the WHERE clause, so a
// race between two accept/decline calls can't both succeed.
func (s *Store) SetProposalStatus(ctx context.Context, id int64, status ProposalStatus, respondingUserID int64) (Proposal, error) {
	var p Proposal
	var moodsRaw string
	err := s.conn.QueryRowContext(ctx, `
		WITH updated AS (
			UPDATE date_night_proposals
			SET status = $2
			WHERE id = $1
			  AND status = 'pending'
			  AND proposed_by_user_id <> $3
			  AND id = (SELECT id FROM date_night_proposals ORDER BY created_at DESC, id DESC LIMIT 1)
			RETURNING id, activity_id, date, time_slot, energy_level, moods, status, proposed_by_user_id, created_at
		)
		SELECT up.id, up.activity_id, up.date, up.time_slot, up.energy_level, up.moods, up.status,
		       up.proposed_by_user_id, COALESCE(u.username, 'someone'), up.created_at
		FROM updated up
		LEFT JOIN users u ON u.id = up.proposed_by_user_id
	`, id, status, respondingUserID).
		Scan(&p.ID, &p.ActivityID, &p.Date, &p.TimeSlot, &p.EnergyLevel, &moodsRaw, &p.Status, &p.ProposedByUserID, &p.ProposedByUsername, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Proposal{}, ErrProposalNotActionable
	}
	if err != nil {
		return Proposal{}, err
	}
	p.Moods = parseMoods(moodsRaw)
	return p, nil
}
