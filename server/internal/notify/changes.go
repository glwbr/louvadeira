package notify

import (
	"context"

	"louvadeira/internal/store"
)

/**
 * NOTE: AnnounceChange tells the people a leader's change to a published schedule touches, the same way a swap made
 * through the bot does: the person who left, the group (or the other leaders when the ministry has no group) and
 * whoever was still invited to a swap the change settled. The person who comes in is asked by the scheduled round,
 * which covers their whole week; when that round already asked them about the week, they are asked now about this
 * service alone. Actor is the leader who made the change, who needs no news of it.
 */
func (b *Bot) AnnounceChange(ctx context.Context, change store.AssignmentChange, actor int64) {
	d := change.After
	if !change.Published || !b.serviceTime(d).After(b.now()) {
		return
	}
	before := change.Before
	line := slotLine(d)
	var after *store.Person
	if d.PersonID != 0 {
		after = &store.Person{ID: d.PersonID, Name: d.PersonName, Phone: d.Phone}
	}
	// NOTE: Whoever was still invited hears the swap is over, whatever the change: the slot taken by someone else,
	// given back to the person who had stepped out, or emptied.
	for _, p := range change.Waiting {
		switch {
		case after != nil && after.ID == p.ID:
		case after != nil && (before == nil || before.ID != after.ID):
			b.send(ctx, p.Phone, alreadyTakenReply(p.Name, after.Name, []string{line}))
		default:
			b.send(ctx, p.Phone, swapSettledReply(p.Name, d))
		}
	}
	if before != nil && before.ID == d.PersonID {
		return
	}

	if before != nil {
		switch {
		case after == nil:
			b.send(ctx, before.Phone, removedNotice(before.Name, []string{line}))
		case change.BeforeDeclined:
			b.send(ctx, before.Phone, replacedNotice(before.Name, after.Name, []string{line}))
		default:
			b.send(ctx, before.Phone, leaderSwapNotice(before.Name, after.Name, []string{line}))
		}
	}
	if after == nil {
		return
	}

	switch {
	case d.WhatsAppGroup != "" && before != nil:
		b.mention(ctx, d.WhatsAppGroup, swapAnnouncement(d.MinistryName, mentionOf(*after), mentionOf(*before), []string{line}), *after, *before)
	case d.WhatsAppGroup != "":
		b.mention(ctx, d.WhatsAppGroup, gapFilledAnnouncement(d.MinistryName, mentionOf(*after), line), *after)
	case before != nil:
		b.tellLeaders(ctx, d.MinistryID, swapAnnouncement(d.MinistryName, after.Name, before.Name, []string{line}), actor, before.ID, after.ID)
	default:
		b.tellLeaders(ctx, d.MinistryID, gapFilledAnnouncement(d.MinistryName, after.Name, line), actor, after.ID)
	}

	if b.now().Before(b.confirmOpens(d)) {
		return
	}
	if _, asked := b.Store.NotifiedAt(ctx, confirmKey(d.MinistryID, d.PersonID, store.GroupKey(d))); asked {
		b.askConfirmation(ctx, []store.AssignmentDetail{d})
	}
}
