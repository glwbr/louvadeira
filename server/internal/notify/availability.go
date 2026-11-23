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
	actionUnavailable = "unavailable"
	actionAvailable   = "available"
	actionPublish     = "publish"
	actionEdit        = "edit"
	// NOTE: The monthly round: availability poll from day 20, a nudge two days later, the draft for approval a day after.
	availabilityDay = 20
	remindAfter     = 48 * time.Hour
	proposeAfter    = 72 * time.Hour
)

func roundKey(ministryID int64, month string) string {
	return fmt.Sprintf("availability-round:%d:%s", ministryID, month)
}

func askedKey(ministryID, personID int64, month string) string {
	return fmt.Sprintf("availability:%d:%d:%s", ministryID, personID, month)
}

func answeredKey(ministryID, personID int64, month string) string {
	return fmt.Sprintf("availability-answer:%d:%d:%s", ministryID, personID, month)
}

func nextMonth(now time.Time) string {
	return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01")
}

/**
 * NOTE: Runs the monthly round for every ministry whose next month is not published yet. Each step is remembered,
 * so a restart or a late start only catches up on what is missing. A round still open when its month arrives, because
 * the server was down or nobody approved it, carries on into that month.
 */
func (b *Bot) availabilityRound(ctx context.Context, now time.Time) {
	ministries, err := b.Store.ListMinistries(ctx)
	if err != nil {
		b.Log.Error("listar ministérios", "erro", err)
		return
	}
	thisMonth := now.Format("2006-01")
	for _, m := range ministries {
		b.runRound(ctx, m.ID, thisMonth, now, false)
		b.runRound(ctx, m.ID, nextMonth(now), now, now.Day() >= availabilityDay)
	}
}

func (b *Bot) runRound(ctx context.Context, ministryID int64, month string, now time.Time, canStart bool) {
	status, err := b.Store.ScheduleStatus(ctx, ministryID, month)
	if err != nil || status == "published" {
		return
	}
	started, ok := b.Store.NotifiedAt(ctx, roundKey(ministryID, month))
	if !ok {
		if !canStart {
			return
		}
		if _, err := b.Store.MarkNotified(ctx, roundKey(ministryID, month), now); err != nil {
			return
		}
		started = now
	}
	ministry, err := b.Store.Ministry(ctx, ministryID)
	if err != nil {
		return
	}
	if now.Sub(started) < proposeAfter {
		b.askAvailability(ctx, ministry, month)
	}
	if now.Sub(started) >= remindAfter {
		b.remindAvailability(ctx, ministry, month)
	}
	if now.Sub(started) >= proposeAfter {
		b.proposeSchedule(ctx, ministry, month, status)
	}
}

func (b *Bot) askAvailability(ctx context.Context, ministry store.MinistryDetail, month string) {
	occurrences, err := b.Store.MonthOccurrences(ctx, ministry.ID, month)
	if err != nil || len(occurrences) == 0 {
		return
	}
	days := map[string]int{}
	for _, o := range occurrences {
		days[o.Date]++
	}
	for _, member := range ministry.Members {
		if !b.allows(member.Phone) {
			continue
		}
		person := store.Person{ID: member.PersonID, Name: member.Name, Phone: member.Phone}
		// NOTE: Each part of a long poll is remembered on its own, so a part that failed is sent again alone.
		for start := 0; start < len(occurrences); start += maxPollSlots {
			chunk := occurrences[start:min(start+maxPollSlots, len(occurrences))]
			part := fmt.Sprintf("%s:%d", askedKey(ministry.ID, member.PersonID, month), start)
			options := make([]store.PollOption, 0, len(chunk)+1)
			for _, o := range chunk {
				label := capitalize(weekday(o.Date)) + ", " + dayMonth(o.Date)
				if days[o.Date] > 1 {
					label += ", às " + hour(o.StartTime)
				}
				options = append(options, store.PollOption{Label: label, Action: actionUnavailable, Ministry: ministry.ID, Month: month,
					Ranges: []store.DateRange{{From: o.Date, To: o.Date}}})
			}
			options = append(options, store.PollOption{Label: optionAvailableAll, Action: actionAvailable, Ministry: ministry.ID, Month: month})
			question := availabilityQuestion(member.Name, ministry.Name, month)
			if b.once(ctx, part, func(ctx context.Context) bool {
				return b.sendPoll(ctx, store.PollConfirm, person, question, options, true)
			}) {
				_, _ = b.Store.MarkNotified(ctx, askedKey(ministry.ID, member.PersonID, month), b.now())
			}
		}
	}
}

