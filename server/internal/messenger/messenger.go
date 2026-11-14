package messenger

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"louvadeira/internal/phone"
)

/**
 * NOTE: An Address is either a phone number in any human format ("+55 (11) 98888-7777") or a WhatsApp
 * chat ID that contains "@" (groups end in "@g.us"). Normalize turns both into one comparable form.
 */
type Address string

type MessageID string

/**
 * NOTE: A Poll lets people answer by tapping. With Multiple they can tick any number of options; otherwise one.
 */
type Poll struct {
	Question string
	Options  []string
	Multiple bool
}

type Messenger interface {
	SendText(ctx context.Context, to Address, text string) (MessageID, error)
	SendPoll(ctx context.Context, to Address, poll Poll) (MessageID, error)
}

var ErrNotAllowed = errors.New("destino fora da lista de permissão do modo de teste")

/**
 * NOTE: A Vote is a poll answer as it arrives: WhatsApp sends the SHA-256 of each chosen option, not its text.
 * FromMe marks votes cast from the linked phone itself.
 */
type Vote struct {
	PollID   MessageID
	Selected [][]byte
	FromMe   bool
	Sender   string
}

func OptionHash(label string) []byte {
	sum := sha256.Sum256([]byte(label))
	return sum[:]
}

/**
 * NOTE: Allows reports whether a messenger would deliver to the address; only the test-mode allowlist ever says no.
 */
func Allows(m Messenger, to Address) bool {
	if gate, ok := m.(interface{ allows(Address) bool }); ok {
		return gate.allows(to)
	}
	return m != nil
}

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
	return Address(phone.Canonical(digits.String()))
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

func (g allowlisted) SendPoll(ctx context.Context, to Address, poll Poll) (MessageID, error) {
	if err := g.check(to); err != nil {
		return "", err
	}
	return g.next.SendPoll(ctx, to, poll)
}

func (g allowlisted) allows(to Address) bool {
	_, ok := g.allowed[to.Normalize()]
	return ok
}

func (g allowlisted) check(to Address) error {
	if !g.allows(to) {
		return fmt.Errorf("%w: %s", ErrNotAllowed, to)
	}
	return nil
}
