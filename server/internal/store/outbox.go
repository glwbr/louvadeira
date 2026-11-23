package store

import (
	"context"
	"encoding/json"
	"time"
)

const (
	MessageText    = "text"
	MessageMention = "mention"
)

type OutboxMessage struct {
	ID        int64
	Recipient string
	Kind      string
	Text      string
	Mentions  []string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func (s *Store) EnqueueMessage(ctx context.Context, m OutboxMessage) (int64, error) {
	mentions, err := json.Marshal(append([]string{}, m.Mentions...))
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO outbox (recipient, kind, text, mentions, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id`, m.Recipient, m.Kind, m.Text, string(mentions), formatTime(m.CreatedAt), formatTime(m.ExpiresAt)).Scan(&id)
	return id, err
}

/**
 * NOTE: The messages still to send, oldest first, for one recipient or, with an empty recipient, for everyone.
 */
func (s *Store) PendingMessages(ctx context.Context, recipient string) ([]OutboxMessage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, recipient, kind, text, mentions, created_at, expires_at FROM outbox
		WHERE status = 'pending' AND (? = '' OR recipient = ?) ORDER BY id`, recipient, recipient)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []OutboxMessage
	for rows.Next() {
		var m OutboxMessage
		var mentions, created, expires string
		if err := rows.Scan(&m.ID, &m.Recipient, &m.Kind, &m.Text, &mentions, &created, &expires); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(mentions), &m.Mentions); err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(timeLayout, created)
		m.ExpiresAt, _ = time.Parse(timeLayout, expires)
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *Store) MessageSent(ctx context.Context, id int64, now time.Time) error {
	_, err := s.db.ExecContext(ctx, "UPDATE outbox SET status = 'sent', sent_at = ?, attempts = attempts + 1 WHERE id = ?", formatTime(now), id)
	return err
}

func (s *Store) MessageDropped(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE outbox SET status = 'dropped' WHERE id = ?", id)
	return err
}

func (s *Store) MessageFailed(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE outbox SET attempts = attempts + 1 WHERE id = ?", id)
	return err
}

// NOTE: The record of sent and dropped messages is kept for a while, to answer "did the bot send it?".
func (s *Store) PruneOutbox(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM outbox WHERE status != 'pending' AND created_at < ?", formatTime(before))
	return err
}
