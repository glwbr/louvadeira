package notify

import (
	"context"
	"errors"
	"time"

	"louvadeira/internal/store"
)

// CONSTRAINT: Two minutes between access links to one person, so a leader sending a link twice in a row sends one.
const loginLinkGap = 2 * time.Minute

var ErrTooSoon = errors.New("o link acabou de ser enviado; espere um pouco")

/**
 * NOTE: ConfirmFromApp confirms slots answered on the member page: the same as answering the poll, without the
 * WhatsApp thank-you, since the page already shows the result.
 */
func (b *Bot) ConfirmFromApp(ctx context.Context, person store.Person, assignmentIDs []int64) {
	b.confirm(ctx, person, assignmentIDs, false)
}

/**
 * NOTE: SendLoginLink sends a person their single-use access link on WhatsApp.
 */
func (b *Bot) SendLoginLink(ctx context.Context, person store.Person, link string) error {
	if person.Phone == "" || b.Messenger == nil {
		return errors.New("essa pessoa não tem WhatsApp cadastrado")
	}
	key := "login-link:" + person.Phone
	if sent, ok := b.Store.NotifiedAt(ctx, key); ok && b.now().Sub(sent) < loginLinkGap {
		return ErrTooSoon
	}
	_ = b.Store.UnmarkNotified(ctx, key)
	if _, err := b.Store.MarkNotified(ctx, key, b.now()); err != nil {
		return err
	}
	if !b.send(ctx, person.Phone, loginLinkText(person.Name, link)) {
		_ = b.Store.UnmarkNotified(ctx, key)
		return errors.New("não consegui enviar a mensagem no WhatsApp")
	}
	return nil
}

func (b *Bot) Today() string {
	return b.now().Format("2006-01-02")
}

func (b *Bot) Upcoming(d store.AssignmentDetail) bool {
	return b.serviceTime(d).After(b.now())
}
