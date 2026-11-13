package store

import (
	"context"
	"strings"
	"time"
)

type Unavailability struct {
	ID         int64  `json:"id"`
	PersonID   int64  `json:"personId"`
	PersonName string `json:"personName"`
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
	Note       string `json:"note"`
}

type UnavailabilityInput struct {
	PersonID  int64  `json:"personId"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Note      string `json:"note"`
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
	end := start
	if in.EndDate != "" {
		if end, err = validDate(in.EndDate, "data final"); err != nil {
			return in, err
		}
	}
	if end < start {
		return in, invalid("a data final vem antes da inicial")
	}
	note := strings.TrimSpace(in.Note)
	if len([]rune(note)) > 200 {
		return in, invalid("o motivo pode ter no máximo 200 caracteres")
	}
	return UnavailabilityInput{PersonID: in.PersonID, StartDate: start, EndDate: end, Note: note}, nil
}

/**
 * NOTE: Returns every unavailability that overlaps [from, to], for people who are not archived.
 */
func (s *Store) ListUnavailabilities(ctx context.Context, from, to string) ([]Unavailability, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.person_id, p.name, u.start_date, u.end_date, u.note
		FROM unavailabilities u JOIN people p ON p.id = u.person_id
		WHERE u.start_date <= ? AND u.end_date >= ? AND p.archived_at IS NULL
		ORDER BY u.start_date, p.name COLLATE NOCASE`, to, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Unavailability{}
	for rows.Next() {
		var u Unavailability
		if err := rows.Scan(&u.ID, &u.PersonID, &u.PersonName, &u.StartDate, &u.EndDate, &u.Note); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

func (s *Store) CreateUnavailability(ctx context.Context, in UnavailabilityInput) (Unavailability, error) {
	in, err := in.clean()
	if err != nil {
		return Unavailability{}, err
	}
	u := Unavailability{PersonID: in.PersonID, StartDate: in.StartDate, EndDate: in.EndDate, Note: in.Note}
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO unavailabilities (person_id, start_date, end_date, note)
		SELECT id, ?, ?, ? FROM people WHERE id = ? AND archived_at IS NULL
		RETURNING id, (SELECT name FROM people WHERE id = person_id)`,
		in.StartDate, in.EndDate, in.Note, in.PersonID).Scan(&u.ID, &u.PersonName)
	return u, notFoundIfNoRows(err)
}

func (s *Store) DeleteUnavailability(ctx context.Context, id int64) error {
	return requireAffected(s.db.ExecContext(ctx, "DELETE FROM unavailabilities WHERE id = ?", id))
}
