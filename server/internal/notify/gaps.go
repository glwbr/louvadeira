package notify

import (
	"context"
	"fmt"
	"time"

	"louvadeira/internal/messenger"
	"louvadeira/internal/store"
)

const (
	actionGapKeep = "gap-keep"
	actionGapEdit = "gap-edit"
	// NOTE: The leaders hear about a service still short of people from 9h of the third day before it.
	gapDays = 3
)

type gapService struct {
	ministry int64
	service  int64
	date     string
}

func gapKey(k gapService) string {
	return fmt.Sprintf("gap:%d:%s:%d", k.ministry, k.date, k.service)
}

/**
 * NOTE: Asks the leaders what to do about each service that is still short of people three days before, once per
 * service. Every leader is asked, including one who is the reason for the gap. A role nobody in the team could fill
 * anymore is left out: there is nothing to decide about it.
 */
func (b *Bot) alertGaps(ctx context.Context, now time.Time) {
	gaps, err := b.Store.ServiceGaps(ctx, now.Format("2006-01-02"), now.AddDate(0, 0, gapDays).Format("2006-01-02"))
	if err != nil {
		b.Log.Error("buscar cultos com falta de gente", "erro", err)
		return
	}
	var open []store.ServiceGap
	for _, g := range gaps {
		if len(g.Free) > 0 && b.at(g.Date, g.StartTime).After(now) {
			open = append(open, g)
		}
	}
	services, byService := groupBy(open, func(g store.ServiceGap) gapService { return gapService{g.MinistryID, g.ServiceID, g.Date} })
	for _, k := range services {
		key := gapKey(k)
		if first, err := b.Store.MarkNotified(ctx, key, b.now()); err != nil || !first {
			continue
		}
		if b.askAboutGaps(ctx, byService[k]) == 0 {
			_ = b.Store.UnmarkNotified(ctx, key)
		}
	}
}

func (b *Bot) askAboutGaps(ctx context.Context, gaps []store.ServiceGap) int {
	first := gaps[0]
	leaders, err := b.Store.Leaders(ctx, first.MinistryID)
	if err != nil {
		b.Log.Error("buscar líderes", "erro", err)
		return 0
	}
	about := store.PollOption{Ministry: first.MinistryID, Service: first.ServiceID, Date: first.Date, Time: first.StartTime}
	keep, edit := about, about
	keep.Label, keep.Action = optionGapKeep, actionGapKeep
	edit.Label, edit.Action = optionEdit, actionGapEdit
	options := []store.PollOption{keep, edit}
	asked := 0
	for _, leader := range leaders {
		if b.sendPoll(ctx, store.PollConfirm, leader, gapQuestion(leader.Name, gaps), options, false) {
			asked++
		}
	}
	return asked
}

/**
 * NOTE: Each leader's answer gets one reply. Keeping the service as it is also tells the other leaders, once, so
 * nobody keeps looking for someone.
 */
func (b *Bot) answerGap(ctx context.Context, leader store.Person, pollID messenger.MessageID, option store.PollOption) {
	if first, err := b.Store.MarkNotified(ctx, fmt.Sprintf("gap-answer:%s:%s", pollID, option.Action), b.now()); err != nil || !first {
		return
	}
	switch option.Action {
	case actionGapKeep:
		b.send(ctx, leader.Phone, gapKeptReply(option.Date, option.Time))
		k := gapService{option.Ministry, option.Service, option.Date}
		if first, err := b.Store.MarkNotified(ctx, gapKey(k)+":kept", b.now()); err == nil && first {
			ministry, err := b.Store.Ministry(ctx, option.Ministry)
			if err == nil {
				b.tellLeaders(ctx, option.Ministry, gapKeptAlert(leader.Name, ministry.Name, option.Date, option.Time), leader.ID)
			}
		}
	case actionGapEdit:
		b.send(ctx, leader.Phone, gapEditReply())
	}
}
