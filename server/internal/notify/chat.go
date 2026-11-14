package notify

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"louvadeira/internal/messenger"
	"louvadeira/internal/store"
)

/**
 * CONSTRAINT: Texts that arrive more than two hours late (the server was offline) get no answer: "não posso amanhã"
 * may no longer mean the same day, and an answer hours later surprises more than it helps.
 */
const staleText = 2 * time.Hour

const (
	actionAbsence = "absence"
	actionKeep    = "keep"
	chatAhead     = 28 * 24 * time.Hour
)

/**
 * NOTE: HandleText answers the commands a team member sends in private. Messages from people outside the team, and
 * anything that is not a command, get no answer: the number also carries ordinary church conversations.
 */
func (b *Bot) HandleText(ctx context.Context, msg messenger.Incoming) {
	sent := b.now()
	if !msg.Time.IsZero() {
		if b.now().Sub(msg.Time) > staleText {
			return
		}
		sent = msg.Time
		if b.Location != nil {
			sent = sent.In(b.Location)
		}
	}
	if yes, ok := parseReply(msg.Text); ok {
		person, err := b.Store.PersonByPhone(ctx, string(msg.From))
		if err != nil {
			return
		}
		if b.answerByText(ctx, person, yes) {
			b.Log.Info("resposta por texto", "pessoa", person.Name, "sim", yes)
			return
		}
	}
	cmd, ok := parseCommand(msg.Text, sent)
	if !ok {
		return
	}
	person, err := b.Store.PersonByPhone(ctx, string(msg.From))
	if err != nil {
		return
	}
	b.Log.Info("comando recebido", "pessoa", person.Name, "comando", string(cmd.kind))
	switch cmd.kind {
	case commandHelp:
		b.send(ctx, person.Phone, helpText(person.Name))
	case commandSchedule:
		b.sendSchedule(ctx, person)
	case commandCannot:
		if len(cmd.ranges) == 0 {
			b.askWhichService(ctx, person)
		} else {
			b.askAbsence(ctx, person, cmd.ranges)
		}
	}
}

/**
 * NOTE: One service a member is in, with every role they hold in it.
 */
type memberService struct {
	date, startTime string
	ministryID      int64
	ministry        string
	communion       bool
	roles           []string
	slots           []store.MemberSlot
}

func (s memberService) open() []int64 {
	var ids []int64
	for _, slot := range s.slots {
		if slot.Status != store.AssignmentDeclined {
			ids = append(ids, slot.AssignmentID)
		}
	}
	return ids
}

/**
 * NOTE: "falta confirmar" only shows inside the week the confirmation is asked for; further services carry no
 * status, since nobody was asked about them yet.
 */
func (s memberService) state(asking bool) string {
	declined, confirmed, searching := true, true, false
	for _, slot := range s.slots {
		declined = declined && slot.Status == store.AssignmentDeclined
		confirmed = confirmed && slot.Status == store.AssignmentConfirmed
		searching = searching || slot.CoverOpen
	}
	switch {
	case declined && searching:
		return " · 🔄 procurando substituto"
	case declined:
		return " · 😕 você avisou que não vai"
	case confirmed:
		return " ✅"
	case asking:
		return " · ⏳ falta confirmar"
	}
	return ""
}

func (b *Bot) memberServices(ctx context.Context, person store.Person, from, to string) ([]memberService, error) {
	slots, err := b.Store.MemberSlots(ctx, person.ID, from, to)
	if err != nil {
		return nil, err
	}
	var services []memberService
	for _, slot := range slots {
		if !b.serviceTime(store.AssignmentDetail{Date: slot.Date, StartTime: slot.StartTime}).After(b.now()) {
			continue
		}
		if n := len(services); n > 0 {
			last := &services[n-1]
			if last.date == slot.Date && last.startTime == slot.StartTime && last.ministryID == slot.MinistryID {
				last.roles = append(last.roles, slot.RoleName)
				last.slots = append(last.slots, slot)
				continue
			}
		}
		services = append(services, memberService{date: slot.Date, startTime: slot.StartTime, ministryID: slot.MinistryID,
			ministry: slot.MinistryName, communion: slot.Communion, roles: []string{slot.RoleName}, slots: []store.MemberSlot{slot}})
	}
	return services, nil
}

