package notify

import (
	"context"
	"fmt"
	"time"

	"louvadeira/internal/store"
)

// NOTE: Two days ahead at 18h, so a Sunday service is announced on Friday evening, before a Saturday rehearsal.
const (
	remindDays = 2
	remindHour = 18
)

/**
 * NOTE: Reminds each service from 18h two days before, or as soon as the server is back if it was down then, until
 * the service starts. A ministry with a group gets one post there marking who serves, so the team is reminded and
 * everyone else knows who plays; a ministry without a group reminds each person who serves in private.
 */
func (b *Bot) remindServices(ctx context.Context, upcoming []store.AssignmentDetail, now time.Time) {
	b.remindInPrivate(ctx, upcoming, now)
	b.remindGroups(ctx, now)
}

func (b *Bot) remindDue(date, startTime string, now time.Time) bool {
	start := b.at(date, startTime)
	due := time.Date(start.Year(), start.Month(), start.Day()-remindDays, remindHour, 0, 0, 0, start.Location())
	return !now.Before(due) && now.Before(start)
}

func (b *Bot) remindInPrivate(ctx context.Context, upcoming []store.AssignmentDetail, now time.Time) {
	type key struct {
		person  int64
		date    string
		service int64
	}
	byKey := map[key][]store.AssignmentDetail{}
	var order []key
	for _, d := range upcoming {
		if d.WhatsAppGroup != "" || d.Status == store.AssignmentDeclined || !b.remindDue(d.Date, d.StartTime, now) {
			continue
		}
		k := key{d.PersonID, d.Date, d.ServiceID}
		if _, ok := byKey[k]; !ok {
			order = append(order, k)
		}
		byKey[k] = append(byKey[k], d)
	}
	for _, k := range order {
		slots := byKey[k]
		d := slots[0]
		if !b.allows(d.Phone) {
			continue
		}
		roles := make([]string, len(slots))
		for i, slot := range slots {
			roles[i] = slot.RoleName
		}
		b.once(ctx, fmt.Sprintf("remind:%d:%s:%d", k.person, k.date, k.service), func(ctx context.Context) bool {
			return b.send(ctx, d.Phone, reminderText(d, roles))
		})
	}
}

func (b *Bot) remindGroups(ctx context.Context, now time.Time) {
	ministries, err := b.Store.ListMinistries(ctx)
	if err != nil {
		b.Log.Error("listar ministérios", "erro", err)
		return
	}
	for _, m := range ministries {
		if !b.allows(m.WhatsAppGroup) {
			continue
		}
		ministry, err := b.Store.Ministry(ctx, m.ID)
		if err != nil {
			b.Log.Error("ler ministério para lembrar o culto", "erro", err)
			continue
		}
		for _, sv := range b.servicesAhead(ctx, m.ID, now) {
			if !b.remindDue(sv.Date, sv.StartTime, now) {
				continue
			}
			text, marked := lineupText(ministry, sv)
			b.once(ctx, fmt.Sprintf("lineup:%d:%s:%d", m.ID, sv.Date, sv.ServiceID), func(ctx context.Context) bool {
				return b.mention(ctx, m.WhatsAppGroup, text, marked...)
			})
		}
	}
}

/**
 * NOTE: The published services from today to the reminder horizon. A week belongs to the month of its first service,
 * so a day early in a month may sit in the month before's schedule, and the horizon may reach into next month.
 */
func (b *Bot) servicesAhead(ctx context.Context, ministryID int64, now time.Time) []store.ScheduleService {
	from := now.Format("2006-01-02")
	to := now.AddDate(0, 0, remindDays).Format("2006-01-02")
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	var list []store.ScheduleService
	for _, month := range []time.Time{first.AddDate(0, -1, 0), first, first.AddDate(0, 1, 0)} {
		view, err := b.Store.ScheduleView(ctx, ministryID, month.Format("2006-01"))
		if err != nil || view.Status != "published" {
			continue
		}
		for _, week := range view.Weeks {
			for _, sv := range week.Services {
				if sv.Date >= from && sv.Date <= to {
					list = append(list, sv)
				}
			}
		}
	}
	return list
}
