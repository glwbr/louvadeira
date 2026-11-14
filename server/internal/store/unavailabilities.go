package store

import (
	"context"
	"database/sql"
	"slices"
	"time"
)

type Reason string

const (
	ReasonTravel  Reason = "viagem"
	ReasonWork    Reason = "trabalho"
	ReasonHealth  Reason = "saude"
	ReasonFamily  Reason = "familia"
	ReasonStudies Reason = "estudos"
	ReasonOther   Reason = "outro"
)

func (r Reason) valid() bool {
	switch r {
	case ReasonTravel, ReasonWork, ReasonHealth, ReasonFamily, ReasonStudies, ReasonOther:
		return true
	}
	return false
}

type Repeat string

const (
	RepeatNone    Repeat = "none"
	RepeatWeekly  Repeat = "weekly"
	RepeatMonthly Repeat = "monthly"
)

// NOTE: The end date stored for a repeating absence with no end; the API shows it as an empty endDate.
const forever = "9999-12-31"

type Unavailability struct {
	ID          int64  `json:"id"`
	PersonID    int64  `json:"personId"`
	PersonName  string `json:"personName"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Reason      Reason `json:"reason"`
	Repeat      Repeat `json:"repeat"`
	Weekday     *int   `json:"weekday"`
	WeekOfMonth *int   `json:"weekOfMonth"`
	// NOTE: FromPoll is true for days the person marked in the monthly availability poll.
	FromPoll bool `json:"fromPoll"`
}

/**
 * NOTE: Covers reports whether the absence includes a date: any day of its range, or only the matching weekday (and
 * week of the month) when it repeats.
 */
func (u Unavailability) Covers(date string) bool {
	if date < u.StartDate || date > u.Until() {
		return false
	}
	if u.Repeat == RepeatNone || u.Repeat == "" {
		return true
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil || u.Weekday == nil || int(t.Weekday()) != *u.Weekday {
		return false
	}
	return u.Repeat == RepeatWeekly || (u.WeekOfMonth != nil && nthWeekdayOfMonth(date) == *u.WeekOfMonth)
}

/**
 * NOTE: Until is the last day the absence can cover, far in the future when it repeats with no end.
 */
func (u Unavailability) Until() string {
	if u.EndDate == "" {
		return forever
	}
	return u.EndDate
}

type UnavailabilityInput struct {
	PersonID    int64  `json:"personId"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Reason      Reason `json:"reason"`
	Repeat      Repeat `json:"repeat"`
	Weekday     *int   `json:"weekday"`
	WeekOfMonth *int   `json:"weekOfMonth"`
}

func validDate(value, field string) (string, error) {
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return "", invalid("%s inválida: use AAAA-MM-DD", field)
	}
	return t.Format("2006-01-02"), nil
}

func (in UnavailabilityInput) clean() (UnavailabilityInput, error) {
	start, err := validDate(in.StartDate, "data inicial")
	if err != nil {
		return in, err
	}
	out := UnavailabilityInput{PersonID: in.PersonID, StartDate: start, EndDate: start, Reason: in.Reason, Repeat: in.Repeat}
	switch in.Repeat {
	case "", RepeatNone:
		out.Repeat = RepeatNone
	case RepeatWeekly, RepeatMonthly:
		if in.Weekday == nil || *in.Weekday < 0 || *in.Weekday > 6 {
			return in, invalid("escolha o dia da semana")
		}
		out.Weekday = in.Weekday
		if in.Repeat == RepeatMonthly {
			if in.WeekOfMonth == nil || *in.WeekOfMonth < 1 || *in.WeekOfMonth > 4 {
				return in, invalid("escolha a semana do mês")
			}
			out.WeekOfMonth = in.WeekOfMonth
		}
		out.EndDate = forever
	default:
		return in, invalid("repetição inválida")
	}
	if in.EndDate != "" {
		if out.EndDate, err = validDate(in.EndDate, "data final"); err != nil {
			return in, err
		}
	}
	if out.EndDate < start {
		return in, invalid("a data final vem antes da inicial")
	}
	if !in.Reason.valid() {
		return in, invalid("escolha o motivo")
	}
	return out, nil
}

const unavailabilityColumns = `u.id, u.person_id, p.name, u.start_date, u.end_date, u.reason, u.repeat, u.weekday, u.week_of_month, u.source IS NOT NULL`

