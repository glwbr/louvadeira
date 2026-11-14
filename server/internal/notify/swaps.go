package notify

import (
	"context"
	"errors"
	"strings"

	"louvadeira/internal/store"
)

// CONSTRAINT: WhatsApp polls take at most 12 options and one is always "não posso em nenhum".
const maxPollSlots = 11

/**
 * NOTE: Why a person is stepping out. The zero value is a plain decline of the week's services; a registered absence
 * carries its period ("de 16/11 a 13/12") for everyone and its reason for the leaders only.
 */
type absence struct {
	period string
	reason string
}

type swap struct {
	detail  store.AssignmentDetail
	request int64
	late    bool
	invited []string
	blocked []string
}

/**
 * NOTE: Decline opens a swap for each slot. Everyone who can cover gets one invitation listing every slot they can
 * take, and the first to accept a slot takes it. Leaders always hear about it; under 72h it is flagged as a late
 * notice.
 */
func (b *Bot) Decline(ctx context.Context, assignmentIDs []int64, byMember bool) ([]store.CoverRequest, error) {
	return b.decline(ctx, assignmentIDs, byMember, absence{})
}

/**
 * NOTE: AffectedBy lists the slots a person still holds during an absence and that have not happened yet.
 */
func (b *Bot) AffectedBy(ctx context.Context, u store.Unavailability) ([]store.AssignmentDetail, error) {
	now := b.now()
	from := max(u.StartDate, now.Format("2006-01-02"))
	if from > u.Until() {
		return nil, nil
	}
	slots, err := b.Store.PersonAssignments(ctx, u.PersonID, from, u.Until())
	if err != nil {
		return nil, err
	}
	var future []store.AssignmentDetail
	for _, d := range slots {
		if u.Covers(d.Date) && b.serviceTime(d).After(now) {
			future = append(future, d)
		}
	}
	return future, nil
}

/**
 * NOTE: DeclinePeriod opens swaps for every slot the person still holds during a registered absence.
 */
func (b *Bot) DeclinePeriod(ctx context.Context, u store.Unavailability) ([]store.CoverRequest, error) {
	slots, err := b.AffectedBy(ctx, u)
	if err != nil || len(slots) == 0 {
		return []store.CoverRequest{}, err
	}
	ids := make([]int64, len(slots))
	for i, d := range slots {
		ids[i] = d.AssignmentID
	}
	return b.decline(ctx, ids, false, absence{period: absenceText(u), reason: reasonLabel(u.Reason)})
}

func (b *Bot) decline(ctx context.Context, assignmentIDs []int64, byMember bool, why absence) ([]store.CoverRequest, error) {
	var swaps []*swap
	var failure error
	for _, id := range assignmentIDs {
		s, err := b.openSwap(ctx, id)
		if err != nil {
			failure = errors.Join(failure, err)
			continue
		}
		swaps = append(swaps, s)
	}
	if len(swaps) == 0 {
		return nil, failure
	}
	if failure != nil {
		b.Log.Error("abrir troca", "erro", failure)
	}

	b.later(ctx, func(ctx context.Context) {
		b.inviteCovers(ctx, swaps, why)
		for _, s := range swaps {
			if len(s.invited) == 0 {
				_ = b.Store.CloseCoverRequest(ctx, s.request, b.now())
			}
		}
		b.alertDeclined(ctx, swaps, why)
		if byMember {
			b.thankAbsent(ctx, swaps)
		}
	})

	requests := make([]store.CoverRequest, 0, len(swaps))
	for _, s := range swaps {
		result, err := b.Store.CoverRequestResult(ctx, s.request)
		if err != nil {
			return requests, err
		}
		requests = append(requests, result.Request)
	}
	return requests, nil
}