func serviceLine(s memberService, withMinistry bool) string {
	line := serviceBullet(s.date, s.startTime, s.roles)
	if withMinistry {
		line += " (" + s.ministry + ")"
	}
	return line + communionMark(s.communion)
}

func manyMinistries(services []memberService) bool {
	for _, s := range services {
		if s.ministryID != services[0].ministryID {
			return true
		}
	}
	return false
}

func (b *Bot) upcoming(ctx context.Context, person store.Person) []memberService {
	services, err := b.memberServices(ctx, person, b.Today(), b.now().Add(chatAhead).Format("2006-01-02"))
	if err != nil {
		b.Log.Error("buscar escalas da pessoa", "erro", err)
	}
	return services
}

func (b *Bot) sendSchedule(ctx context.Context, person store.Person) {
	services := b.upcoming(ctx, person)
	if len(services) == 0 {
		b.send(ctx, person.Phone, noScheduleText(person.Name))
		return
	}
	lines := make([]string, len(services))
	for i, s := range services {
		asking := b.serviceTime(store.AssignmentDetail{Date: s.date, StartTime: s.startTime}).Sub(b.now()) <= confirmAhead
		lines[i] = serviceLine(s, manyMinistries(services)) + s.state(asking)
	}
	b.send(ctx, person.Phone, scheduleText(lines))
}

func (b *Bot) askWhichService(ctx context.Context, person store.Person) {
	var services []memberService
	for _, s := range b.upcoming(ctx, person) {
		if len(s.open()) > 0 {
			services = append(services, s)
		}
	}
	switch len(services) {
	case 0:
		b.send(ctx, person.Phone, noScheduleText(person.Name))
	case 1:
		options := []store.PollOption{
			{Label: optionYesNotGoing, Action: actionDecline, Assignments: services[0].open()},
			{Label: optionMistakeGoing, Action: actionKeep},
		}
		b.sendPoll(ctx, store.PollConfirm, person, singleServiceQuestion(person.Name, services[0]), options, false)
	default:
		services = services[:min(len(services), maxPollSlots)]
		days := map[string]int{}
		for _, s := range services {
			days[s.date]++
		}
		options := make([]store.PollOption, 0, len(services)+1)
		for _, s := range services {
			label := "😕 " + capitalize(weekday(s.date)) + ", " + dayMonth(s.date)
			if days[s.date] > 1 {
				label += ", às " + hour(s.startTime)
			}
			options = append(options, store.PollOption{Label: label, Action: actionDecline, Assignments: s.open()})
		}
		options = append(options, store.PollOption{Label: optionMistakeAll, Action: actionKeep})
		b.sendPoll(ctx, store.PollConfirm, person, whichServiceQuestion(person.Name), options, true)
	}
}

func rangesLabel(ranges []store.DateRange) string {
	var days, spans []string
	for _, r := range ranges {
		if r.From == r.To {
			days = append(days, dayMonth(r.From))
		} else {
			spans = append(spans, fmt.Sprintf("de %s a %s", dayMonth(r.From), dayMonth(r.To)))
		}
	}
	if len(days) > 0 {
		spans = append([]string{"em " + joinNames(days)}, spans...)
	}
	return joinNames(spans)
}

/**
 * NOTE: One period gets a yes/no question; several days get a poll where the person ticks only the days that are
 * right, so "19 e 22 sim, 15 não" needs no typing.
 */