func scanUnavailability(row interface{ Scan(...any) error }) (Unavailability, error) {
	var u Unavailability
	var weekday, week sql.NullInt64
	err := row.Scan(&u.ID, &u.PersonID, &u.PersonName, &u.StartDate, &u.EndDate, &u.Reason, &u.Repeat, &weekday, &week, &u.FromPoll)
	if u.EndDate == forever {
		u.EndDate = ""
	}
	if weekday.Valid {
		v := int(weekday.Int64)
		u.Weekday = &v
	}
	if week.Valid {
		v := int(week.Int64)
		u.WeekOfMonth = &v
	}
	return u, err
}

func (s *Store) queryUnavailabilities(ctx context.Context, where string, args ...any) ([]Unavailability, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+unavailabilityColumns+`
		FROM unavailabilities u JOIN people p ON p.id = u.person_id
		WHERE `+where+`
		ORDER BY u.start_date, p.name COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Unavailability{}
	for rows.Next() {
		u, err := scanUnavailability(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

/**
 * NOTE: Returns every unavailability whose range overlaps [from, to], for people who are not archived. A repeating
 * one is listed even when none of its days fall in the range; Covers tells the days apart.
 */
func (s *Store) ListUnavailabilities(ctx context.Context, from, to string) ([]Unavailability, error) {
	return s.queryUnavailabilities(ctx, "u.start_date <= ? AND u.end_date >= ? AND p.archived_at IS NULL", to, from)
}

/**
 * NOTE: A person's absences that still matter: the ones not over before from.
 */
func (s *Store) PersonUnavailabilities(ctx context.Context, personID int64, from string) ([]Unavailability, error) {
	return s.queryUnavailabilities(ctx, "u.person_id = ? AND u.end_date >= ?", personID, from)
}

func (s *Store) CreateUnavailability(ctx context.Context, in UnavailabilityInput) (Unavailability, error) {
	in, err := in.clean()
	if err != nil {
		return Unavailability{}, err
	}
	var id int64
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO unavailabilities (person_id, start_date, end_date, reason, repeat, weekday, week_of_month)
		SELECT id, ?, ?, ?, ?, ?, ? FROM people WHERE id = ? AND archived_at IS NULL
		RETURNING id`,
		in.StartDate, in.EndDate, in.Reason, in.Repeat, in.Weekday, in.WeekOfMonth, in.PersonID).Scan(&id)
	if err != nil {
		return Unavailability{}, notFoundIfNoRows(err)
	}
	return s.Unavailability(ctx, id)
}

func (s *Store) Unavailability(ctx context.Context, id int64) (Unavailability, error) {
	u, err := scanUnavailability(s.db.QueryRowContext(ctx, `
		SELECT `+unavailabilityColumns+`
		FROM unavailabilities u JOIN people p ON p.id = u.person_id WHERE u.id = ?`, id))
	return u, notFoundIfNoRows(err)
}

/**
 * NOTE: Replaces the days a person marked in one availability poll with the ones in their latest answer, and reports
 * whether anything changed, so the same answer arriving twice is acknowledged once.
 */
func (s *Store) ReplacePollAbsences(ctx context.Context, personID int64, source string, days []string) (bool, error) {
	changed := false
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT start_date FROM unavailabilities WHERE person_id = ? AND source = ? ORDER BY start_date", personID, source)
		if err != nil {
			return err
		}
		var before []string
		for rows.Next() {
			var day string
			if err := rows.Scan(&day); err != nil {
				rows.Close()
				return err
			}
			before = append(before, day)
		}
		rows.Close()
		after := slices.Sorted(slices.Values(days))
		var answered bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM notifications WHERE key = ?)", "answered:"+source).Scan(&answered); err != nil {
			return err
		}
		if answered && slices.Equal(before, after) {
			return nil
		}
		changed = true
		if _, err := tx.ExecContext(ctx, "INSERT INTO notifications (key, sent_at) VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now')) ON CONFLICT DO NOTHING", "answered:"+source); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM unavailabilities WHERE person_id = ? AND source = ?", personID, source); err != nil {
			return err
		}
		for _, day := range days {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO unavailabilities (person_id, start_date, end_date, reason, source) VALUES (?, ?, ?, 'outro', ?)`,
				personID, day, day, source); err != nil {
				return err
			}
		}
		return nil
	})
	return changed, err
}

func (s *Store) DeleteUnavailability(ctx context.Context, id int64) error {
	return requireAffected(s.db.ExecContext(ctx, "DELETE FROM unavailabilities WHERE id = ?", id))
}