func (b *Bot) openSwap(ctx context.Context, assignmentID int64) (*swap, error) {
	d, err := b.Store.AssignmentDetail(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	late := b.serviceTime(d).Sub(b.now()) < lateNotice
	request, err := b.Store.OpenCoverRequest(ctx, assignmentID, late, b.now())
	if err != nil {
		return nil, err
	}
	return &swap{detail: d, request: request, late: late}, nil
}

/**
 * NOTE: The swaps of one service and role, offered together: when two singers of the same Thursday are away, a
 * free singer gets one "Thursday" option and takes whichever place is still open.
 */
type offer struct {
	swaps []*swap
}

func (o offer) first() store.AssignmentDetail {
	return o.swaps[0].detail
}

func (o offer) requests() []int64 {
	ids := make([]int64, len(o.swaps))
	for i, s := range o.swaps {
		ids[i] = s.request
	}
	return ids
}

func (o offer) absentNames() []string {
	var names []string
	for _, s := range o.swaps {
		if !contains(names, s.detail.PersonName) {
			names = append(names, s.detail.PersonName)
		}
	}
	return names
}

func sameSlot(a, b store.AssignmentDetail) bool {
	return a.ServiceID == b.ServiceID && a.Date == b.Date && a.RoleID == b.RoleID
}

func groupOffers(swaps []*swap) []offer {
	var offers []offer
	for _, s := range swaps {
		joined := false
		for i := range offers {
			if sameSlot(offers[i].first(), s.detail) {
				offers[i].swaps = append(offers[i].swaps, s)
				joined = true
				break
			}
		}
		if !joined {
			offers = append(offers, offer{swaps: []*swap{s}})
		}
	}
	return offers
}

func (b *Bot) inviteCovers(ctx context.Context, swaps []*swap, why absence) {
	type invitee struct{ ministry, person int64 }
	pending := map[invitee][]*swap{}
	people := map[invitee]store.Person{}
	var order []invitee
	for _, s := range swaps {
		candidates, err := b.Store.CoverCandidates(ctx, s.detail.AssignmentID)
		if err != nil {
			b.Log.Error("buscar quem pode cobrir", "erro", err)
			continue
		}
		for _, c := range candidates {
			if !b.allows(c.Phone) {
				s.blocked = append(s.blocked, c.Name)
				continue
			}
			k := invitee{s.detail.MinistryID, c.ID}
			if _, ok := pending[k]; !ok {
				order = append(order, k)
				people[k] = c
			}
			pending[k] = append(pending[k], s)
		}
	}
	for _, k := range order {
		candidate := people[k]
		var fresh []*swap
		for _, s := range pending[k] {
			if !b.joinInvitation(ctx, candidate, s) {
				fresh = append(fresh, s)
			}
		}
		offers := groupOffers(fresh)
		for start := 0; start < len(offers); start += maxPollSlots {
			chunk := offers[start:min(start+maxPollSlots, len(offers))]
			if !b.sendInvite(ctx, candidate, chunk, why) {
				continue
			}
			for _, o := range chunk {
				for _, s := range o.swaps {
					if err := b.Store.AddCoverInvite(ctx, s.request, candidate.ID); err != nil {
						b.Log.Error("registrar convite", "erro", err)
						continue
					}
					s.invited = append(s.invited, candidate.Name)
				}
			}
		}
	}
}

/**
 * NOTE: When the candidate already has an invitation for the same service and role, the new place joins that
 * invitation instead of becoming another message. Someone who already said no to that service is not asked again.
 */
func (b *Bot) joinInvitation(ctx context.Context, candidate store.Person, s *swap) bool {
	polls, err := b.Store.PersonCoverPolls(ctx, candidate.ID)
	if err != nil {
		return false
	}
	for _, poll := range polls {
		for i, option := range poll.Options {
			if option.Action != actionAccept {
				continue
			}
			ids := requestsOf(option)
			earlier, err := b.Store.CoverRequestResult(ctx, ids[0])
			if err != nil || !sameSlot(earlier.Assignment, s.detail) {
				continue
			}
			if err := b.Store.AddCoverInvite(ctx, s.request, candidate.ID); err != nil {
				return false
			}
			if b.Store.InviteAnswer(ctx, ids[0], candidate.ID) == "no" {
				_, _ = b.Store.RefuseCover(ctx, s.request, candidate.ID, b.now())
				return true
			}
			poll.Options[i].Requests = append(ids, s.request)
			poll.Options[i].Request = 0
			for j := range poll.Options {
				if poll.Options[j].Action == actionRefuse {
					poll.Options[j].Requests = append(requestsOf(poll.Options[j]), s.request)
					poll.Options[j].Request = 0
				}
			}
			if err := b.Store.UpdatePollOptions(ctx, poll.MessageID, poll.Options); err != nil {
				b.Log.Error("juntar convite", "erro", err)
				return false
			}
			s.invited = append(s.invited, candidate.Name)
			return true
		}
	}
	return false
}

func (b *Bot) sendInvite(ctx context.Context, candidate store.Person, offers []offer, why absence) bool {
	first := offers[0].first()
	absent := offers[0].absentNames()
	for _, o := range offers[1:] {
		for _, name := range o.absentNames() {
			if !contains(absent, name) {
				absent = append(absent, name)
			}
		}
	}
	if len(offers) == 1 && why.period == "" {
		ids := offers[0].requests()
		options := []store.PollOption{
			{Label: optionCanCover, Action: actionAccept, Requests: ids},
			{Label: optionCannotCover, Action: actionRefuse, Requests: ids},
		}
		return b.sendPoll(ctx, store.PollCover, candidate, coverQuestion(candidate.Name, joinNames(absent), first), options, false)
	}

	question := coverManyQuestion(candidate.Name, joinNames(absent), first, why.period)
	labels := offerLabels(offers)
	options := make([]store.PollOption, 0, len(offers)+1)
	var all []int64
	for i, o := range offers {
		options = append(options, store.PollOption{Label: coverOption(labels[i]), Action: actionAccept, Requests: o.requests()})
		all = append(all, o.requests()...)
	}
	none := optionCoverNone
	if len(offers) == 1 {
		none = optionCannotCover
	}
	options = append(options, store.PollOption{Label: none, Action: actionRefuse, Requests: all})
	return b.sendPoll(ctx, store.PollCover, candidate, question, options, len(offers) > 1)
}

/**
 * NOTE: Option labels name the day ("quinta 19/11"), adding the time only when a day repeats and the role only when
 * the places differ in role, so the poll stays short.
 */
func offerLabels(offers []offer) []string {
	days := map[string]int{}
	roles := map[string]bool{}
	for _, o := range offers {
		days[o.first().Date]++
		roles[o.first().RoleName] = true
	}
	labels := make([]string, len(offers))
	for i, o := range offers {
		d := o.first()
		label := weekday(d.Date) + ", " + dayMonth(d.Date)
		if days[d.Date] > 1 {
			label += ", às " + hour(d.StartTime)
		}
		if len(roles) > 1 {
			label += " (" + d.RoleName + ")"
		}
		labels[i] = label
	}
	return labels
}

func (b *Bot) alertDeclined(ctx context.Context, swaps []*swap, why absence) {
	ministries, byMinistry := groupBy(swaps, func(s *swap) int64 { return s.detail.MinistryID })
	for _, ministry := range ministries {
		list := byMinistry[ministry]
		first := list[0].detail
		var blocked []string
		for _, s := range list {
			for _, name := range s.blocked {
				if !contains(blocked, name) {
					blocked = append(blocked, name)
				}
			}
		}

		var alert string
		if len(list) == 1 && why.period == "" {
			alert = declinedAlert(first, list[0].late, list[0].invited)
		} else {
			lines := []string{declinedManyHeader(first, why)}
			for _, s := range list {
				lines = append(lines, declinedManyLine(s.detail, s.late, s.invited))
			}
			alert = strings.Join(lines, "\n")
		}
		if len(blocked) > 0 {
			alert += blockedNote(blocked)
		}
		b.tellLeaders(ctx, ministry, alert, first.PersonID)
	}
}

func (b *Bot) thankAbsent(ctx context.Context, swaps []*swap) {
	first := swaps[0].detail
	var invited []string
	for _, s := range swaps {
		for _, name := range s.invited {
			if !contains(invited, name) {
				invited = append(invited, name)
			}
		}
	}
	b.send(ctx, first.Phone, declinedReply(first.PersonName, invited))
}

/**
 * NOTE: In a poll listing several slots, a slot left unticked counts as "não posso" for it. A slot already taken stays
 * taken when it is unticked later; giving it back goes through the leaders.
 */
func (b *Bot) answerCover(ctx context.Context, person store.Person, options, chosen []store.PollOption) {
	picked := map[string]bool{}
	for _, o := range chosen {
		picked[o.Label] = true
	}
	slots := 0
	for _, o := range options {
		if o.Action == actionAccept {
			slots++
		}
	}
	var accepts [][]int64
	var refusals []int64
	for _, o := range options {
		switch {
		case o.Action == actionAccept && picked[o.Label]:
			accepts = append(accepts, requestsOf(o))
		case o.Action == actionAccept && slots > 1:
			refusals = append(refusals, requestsOf(o)...)
		case o.Action == actionRefuse && picked[o.Label]:
			refusals = append(refusals, requestsOf(o)...)
		}
	}
	b.acceptCovers(ctx, person, accepts)
	b.refuseCovers(ctx, person, refusals, len(accepts) == 0)
}

func requestsOf(o store.PollOption) []int64 {
	if len(o.Requests) > 0 {
		return o.Requests
	}
	return []int64{o.Request}
}

/**
 * NOTE: Each group holds the places of one option; the person takes the first one still open and only one, since
 * nobody serves twice in the same service.
 */
func (b *Bot) acceptCovers(ctx context.Context, substitute store.Person, groups [][]int64) {
	if len(groups) == 0 {
		return
	}
	var won, gone []store.CoverResult
	var busy []string
	for _, group := range groups {
		var last store.CoverResult
		settled := false
		for _, id := range group {
			result, err := b.Store.AcceptCover(ctx, id, substitute.ID, b.now())
			if err == nil {
				won = append(won, result)
				settled = true
				break
			}
			r, rerr := b.Store.CoverRequestResult(ctx, id)
			if rerr == nil && r.Assignment.PersonID == substitute.ID {
				// NOTE: A multiple-choice vote arrives again with the places this person already took; those stay quiet.
				settled = true
				break
			}
			if errors.Is(err, store.ErrAlreadyServing) {
				if rerr == nil {
					busy = append(busy, slotText(r.Assignment))
				}
				b.refuseCovers(ctx, substitute, group, false)
				settled = true
				break
			}
			if !errors.Is(err, store.ErrCoverClosed) {
				b.Log.Error("aceitar troca", "erro", err)
				settled = true
				break
			}
			last = r
		}
		if !settled && last.Request.ID != 0 {
			gone = append(gone, last)
		}
	}

	if len(gone) > 0 {
		b.send(ctx, substitute.Phone, goneReply(len(groups) == 1, slotTexts(gone)))
	}
	if len(busy) > 0 {
		b.send(ctx, substitute.Phone, alreadyServingReply(busy))
	}
	if len(won) == 0 {
		return
	}

	b.send(ctx, substitute.Phone, coveredReply(substitute.Name, slotTexts(won)))

	type handover struct{ ministry, absent int64 }
	handovers, byHandover := groupBy(won, func(r store.CoverResult) handover {
		return handover{r.Assignment.MinistryID, r.Absent.ID}
	})
	for _, k := range handovers {
		list := byHandover[k]
		absent, d := list[0].Absent, list[0].Assignment
		b.send(ctx, absent.Phone, replacedNotice(absent.Name, substitute.Name, slotTexts(list)))
		b.tellLeaders(ctx, d.MinistryID, swapAnnouncement(d.MinistryName, substitute.Name, absent.Name, slotTexts(list)), absent.ID, substitute.ID)
		if d.WhatsAppGroup != "" {
			text := swapAnnouncement(d.MinistryName, mentionOf(substitute), mentionOf(absent), slotTexts(list))
			b.mention(ctx, d.WhatsAppGroup, text, substitute, absent)
		}
	}

	var waiting []store.Person
	released := map[int64][]store.CoverResult{}
	for _, r := range won {
		for _, p := range r.Waiting {
			if _, ok := released[p.ID]; !ok {
				waiting = append(waiting, p)
			}
			released[p.ID] = append(released[p.ID], r)
		}
	}
	for _, p := range waiting {
		b.send(ctx, p.Phone, alreadyTakenReply(p.Name, substitute.Name, slotTexts(released[p.ID])))
	}
}

func (b *Bot) refuseCovers(ctx context.Context, person store.Person, requestIDs []int64, reply bool) {
	refused := false
	var uncovered []store.CoverResult
	for _, id := range requestIDs {
		everyone, err := b.Store.RefuseCover(ctx, id, person.ID, b.now())
		if errors.Is(err, store.ErrCoverClosed) {
			continue
		}
		if err != nil {
			b.Log.Error("recusar troca", "erro", err)
			continue
		}
		refused = true
		if !everyone {
			continue
		}
		_ = b.Store.CloseCoverRequest(ctx, id, b.now())
		if r, err := b.Store.CoverRequestResult(ctx, id); err == nil {
			uncovered = append(uncovered, r)
		}
	}
	if refused && reply {
		b.send(ctx, person.Phone, refusedReply())
	}

	type gap struct{ ministry, absent int64 }
	gaps, byGap := groupBy(uncovered, func(r store.CoverResult) gap { return gap{r.Assignment.MinistryID, r.Absent.ID} })
	for _, k := range gaps {
		list := byGap[k]
		b.tellLeaders(ctx, k.ministry, uncoveredAlert(list[0].Assignment, list[0].Absent.Name, slotTexts(list)), k.absent)
	}
}

func slotTexts(results []store.CoverResult) []string {
	texts := make([]string, len(results))
	for i, r := range results {
		texts[i] = slotText(r.Assignment)
	}
	return texts
}

func groupBy[T any, K comparable](items []T, key func(T) K) ([]K, map[K][]T) {
	var order []K
	groups := map[K][]T{}
	for _, item := range items {
		k := key(item)
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], item)
	}
	return order, groups
}