func (b *Bot) askAbsence(ctx context.Context, person store.Person, ranges []store.DateRange) {
	ranges = slices.Clone(ranges)
	slices.SortFunc(ranges, func(a, b store.DateRange) int { return strings.Compare(a.From, b.From) })
	var lines []string
	for _, r := range ranges {
		services, err := b.memberServices(ctx, person, r.From, r.To)
		if err != nil {
			b.Log.Error("buscar escalas da pessoa", "erro", err)
			return
		}
		for _, s := range services {
			if len(s.open()) > 0 {
				lines = append(lines, serviceLine(s, false))
			}
		}
	}
	if len(ranges) == 1 {
		options := []store.PollOption{
			{Label: optionYesTell, Action: actionAbsence, Ranges: ranges},
			{Label: optionMistake, Action: actionKeep},
		}
		b.sendPoll(ctx, store.PollConfirm, person, absenceQuestion(person.Name, rangesLabel(ranges), lines), options, false)
		return
	}
	ranges = ranges[:min(len(ranges), maxPollSlots)]
	options := make([]store.PollOption, 0, len(ranges)+1)
	for _, r := range ranges {
		options = append(options, store.PollOption{Label: "😕 " + rangeOption(r), Action: actionAbsence, Ranges: []store.DateRange{r}})
	}
	options = append(options, store.PollOption{Label: optionMistake, Action: actionKeep})
	b.sendPoll(ctx, store.PollConfirm, person, absenceDaysQuestion(person.Name, lines), options, true)
}

func rangeOption(r store.DateRange) string {
	if r.From == r.To {
		return capitalize(weekday(r.From)) + ", " + dayMonth(r.From)
	}
	return fmt.Sprintf("De %s a %s", dayMonth(r.From), dayMonth(r.To))
}

func isChatPoll(options []store.PollOption) bool {
	return hasAction(options, actionKeep)
}

func hasAction(options []store.PollOption, action string) bool {
	for _, o := range options {
		if o.Action == action {
			return true
		}
	}
	return false
}

/**
 * INVARIANT: A poll the bot sent in reply to a command acts once. Unlike the weekly confirmation, changing the
 * answer later does not undo it; the reply says to talk to the leaders.
 */
func (b *Bot) answerChatPoll(ctx context.Context, person store.Person, pollID messenger.MessageID, chosen []store.PollOption) {
	key := "chat-answer:" + string(pollID)
	var declineIDs []int64
	var ranges []store.DateRange
	keep := false
	for _, o := range chosen {
		switch o.Action {
		case actionDecline:
			declineIDs = append(declineIDs, o.Assignments...)
		case actionAbsence:
			ranges = append(ranges, o.Ranges...)
		case actionKeep:
			keep = true
		}
	}
	if len(declineIDs) == 0 && len(ranges) == 0 {
		if !keep {
			return
		}
		if _, done := b.Store.NotifiedAt(ctx, key); done {
			b.send(ctx, person.Phone, alreadyToldReply())
		} else {
			b.send(ctx, person.Phone, keptReply())
		}
		return
	}
	if first, err := b.Store.MarkNotified(ctx, key, b.now()); err != nil || !first {
		return
	}
	if len(ranges) > 0 {
		b.registerAbsence(ctx, person, ranges)
		return
	}
	b.declineOwn(ctx, person, declineIDs)
}

