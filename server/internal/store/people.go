package store

import (
	"context"
	"database/sql"
)

type Person struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	IsAdmin bool   `json:"isAdmin"`
}

type PersonInput struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

const (
	personColumns  = "id, name, coalesce(phone, ''), is_admin"
	personColumnsP = "p.id, p.name, coalesce(p.phone, ''), p.is_admin"
)

func scanPerson(row interface{ Scan(...any) error }) (Person, error) {
	var p Person
	err := row.Scan(&p.ID, &p.Name, &p.Phone, &p.IsAdmin)
	return p, err
}

func (in PersonInput) clean() (PersonInput, error) {
	name, err := cleanName(in.Name, "Nome")
	if err != nil {
		return PersonInput{}, err
	}
	phone, err := NormalizePhone(in.Phone)
	if err != nil {
		return PersonInput{}, err
	}
	return PersonInput{Name: name, Phone: phone}, nil
}

func (s *Store) ListPeople(ctx context.Context) ([]Person, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+personColumns+" FROM people WHERE archived_at IS NULL ORDER BY name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	people := []Person{}
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		people = append(people, p)
	}
	return people, rows.Err()
}

func (s *Store) Person(ctx context.Context, id int64) (Person, error) {
	p, err := scanPerson(s.db.QueryRowContext(ctx,
		"SELECT "+personColumns+" FROM people WHERE id = ? AND archived_at IS NULL", id))
	return p, notFoundIfNoRows(err)
}

func (s *Store) PersonByPhone(ctx context.Context, rawPhone string) (Person, error) {
	phone, err := NormalizePhone(rawPhone)
	if err != nil {
		return Person{}, err
	}
	p, err := scanPerson(s.db.QueryRowContext(ctx,
		"SELECT "+personColumns+" FROM people WHERE phone = ? AND archived_at IS NULL", phone))
	return p, notFoundIfNoRows(err)
}

func (s *Store) CreatePerson(ctx context.Context, in PersonInput) (Person, error) {
	in, err := in.clean()
	if err != nil {
		return Person{}, err
	}
	p, err := scanPerson(s.db.QueryRowContext(ctx,
		"INSERT INTO people (name, phone) VALUES (?, ?) RETURNING "+personColumns, in.Name, nullable(in.Phone)))
	if isUniqueViolation(err) {
		return Person{}, invalid("já existe uma pessoa com esse telefone")
	}
	return p, err
}

func (s *Store) UpdatePerson(ctx context.Context, id int64, in PersonInput) (Person, error) {
	in, err := in.clean()
	if err != nil {
		return Person{}, err
	}
	p, err := scanPerson(s.db.QueryRowContext(ctx,
		"UPDATE people SET name = ?, phone = ? WHERE id = ? AND archived_at IS NULL RETURNING "+personColumns,
		in.Name, nullable(in.Phone), id))
	if isUniqueViolation(err) {
		return Person{}, invalid("já existe uma pessoa com esse telefone")
	}
	return p, notFoundIfNoRows(err)
}

/**
 * NOTE: Archiving keeps the person for future schedule history but takes them out of every ministry, ends their
 * sessions and frees their phone number for a new record.
 */
func (s *Store) ArchivePerson(ctx context.Context, id int64) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		err := requireAffected(tx.ExecContext(ctx,
			"UPDATE people SET archived_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), phone = NULL WHERE id = ? AND archived_at IS NULL", id))
		if err != nil {
			return err
		}
		for _, query := range []string{
			"DELETE FROM ministry_members WHERE person_id = ?",
			"DELETE FROM sessions WHERE person_id = ?",
			"DELETE FROM login_tokens WHERE person_id = ?",
		} {
			if _, err := tx.ExecContext(ctx, query, id); err != nil {
				return err
			}
		}
		return nil
	})
}

/**
 * NOTE: Promotes the person with this phone to administrator, creating them if needed. It is the CLI's way in
 * before anyone can sign in.
 */
func (s *Store) EnsureAdmin(ctx context.Context, in PersonInput) (Person, error) {
	in, err := in.clean()
	if err != nil {
		return Person{}, err
	}
	if in.Phone == "" {
		return Person{}, invalid("o administrador precisa de telefone")
	}
	p, err := scanPerson(s.db.QueryRowContext(ctx, `
		INSERT INTO people (name, phone, is_admin) VALUES (?, ?, 1)
		ON CONFLICT (phone) DO UPDATE SET is_admin = 1
		RETURNING `+personColumns, in.Name, in.Phone))
	return p, err
}
