package messenger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

/**
 * NOTE: An Address is either a phone number in any human format ("+55 (11) 98888-7777") or a WhatsApp
 * chat ID that contains "@" (groups end in "@g.us"). Normalize turns both into one comparable form.
 */
type Address string

type MessageID string

type Messenger interface {
	SendText(ctx context.Context, to Address, text string) (MessageID, error)
	SendPoll(ctx context.Context, to Address, question string, options []string) (MessageID, error)
}

var ErrNotAllowed = errors.New("destino fora da lista de permissão do modo de teste")

func (a Address) Normalize() Address {
	raw := strings.TrimSpace(string(a))
	if strings.Contains(raw, "@") {
		return Address(strings.ToLower(raw))
	}
	var digits strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		}
	}
	return Address(digits.String())
}

func (a Address) IsGroup() bool {
	return strings.HasSuffix(string(a.Normalize()), "@g.us")
}

type allowlisted struct {
	next    Messenger
	allowed map[Address]struct{}
}

/**
 * INVARIANT: In test mode every send goes through this guard, so a bug elsewhere can only ever reach the
 * test group and the numbers listed here, never the owner's other contacts.
 */
func WithAllowlist(next Messenger, allowed []Address) Messenger {
	set := make(map[Address]struct{}, len(allowed))
	for _, address := range allowed {
		if normalized := address.Normalize(); normalized != "" {
			set[normalized] = struct{}{}
		}
	}
	return allowlisted{next: next, allowed: set}
}

func (g allowlisted) SendText(ctx context.Context, to Address, text string) (MessageID, error) {
	if err := g.check(to); err != nil {
		return "", err
	}
	return g.next.SendText(ctx, to, text)
}

func (g allowlisted) SendPoll(ctx context.Context, to Address, question string, options []string) (MessageID, error) {
	if err := g.check(to); err != nil {
		return "", err
	}
	return g.next.SendPoll(ctx, to, question, options)
}

func (g allowlisted) check(to Address) error {
	if _, ok := g.allowed[to.Normalize()]; !ok {
		return fmt.Errorf("%w: %s", ErrNotAllowed, to)
	}
	return nil
}
