package messenger

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"
)

/**
 * NOTE: Paced leaves a random pause between consecutive messages, so a burst (the whole team's confirmations at 8h)
 * does not look like spam to WhatsApp. Sends go out one at a time; callers wait for their turn.
 */
func Paced(next Messenger, min, max time.Duration) Messenger {
	return &paced{next: next, min: min, max: max}
}

type paced struct {
	next     Messenger
	min, max time.Duration

	mu   sync.Mutex
	last time.Time
}

func (p *paced) SendText(ctx context.Context, to Address, text string) (MessageID, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.wait(ctx); err != nil {
		return "", err
	}
	defer p.sent()
	return p.next.SendText(ctx, to, text)
}

func (p *paced) SendPoll(ctx context.Context, to Address, poll Poll) (MessageID, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.wait(ctx); err != nil {
		return "", err
	}
	defer p.sent()
	return p.next.SendPoll(ctx, to, poll)
}

func (p *paced) sent() {
	p.last = time.Now()
}

func (p *paced) wait(ctx context.Context) error {
	if p.last.IsZero() {
		return nil
	}
	gap := p.min
	if p.max > p.min {
		gap += rand.N(p.max - p.min)
	}
	delay := time.Until(p.last.Add(gap))
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
