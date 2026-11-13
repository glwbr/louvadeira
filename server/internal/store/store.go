package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var (
	ErrNotFound = errors.New("não encontrado")
	ErrConflict = errors.New("já existe")
)

/**
 * NOTE: ValidationError carries a message meant for the person filling the form, so handlers can show it as is.
 */
type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func invalid(format string, args ...any) error {
	return ValidationError{Message: fmt.Sprintf(format, args...)}
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

const timeLayout = "2006-01-02T15:04:05.000Z"

func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func (s *Store) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func isUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) &&
		(sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE || sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY)
}

func isForeignKeyViolation(err error) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
}

func notFoundIfNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func requireAffected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func cleanName(raw, field string) (string, error) {
	name := strings.Join(strings.Fields(raw), " ")
	if name == "" {
		return "", invalid("%s é obrigatório", field)
	}
	if len([]rune(name)) > 80 {
		return "", invalid("%s pode ter no máximo 80 caracteres", field)
	}
	return name, nil
}

/**
 * NOTE: Numbers typed without "+" and with 10 or 11 digits (DDD and number, as Brazilians write them) get 55 in
 * front, so "(11) 98888-7777" and "+55 11 98888-7777" end up as the same person.
 */
func NormalizePhone(raw string) (string, error) {
	var digits strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		}
	}
	phone := digits.String()
	international := strings.HasPrefix(strings.TrimSpace(raw), "+")
	switch {
	case phone == "":
		return "", nil
	case !international && (len(phone) == 10 || len(phone) == 11):
		phone = "55" + phone
	}
	brazilian := strings.HasPrefix(phone, "55")
	if (brazilian && len(phone) != 12 && len(phone) != 13) || len(phone) < 8 || len(phone) > 15 {
		return "", invalid("telefone inválido: use DDD e número, como (11) 98888-7777")
	}
	return phone, nil
}

func nullable(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