func (b *Bot) declineOwn(ctx context.Context, person store.Person, assignmentIDs []int64) {
	var ids []int64
	for _, id := range assignmentIDs {
		d, err := b.Store.AssignmentDetail(ctx, id)
		if err != nil || d.PersonID != person.ID || d.Status == store.AssignmentDeclined {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}
	if _, err := b.Decline(ctx, ids, true); err != nil {
		b.Log.Error("registrar ausência", "erro", err)
	}
}

func (b *Bot) registerAbsence(ctx context.Context, person store.Person, ranges []store.DateRange) {
	label := rangesLabel(ranges)
	var ids []int64
	seen := map[int64]bool{}
	for _, r := range ranges {
		u, err := b.Store.CreateUnavailability(ctx, store.UnavailabilityInput{PersonID: person.ID, StartDate: r.From, EndDate: r.To, Reason: store.ReasonOther})
		if err != nil {
			b.Log.Error("registrar ausência", "erro", err)
			continue
		}
		slots, err := b.AffectedBy(ctx, u)
		if err != nil {
			b.Log.Error("buscar escalas afetadas", "erro", err)
			continue
		}
		for _, d := range slots {
			if !seen[d.AssignmentID] {
				seen[d.AssignmentID] = true
				ids = append(ids, d.AssignmentID)
			}
		}
	}
	if len(ids) == 0 {
		b.send(ctx, person.Phone, absenceNotedReply(person.Name, label))
		ministries, err := b.Store.PersonMinistries(ctx, person.ID)
		if err != nil {
			b.Log.Error("buscar ministérios da pessoa", "erro", err)
		}
		for _, id := range ministries {
			b.tellLeaders(ctx, id, absenceNotedAlert(person.Name, label), person.ID)
		}
		return
	}
	if _, err := b.decline(ctx, ids, true, absence{period: label}); err != nil {
		b.Log.Error("abrir trocas da ausência", "erro", err)
	}
}

// CONSTRAINT: A short "sim" or "não" only answers a poll sent in the last two days; older ones need a tap.
const replyWindow = 48 * time.Hour

var (
	yesReplies = map[string]bool{"sim": true, "vou": true, "vou sim": true, "sim vou": true, "confirmo": true, "confirmado": true,
		"confirmada": true, "estarei la": true, "vou estar la": true, "pode contar comigo": true, "to dentro": true,
		"consigo": true, "consigo sim": true, "posso": true, "posso sim": true}
	noReplies = map[string]bool{"nao": true, "nao vou": true, "nao posso": true, "nao consigo": true, "nao vou conseguir": true,
		"nao vou poder": true, "nao da": true, "nao vai dar": true, "infelizmente nao": true, "dessa vez nao": true}
)

/**
 * NOTE: Reads a short yes or no typed instead of tapping a poll option. Anything longer is not a reply.
 */
func parseReply(text string) (yes bool, ok bool) {
	t := normalizeText(text)
	switch {
	case yesReplies[t]:
		return true, true
	case noReplies[t]:
		return false, true
	}
	return false, false
}

/**
 * NOTE: Applies a typed yes or no to the person's latest weekly confirmation or invitation to cover, when the answer
 * is unambiguous; otherwise it points them to the poll. Reports whether the text was taken as a reply.
 */
func (b *Bot) answerByText(ctx context.Context, person store.Person, yes bool) bool {
	polls, err := b.Store.PersonRecentPolls(ctx, person.ID, b.now().Add(-replyWindow))
	if err != nil {
		b.Log.Error("buscar enquetes recentes", "erro", err)
		return false
	}
	for _, poll := range polls {
		switch {
		case poll.Kind == store.PollCover:
			b.answerCoverByText(ctx, person, poll, yes)
			return true
		case hasAction(poll.Options, actionConfirm):
			b.answerConfirmByText(ctx, person, poll, yes)
			return true
		}
	}
	return false
}

func (b *Bot) answerConfirmByText(ctx context.Context, person store.Person, poll store.Poll, yes bool) {
	var declines []store.PollOption
	for _, o := range poll.Options {
		switch {
		case yes && o.Action == actionConfirm:
			b.confirm(ctx, person, o.Assignments, true)
			return
		case o.Action == actionDecline:
			declines = append(declines, o)
		}
	}
	if !yes && len(declines) == 1 {
		b.declineOwn(ctx, person, declines[0].Assignments)
		return
	}
	b.send(ctx, person.Phone, pickInPollHint(false))
}

func (b *Bot) answerCoverByText(ctx context.Context, person store.Person, poll store.Poll, yes bool) {
	var accepts []store.PollOption
	var refuse *store.PollOption
	for i, o := range poll.Options {
		switch o.Action {
		case actionAccept:
			accepts = append(accepts, o)
		case actionRefuse:
			refuse = &poll.Options[i]
		}
	}
	switch {
	case yes && len(accepts) == 1:
		b.answerCover(ctx, person, poll.Options, accepts)
	case !yes && refuse != nil:
		b.answerCover(ctx, person, poll.Options, []store.PollOption{*refuse})
	default:
		b.send(ctx, person.Phone, pickInPollHint(true))
	}
}
