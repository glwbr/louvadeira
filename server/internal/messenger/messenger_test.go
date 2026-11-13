package messenger_test

import (
	"context"
	"errors"
	"testing"

	"louvadeira/internal/messenger"
)

type recorder struct {
	sent []messenger.Address
}

func (r *recorder) SendText(_ context.Context, to messenger.Address, _ string) (messenger.MessageID, error) {
	r.sent = append(r.sent, to)
	return "text-id", nil
}

func (r *recorder) SendPoll(_ context.Context, to messenger.Address, _ string, _ []string) (messenger.MessageID, error) {
	r.sent = append(r.sent, to)
	return "poll-id", nil
}

func TestNormalize(t *testing.T) {
	cases := map[messenger.Address]messenger.Address{
		"+55 (11) 98888-7777":       "5511988887777",
		"5511988887777":             "5511988887777",
		"551188887777":              "5511988887777",
		" 120363000000000000@G.US ": "120363000000000000@g.us",
		"":                          "",
	}
	for in, want := range cases {
		if got := in.Normalize(); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAllowlistBlocksAddressesOutsideTheList(t *testing.T) {
	next := &recorder{}
	guarded := messenger.WithAllowlist(next, []messenger.Address{"120363000000000000@g.us", "+55 11 98888-7777"})

	_, err := guarded.SendText(context.Background(), "5511900001111", "oi")
	if !errors.Is(err, messenger.ErrNotAllowed) {
		t.Fatalf("SendText to an unlisted number: err = %v, want ErrNotAllowed", err)
	}
	_, err = guarded.SendPoll(context.Background(), "999@g.us", "Confirma?", []string{"Sim", "Não"})
	if !errors.Is(err, messenger.ErrNotAllowed) {
		t.Fatalf("SendPoll to an unlisted group: err = %v, want ErrNotAllowed", err)
	}
	if len(next.sent) != 0 {
		t.Fatalf("blocked sends reached the messenger: %v", next.sent)
	}
}

func TestAllowlistPassesListedAddressesInAnyFormat(t *testing.T) {
	next := &recorder{}
	guarded := messenger.WithAllowlist(next, []messenger.Address{"120363000000000000@g.us", "+55 11 98888-7777"})

	if _, err := guarded.SendText(context.Background(), "5511988887777", "oi"); err != nil {
		t.Fatalf("SendText to a listed number: %v", err)
	}
	if _, err := guarded.SendPoll(context.Background(), "120363000000000000@G.US", "Confirma?", []string{"Sim", "Não"}); err != nil {
		t.Fatalf("SendPoll to a listed group: %v", err)
	}
	if len(next.sent) != 2 {
		t.Fatalf("sent = %v, want two sends", next.sent)
	}
}

func TestEmptyAllowlistBlocksEverything(t *testing.T) {
	guarded := messenger.WithAllowlist(&recorder{}, nil)
	if _, err := guarded.SendText(context.Background(), "5511988887777", "oi"); !errors.Is(err, messenger.ErrNotAllowed) {
		t.Fatalf("err = %v, want ErrNotAllowed", err)
	}
}

func TestAllowsFollowsTheAllowlist(t *testing.T) {
	guarded := messenger.WithAllowlist(&recorder{}, []messenger.Address{"5511988887777"})
	if !messenger.Allows(guarded, "+55 11 98888-7777") || messenger.Allows(guarded, "5511900001111") {
		t.Fatal("Allows does not follow the allowlist")
	}
	if !messenger.Allows(&recorder{}, "5511900001111") {
		t.Fatal("an unguarded messenger should allow everything")
	}
	if messenger.Allows(nil, "5511900001111") {
		t.Fatal("a nil messenger delivers nothing")
	}
}
