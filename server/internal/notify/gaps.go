package notify

import (
	"context"
	"errors"
	"fmt"
	"time"

	"louvadeira/internal/messenger"
	"louvadeira/internal/store"
)

const (
	actionGapKeep = "gap-keep"
	actionGapAsk  = "gap-ask"
	actionGapEdit = "gap-edit"
	actionGapTake = "gap-take"
	actionGapPass = "gap-pass"
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
		b.once(ctx, gapKey(k), func(ctx context.Context) bool { return b.askAboutGaps(ctx, byService[k]) > 0 })
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
	keep, ask, edit := about, about, about
	keep.Label, keep.Action = optionGapKeep, actionGapKeep
	ask.Label, ask.Action = optionGapAsk, actionGapAsk
	edit.Label, edit.Action = optionEdit, actionGapEdit
	options := []store.PollOption{keep, edit}
	if b.allows(first.WhatsAppGroup) {
		options = []store.PollOption{keep, ask, edit}
	}
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
	case actionGapAsk:
		k := gapService{option.Ministry, option.Service, option.Date}
		if first, err := b.Store.MarkNotified(ctx, gapKey(k)+":asked", b.now()); err != nil || !first {
			b.send(ctx, leader.Phone, gapAlreadyAskedReply())
			return
		}
		b.send(ctx, leader.Phone, b.callGroup(ctx, k))
	case actionGapEdit:
		b.send(ctx, leader.Phone, gapEditReply())
	}
}

/**
 * NOTE: Posts a poll in the ministry's group with one option per role still short of people, and returns the reply
 * for the leader who asked for it.
 */
func (b *Bot) callGroup(ctx context.Context, k gapService) string {
	gaps, err := b.Store.ServiceGaps(ctx, k.date, k.date)
	if err != nil {
		b.Log.Error("buscar cultos com falta de gente", "erro", err)
		return gapCallFailedReply()
	}
	var open []store.ServiceGap
	for _, g := range gaps {
		if g.MinistryID == k.ministry && g.ServiceID == k.service && len(g.Free) > 0 {
			open = append(open, g)
		}
	}
	if len(open) == 0 {
		return gapClosedReply()
	}
	first := open[0]
	options := make([]store.PollOption, 0, len(open)+1)
	for _, g := range open {
		options = append(options, store.PollOption{Label: gapTakeOption(g.RoleName), Action: actionGapTake,
			Ministry: g.MinistryID, Service: g.ServiceID, Date: g.Date, Time: g.StartTime, Role: g.RoleID})
	}
	options = append(options, store.PollOption{Label: optionCannotCover, Action: actionGapPass})
	if !b.sendGroupPoll(ctx, first.WhatsAppGroup, gapCallQuestion(open), options) {
		return gapCallFailedReply()
	}
	return gapCalledReply(first.MinistryName)
}

func (b *Bot) sendGroupPoll(ctx context.Context, group, question string, options []store.PollOption) bool {
	if group == "" || b.Messenger == nil {
		return false
	}
	id, err := b.Messenger.SendPoll(ctx, messenger.Address(group), messenger.Poll{Question: question, Options: labelsOf(options)})
	if !b.delivered(group, err) {
		return false
	}
	if err := b.Store.SavePoll(ctx, store.Poll{MessageID: string(id), Kind: store.PollConfirm, Options: options}, b.now()); err != nil {
		b.Log.Error("guardar enquete", "erro", err)
	}
	return true
}

/**
 * INVARIANT: In the group poll only a volunteer's first answer counts: it may already have moved them into the
 * schedule and told the group, so changing it afterwards does nothing. Votes from the linked phone are ignored, since
 * the group poll belongs to nobody.
 */
func (b *Bot) answerGroupCall(ctx context.Context, vote messenger.Vote, option store.PollOption) {
	if vote.FromMe || option.Action != actionGapTake {
		return
	}
	volunteer, err := b.Store.PersonByPhone(ctx, string(vote.From))
	if err != nil {
		b.Log.Info("voto no grupo de quem não está cadastrado", "quem", vote.Sender)
		return
	}
	if first, err := b.Store.MarkNotified(ctx, fmt.Sprintf("gap-take:%s:%d", vote.PollID, volunteer.ID), b.now()); err != nil || !first {
		return
	}
	b.Log.Info("voluntário no grupo", "pessoa", volunteer.Name, "escolha", option.Label)
	filled, err := b.Store.FillGap(ctx, option.Ministry, option.Service, option.Date, option.Role, volunteer.ID, b.now())
	switch {
	case errors.Is(err, store.ErrNotMember):
		b.send(ctx, volunteer.Phone, outsiderReply(volunteer.Name))
		ministry, err := b.Store.Ministry(ctx, option.Ministry)
		if err == nil {
			b.tellLeaders(ctx, option.Ministry, outsiderAlert(volunteer.Name, ministry.Name, gapRoleName(option.Label), option.Date, option.Time))
		}
		return
	case errors.Is(err, store.ErrAlreadyServing):
		b.send(ctx, volunteer.Phone, alreadyServingReply([]string{whenAt(option.Date, option.Time)}))
		return
	case errors.Is(err, store.ErrAway):
		b.send(ctx, volunteer.Phone, awayVolunteerReply(volunteer.Name))
		return
	case errors.Is(err, store.ErrNothingMissing):
		b.send(ctx, volunteer.Phone, goneReply(true, nil))
		return
	case err != nil:
		b.Log.Error("colocar voluntário na escala", "erro", err)
		return
	}

	d := filled.Assignment
	line := slotLine(d)
	b.send(ctx, volunteer.Phone, coveredReply(volunteer.Name, []string{line}))
	if filled.Absent == nil {
		b.mention(ctx, d.WhatsAppGroup, gapFilledAnnouncement(d.MinistryName, mentionOf(volunteer), line), volunteer)
	} else {
		absent := *filled.Absent
		b.send(ctx, absent.Phone, replacedNotice(absent.Name, volunteer.Name, []string{line}))
		b.mention(ctx, d.WhatsAppGroup, swapAnnouncement(d.MinistryName, mentionOf(volunteer), mentionOf(absent), []string{line}), volunteer, absent)
	}
	for _, p := range filled.Waiting {
		if p.ID != volunteer.ID {
			b.send(ctx, p.Phone, alreadyTakenReply(p.Name, volunteer.Name, []string{line}))
		}
	}
}
