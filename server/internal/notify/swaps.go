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
	if from > u.EndDate {
		return nil, nil
	}
	slots, err := b.Store.PersonAssignments(ctx, u.PersonID, from, u.EndDate)
	if err != nil {
		return nil, err
	}
	var future []store.AssignmentDetail
	for _, d := range slots {
		if b.serviceTime(d).After(now) {
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
	return b.decline(ctx, ids, false, absence{period: periodText(u.StartDate, u.EndDate), reason: reasonLabel(u.Reason)})
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

func (b *Bot) inviteCovers(ctx context.Context, swaps []*swap, why absence) {
	type invitee struct{ ministry, person int64 }
	offers := map[invitee][]*swap{}
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
			if _, ok := offers[k]; !ok {
				order = append(order, k)
				people[k] = c
			}
			offers[k] = append(offers[k], s)
		}
	}
	for _, k := range order {
		list, candidate := offers[k], people[k]
		for start := 0; start < len(list); start += maxPollSlots {
			chunk := list[start:min(start+maxPollSlots, len(list))]
			if !b.sendInvite(ctx, candidate, chunk, why) {
				continue
			}
			for _, s := range chunk {
				if err := b.Store.AddCoverInvite(ctx, s.request, candidate.ID); err != nil {
					b.Log.Error("registrar convite", "erro", err)
					continue
				}
				s.invited = append(s.invited, candidate.Name)
			}
		}
	}
}

func (b *Bot) sendInvite(ctx context.Context, candidate store.Person, offers []*swap, why absence) bool {
	first := offers[0].detail
	if len(offers) == 1 && why.period == "" {
		options := []store.PollOption{
			{Label: optionCanCover, Action: actionAccept, Request: offers[0].request},
			{Label: optionCannotCover, Action: actionRefuse, Request: offers[0].request},
		}
		return b.sendPoll(ctx, store.PollCover, candidate, coverQuestion(candidate.Name, first), options, false)
	}

	question := coverManyQuestion(candidate.Name, first, why.period)
	labels := offerLabels(offers)
	options := make([]store.PollOption, 0, len(offers)+1)
	all := make([]int64, len(offers))
	for i, s := range offers {
		options = append(options, store.PollOption{Label: coverOption(labels[i]), Action: actionAccept, Request: s.request})
		all[i] = s.request
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
 * the slots differ in role, so the poll stays short.
 */
func offerLabels(offers []*swap) []string {
	days := map[string]int{}
	roles := map[string]bool{}
	for _, s := range offers {
		days[s.detail.Date]++
		roles[s.detail.RoleName] = true
	}
	labels := make([]string, len(offers))
	for i, s := range offers {
		label := weekday(s.detail.Date) + ", " + dayMonth(s.detail.Date)
		if days[s.detail.Date] > 1 {
			label += ", às " + hour(s.detail.StartTime)
		}
		if len(roles) > 1 {
			label += " (" + s.detail.RoleName + ")"
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
		b.tellLeaders(ctx, ministry, alert)
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
	var accepts, refusals []int64
	for _, o := range options {
		switch {
		case o.Action == actionAccept && picked[o.Label]:
			accepts = append(accepts, o.Request)
		case o.Action == actionAccept && slots > 1:
			refusals = append(refusals, o.Request)
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

func (b *Bot) acceptCovers(ctx context.Context, substitute store.Person, requestIDs []int64) {
	if len(requestIDs) == 0 {
		return
	}
	var won, gone []store.CoverResult
	for _, id := range requestIDs {
		result, err := b.Store.AcceptCover(ctx, id, substitute.ID, b.now())
		switch {
		case errors.Is(err, store.ErrCoverClosed):
			// NOTE: A multiple-choice vote arrives again with the slots this person already took; those stay quiet.
			if r, err := b.Store.CoverRequestResult(ctx, id); err == nil && r.Assignment.PersonID != substitute.ID {
				gone = append(gone, r)
			}
		case err != nil:
			b.Log.Error("aceitar troca", "erro", err)
		default:
			won = append(won, result)
		}
	}

	if len(gone) > 0 {
		b.send(ctx, substitute.Phone, goneReply(len(requestIDs) == 1, slotTexts(gone)))
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
		swap := swapAnnouncement(d.MinistryName, substitute.Name, absent.Name, slotTexts(list))
		b.tellLeaders(ctx, d.MinistryID, swap)
		if d.WhatsAppGroup != "" {
			b.send(ctx, d.WhatsAppGroup, swap)
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
		b.tellLeaders(ctx, k.ministry, uncoveredAlert(list[0].Assignment, list[0].Absent.Name, slotTexts(list)))
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