func (b *Bot) answerAvailability(ctx context.Context, person store.Person, pollID messenger.MessageID, options, chosen []store.PollOption) {
	var days, labels []string
	for _, o := range chosen {
		if o.Action == actionUnavailable && len(o.Ranges) == 1 {
			days = append(days, o.Ranges[0].From)
			labels = append(labels, weekday(o.Ranges[0].From)+", "+dayMonth(o.Ranges[0].From))
		}
	}
	changed, err := b.Store.ReplacePollAbsences(ctx, person.ID, "enquete:"+string(pollID), days)
	if err != nil {
		b.Log.Error("guardar disponibilidade", "erro", err)
		return
	}
	if !changed {
		return
	}
	first := options[0]
	_, _ = b.Store.MarkNotified(ctx, answeredKey(first.Ministry, person.ID, first.Month), b.now())
	b.send(ctx, person.Phone, availabilityReply(person.Name, labels))
}

func (b *Bot) remindAvailability(ctx context.Context, ministry store.MinistryDetail, month string) {
	for _, member := range ministry.Members {
		if _, asked := b.Store.NotifiedAt(ctx, askedKey(ministry.ID, member.PersonID, month)); !asked {
			continue
		}
		if _, answered := b.Store.NotifiedAt(ctx, answeredKey(ministry.ID, member.PersonID, month)); answered {
			continue
		}
		b.once(ctx, fmt.Sprintf("availability-remind:%d:%d:%s", ministry.ID, member.PersonID, month), func(ctx context.Context) bool {
			return b.send(ctx, member.Phone, availabilityReminder(member.Name, month))
		})
	}
}

/**
 * NOTE: Builds the draft once, unless the leaders already started one in the app, and asks each leader to approve it
 * until they got it. A draft they started is kept as it is, so their edits are never thrown away. The summary goes
 * before the poll, so a leader never gets the question without the schedule it is about.
 */
func (b *Bot) proposeSchedule(ctx context.Context, ministry store.MinistryDetail, month, status string) {
	built := fmt.Sprintf("availability-draft:%d:%s", ministry.ID, month)
	first, err := b.Store.MarkNotified(ctx, built, b.now())
	if err != nil {
		return
	}
	if status == "" {
		if !first {
			return
		}
		if err := b.Store.GenerateDraft(ctx, ministry.ID, month, b.now()); err != nil {
			b.tellLeaders(ctx, ministry.ID, draftFailedAlert(ministry.Name, month, err))
			return
		}
	}
	leaders, err := b.Store.Leaders(ctx, ministry.ID)
	if err != nil {
		b.Log.Error("buscar líderes", "erro", err)
		return
	}
	options := []store.PollOption{
		{Label: optionPublish, Action: actionPublish, Ministry: ministry.ID, Month: month},
		{Label: optionEdit, Action: actionEdit, Ministry: ministry.ID, Month: month},
	}
	for _, leader := range leaders {
		summary := fmt.Sprintf("%s:%d:summary", built, leader.ID)
		b.once(ctx, summary, func(ctx context.Context) bool {
			text, ok := b.proposalText(ctx, ministry, month)
			return ok && b.send(ctx, leader.Phone, text)
		})
		if _, sent := b.Store.NotifiedAt(ctx, summary); !sent {
			continue
		}
		b.once(ctx, fmt.Sprintf("%s:%d:poll", built, leader.ID), func(ctx context.Context) bool {
			return b.sendPoll(ctx, store.PollConfirm, leader, approvalQuestion(month), options, false)
		})
	}
}

func (b *Bot) proposalText(ctx context.Context, ministry store.MinistryDetail, month string) (string, bool) {
	view, err := b.Store.ScheduleView(ctx, ministry.ID, month)
	if err != nil {
		b.Log.Error("ler escala proposta", "erro", err)
		return "", false
	}
	asked, answered := 0, 0
	for _, member := range ministry.Members {
		if _, ok := b.Store.NotifiedAt(ctx, askedKey(ministry.ID, member.PersonID, month)); ok {
			asked++
		}
		if _, ok := b.Store.NotifiedAt(ctx, answeredKey(ministry.ID, member.PersonID, month)); ok {
			answered++
		}
	}
	return draftProposal(ministry.Name, month, Summary(ministry, view), answered, asked, view.Warnings), true
}

func (b *Bot) answerApproval(ctx context.Context, leader store.Person, pollID messenger.MessageID, option store.PollOption) {
	if first, err := b.Store.MarkNotified(ctx, fmt.Sprintf("approval:%s:%s", pollID, option.Action), b.now()); err != nil || !first {
		return
	}
	if option.Action == actionEdit {
		b.send(ctx, leader.Phone, editReply())
		return
	}
	err := b.Store.PublishSchedule(ctx, option.Ministry, option.Month, b.now())
	if errors.Is(err, store.ErrNotFound) {
		b.send(ctx, leader.Phone, alreadyPublishedReply(option.Month))
		return
	}
	if err != nil {
		b.Log.Error("publicar escala", "erro", err)
		return
	}
	b.send(ctx, leader.Phone, publishedReply(option.Month, b.announce(ctx, option.Ministry, option.Month)))
}
