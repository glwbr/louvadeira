package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

var ErrInvalidToken = errors.New("link de acesso inválido, expirado ou já usado")

func newToken() (raw, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

// NOTE: Only hashes are stored, so a copy of the database cannot be used to sign in.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Store) CreateLoginToken(ctx context.Context, personID int64, now time.Time, ttl time.Duration) (string, error) {
	raw, hash, err := newToken()
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO login_tokens (token_hash, person_id, expires_at) VALUES (?, ?, ?)",
		hash, personID, formatTime(now.Add(ttl)))
	if isForeignKeyViolation(err) {
		return "", ErrNotFound
	}
	return raw, err
}

/**
 * INVARIANT: A login token turns into at most one session: it is marked used in the same transaction that
 * creates the session, and expired or used tokens are rejected.
 */
func (s *Store) RedeemLoginToken(ctx context.Context, raw string, now time.Time, sessionTTL time.Duration) (string, Person, error) {
	var session string
	var person Person
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var personID int64
		err := tx.QueryRowContext(ctx, `
			UPDATE login_tokens SET used_at = ?
			WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?
			RETURNING person_id`, formatTime(now), hashToken(raw), formatTime(now)).Scan(&personID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidToken
		}
		if err != nil {
			return err
		}
		person, err = scanPerson(tx.QueryRowContext(ctx,
			"SELECT "+personColumns+" FROM people WHERE id = ? AND archived_at IS NULL", personID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidToken
		}
		if err != nil {
			return err
		}
		var hash string
		session, hash, err = newToken()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			"INSERT INTO sessions (token_hash, person_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
			hash, personID, formatTime(now), formatTime(now.Add(sessionTTL)))
		return err
	})
	return session, person, err
}

func (s *Store) SessionPerson(ctx context.Context, raw string, now time.Time) (Person, error) {
	p, err := scanPerson(s.db.QueryRowContext(ctx, `
		SELECT p.id, p.name, coalesce(p.phone, ''), p.is_admin
		FROM sessions s JOIN people p ON p.id = s.person_id
		WHERE s.token_hash = ? AND s.expires_at > ? AND p.archived_at IS NULL`, hashToken(raw), formatTime(now)))
	return p, notFoundIfNoRows(err)
}

func (s *Store) DeleteSession(ctx context.Context, raw string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", hashToken(raw))
	return err
}
