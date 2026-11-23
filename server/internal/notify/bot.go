package notify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"louvadeira/internal/messenger"
	"louvadeira/internal/phone"
	"louvadeira/internal/store"
)

const (
	confirmDays    = 7
	confirmHour    = 18
	pendingDays    = 3
	answerTime     = 24 * time.Hour
	lateNotice     = 72 * time.Hour
	firstHour      = 8
	lastHour       = 21
	roundHour      = 10
	pendingHour    = 9
	actionConfirm  = "confirm"
	actionDecline  = "decline"
	actionAccept   = "accept"
	actionRefuse   = "refuse"
	actionTestPoll = "test"
)

/**
 * NOTE: Bot sends every WhatsApp message of the app and reacts to poll votes. Messenger may be nil (WhatsApp off):
 * the bot then still records swaps but sends nothing.
 */
type Bot struct {
	Store     *store.Store
	Messenger messenger.Messenger
	Log       *slog.Logger
	Location  *time.Location
	Now       func() time.Time
	OwnPhone  func() string
	// NOTE: Settle waits after a vote before acting, so someone changing their answer in a hurry only counts once.
	Settle time.Duration
	// NOTE: Async runs a batch of messages in the background, so a request from the app does not wait out the pauses
	// between messages. When nil the batch runs inline.
	Async func(task func())
	// NOTE: ClockSynced tells whether the system clock can be trusted; nil trusts it. A board without a clock battery
	// boots at a stale time, and a scheduled message sent then goes out at the wrong hour.
	ClockSynced func() bool

	clockWarned atomic.Bool

	mu      sync.Mutex
	pending map[pendingKey]*time.Timer
}

// NOTE: A vote waits per poll and voter: in a group poll, one person's answer never replaces another's.
type pendingKey struct {
	poll  messenger.MessageID
	voter messenger.Address
}

func (b *Bot) now() time.Time {
	t := time.Now()
	if b.Now != nil {
		t = b.Now()
	}
	if b.Location != nil {
		t = t.In(b.Location)
	}
	return t
}

/**
 * NOTE: InBackground runs a task after the request that started it has been answered, on a context the request's end
 * does not cancel.
 */
func (b *Bot) InBackground(ctx context.Context, task func(context.Context)) {
	b.later(ctx, task)
}

func (b *Bot) later(ctx context.Context, task func(context.Context)) {
	if b.Async == nil {
		task(ctx)
		return
	}
	detached := context.WithoutCancel(ctx)
	b.Async(func() { task(detached) })
}

func (b *Bot) serviceTime(d store.AssignmentDetail) time.Time {
	return b.at(d.Date, d.StartTime)
}

/**
 * NOTE: An answer can reach the server long after it was given, when the server was off. By then the service may have
 * happened, and calling substitutes or announcing a change for it would only confuse people.
 */
func (b *Bot) started(date, startTime string) bool {
	return !b.at(date, startTime).After(b.now())
}

func (b *Bot) upcomingSlots(ctx context.Context, assignmentIDs []int64) []int64 {
	var ids []int64
	for _, id := range assignmentIDs {
		if d, err := b.Store.AssignmentDetail(ctx, id); err == nil && !b.started(d.Date, d.StartTime) {
			ids = append(ids, id)
		}
	}
	return ids
}

func (b *Bot) at(date, startTime string) time.Time {
	loc := b.Location
	if loc == nil {
		loc = time.UTC
	}
	t, _ := time.ParseInLocation("2006-01-02 15:04", date+" "+startTime, loc)
	return t
}

func (b *Bot) send(ctx context.Context, to string, text string) bool {
	if to == "" || b.Messenger == nil {
		return false
	}
	_, err := b.Messenger.SendText(ctx, messenger.Address(to), text)
	return b.delivered(to, err)
}

/**
 * INVARIANT: once sends a scheduled message at most once per key. The key is claimed before the send, so two ticks
 * never both send it, and dropped when the send fails, so the next tick tries again. After shutdown begins no new
 * message starts, and the one already started finishes on a detached context: cut short, it might reach WhatsApp
 * with its claim dropped. A claim a power cut leaves behind is dropped at the next start (DropUnsentClaims).
 */
func (b *Bot) once(ctx context.Context, key string, send func(context.Context) bool) bool {
	if ctx.Err() != nil {
		return false
	}
	if first, err := b.Store.ClaimNotification(ctx, key, b.now()); err != nil || !first {
		return false
	}
	ctx = context.WithoutCancel(ctx)
	if !send(ctx) {
		_ = b.Store.UnmarkNotified(ctx, key)
		return false
	}
	if err := b.Store.SettleNotification(ctx, key); err != nil {
		b.Log.Error("registrar envio", "chave", key, "erro", err)
	}
	return true
}

/**
 * INVARIANT: A send WhatsApp did not confirm in time counts as delivered. Sending it again risks the person getting
 * it twice; if it really got lost, the leaders' "falta responder" alert still catches it.
 */
func (b *Bot) delivered(to string, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, messenger.ErrMaybeDelivered):
		b.Log.Warn("envio sem confirmação do WhatsApp; não vou repetir", "para", "…"+tail(to))
		return true
	}
	b.logSendError(to, err)
	return false
}

/**
 * NOTE: mention posts to a group marking people, so they are notified even with the group muted. People without a
 * phone are written by name and not marked.
 */
func (b *Bot) mention(ctx context.Context, group string, text string, people ...store.Person) bool {
	if b.Messenger == nil {
		return false
	}
	var marked []messenger.Address
	for _, p := range people {
		if p.Phone != "" {
			marked = append(marked, messenger.Address(p.Phone))
		}
	}
	_, err := b.Messenger.SendMention(ctx, messenger.Address(group), text, marked)
	return b.delivered(group, err)
}

func mentionOf(p store.Person) string {
	if p.Phone == "" {
		return p.Name
	}
	return "@" + p.Phone
}

func (b *Bot) sendPoll(ctx context.Context, kind string, person store.Person, question string, options []store.PollOption, multiple bool) bool {
	if person.Phone == "" || b.Messenger == nil {
		return false
	}
	id, err := b.Messenger.SendPoll(ctx, messenger.Address(person.Phone), messenger.Poll{Question: question, Options: labelsOf(options), Multiple: multiple})
	if !b.delivered(person.Phone, err) {
		return false
	}
	if err := b.Store.SavePoll(ctx, store.Poll{MessageID: string(id), Kind: kind, PersonID: person.ID, Options: options}, b.now()); err != nil {
		b.Log.Error("guardar enquete", "erro", err)
	}
	return true
}

func labelsOf(options []store.PollOption) []string {
	labels := make([]string, len(options))
	for i, option := range options {
		labels[i] = option.Label
	}
	return labels
}

func (b *Bot) logSendError(to string, err error) {
	if errors.Is(err, messenger.ErrNotAllowed) {
		b.Log.Info("envio bloqueado pelo modo de teste", "para", "…"+tail(to))
		return
	}
	b.Log.Error("falha ao enviar mensagem", "para", "…"+tail(to), "erro", err)
}

func tail(phone string) string {
	if len(phone) <= 4 {
		return phone
	}
	return phone[len(phone)-4:]
}

func (b *Bot) allows(to string) bool {
	return to != "" && messenger.Allows(b.Messenger, messenger.Address(to))
}

/**
 * NOTE: Sends to the ministry's leaders, except the people the news is about: a leader who is away already knows,
 * and a leader who covers already got their own reply.
 */
func (b *Bot) tellLeaders(ctx context.Context, ministryID int64, text string, about ...int64) int {
	leaders, err := b.Store.Leaders(ctx, ministryID)
	if err != nil {
		b.Log.Error("buscar líderes", "erro", err)
		return 0
	}
	told := 0
	for _, leader := range leaders {
		if !containsID(about, leader.ID) && b.send(ctx, leader.Phone, text) {
			told++
		}
	}
	return told
}

func (b *Bot) person(ctx context.Context, id int64) (store.Person, bool) {
	p, err := b.Store.Person(ctx, id)
	if err != nil {
		return store.Person{}, false
	}
	return p, true
}

/**
 * NOTE: Tick runs the scheduled messages. It only sends between 8h and 21h local time, and in test mode it skips
 * people outside the allowlist without recording them, so they still get the message once the app goes live.
 */
func (b *Bot) Tick(ctx context.Context) {
	if b.ClockSynced != nil && !b.ClockSynced() {
		if !b.clockWarned.Swap(true) {
			b.Log.Warn("relógio sem sincronizar; as mensagens agendadas esperam")
		}
		return
	}
	b.clockWarned.Store(false)
	now := b.now()
	if now.Hour() < firstHour || now.Hour() >= lastHour {
		return
	}
	today := now.Format("2006-01-02")
	// NOTE: Reads two weeks so a week that only starts inside the 7-day window is asked about as a whole.
	upcoming, err := b.Store.UpcomingAssignments(ctx, today, now.AddDate(0, 0, 2*confirmDays).Format("2006-01-02"))
	if err != nil {
		b.Log.Error("buscar escalas", "erro", err)
		return
	}
	var future []store.AssignmentDetail
	for _, d := range upcoming {
		if b.serviceTime(d).After(now) {
			future = append(future, d)
		}
	}
	b.retryInvites(ctx)
	b.retryAnnouncements(ctx, now)
	b.sendConfirmations(ctx, future, now)
	b.remindServices(ctx, future, now)
	if now.Hour() >= pendingHour {
		b.alertPending(ctx, future, now)
		b.alertGaps(ctx, now)
	}
	if now.Hour() >= roundHour {
		b.availabilityRound(ctx, now)
	}
}

type personGroup struct {
	ministryID int64
	personID   int64
	key        string
}

/**
 * NOTE: AskNow sends the confirmations for services in [from, to] right away, even outside the scheduled hours, to
 * whoever was not asked yet. It returns who was asked.
 */
func (b *Bot) AskNow(ctx context.Context, from, to string) ([]string, error) {
	now := b.now()
	upcoming, err := b.Store.UpcomingAssignments(ctx, from, to)
	if err != nil {
		return nil, err
	}
	var future []store.AssignmentDetail
	for _, d := range upcoming {
		if b.serviceTime(d).After(now) {
			future = append(future, d)
		}
	}
	return b.sendConfirmations(ctx, future, now), nil
}

func (b *Bot) sendConfirmations(ctx context.Context, upcoming []store.AssignmentDetail, now time.Time) []string {
	groups := map[personGroup][]store.AssignmentDetail{}
	var order []personGroup
	for _, d := range upcoming {
		k := personGroup{d.MinistryID, d.PersonID, store.GroupKey(d)}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], d)
	}
	asked := []string{}
	for _, k := range order {
		slots := groups[k]
		pending := false
		for _, d := range slots {
			pending = pending || d.Status == store.AssignmentScheduled
		}
		if !pending || !b.allows(slots[0].Phone) || now.Before(b.confirmOpens(slots[0])) {
			continue
		}
		key := confirmKey(k.ministryID, k.personID, k.key)
		if b.once(ctx, key, func(ctx context.Context) bool { return b.askConfirmation(ctx, slots) }) {
			asked = append(asked, slots[0].PersonName)
		}
	}
	return asked
}

/**
 * NOTE: A service is asked about from 18h of the same weekday a week before: people are off work by then, and a
 * service at its own start time would find them already in church.
 */
func (b *Bot) confirmOpens(d store.AssignmentDetail) time.Time {
	start := b.serviceTime(d)
	return time.Date(start.Year(), start.Month(), start.Day()-confirmDays, confirmHour, 0, 0, 0, start.Location())
}

func confirmKey(ministryID, personID int64, group string) string {
	return fmt.Sprintf("confirm:%d:%d:%s", ministryID, personID, group)
}

type occurrence struct {
	date        string
	startTime   string
	communion   bool
	roles       []string
	assignments []int64
}

func occurrencesOf(slots []store.AssignmentDetail) []occurrence {
	var list []occurrence
	for _, d := range slots {
		if n := len(list); n > 0 && list[n-1].date == d.Date && list[n-1].startTime == d.StartTime {
			list[n-1].roles = append(list[n-1].roles, d.RoleName)
			list[n-1].assignments = append(list[n-1].assignments, d.AssignmentID)
			continue
		}
		list = append(list, occurrence{date: d.Date, startTime: d.StartTime, communion: d.Communion, roles: []string{d.RoleName}, assignments: []int64{d.AssignmentID}})
	}
	return list
}

func (b *Bot) askConfirmation(ctx context.Context, slots []store.AssignmentDetail) bool {
	first := slots[0]
	occurrences := occurrencesOf(slots)
	var all []int64
	for _, o := range occurrences {
		all = append(all, o.assignments...)
	}
	question := confirmQuestion(first.PersonName, first.MinistryName, occurrences)

	options := []store.PollOption{{Label: goingOption(len(occurrences)), Action: actionConfirm, Assignments: all}}
	if len(occurrences) == 1 {
		options = append(options, store.PollOption{Label: optionNotGoing, Action: actionDecline, Assignments: all})
	} else {
		for _, o := range occurrences {
			options = append(options, store.PollOption{Label: notGoingOption(o.date), Action: actionDecline, Assignments: o.assignments})
		}
		options = append(options, store.PollOption{Label: optionNotGoingAny, Action: actionDecline, Assignments: all})
	}
	return b.sendPoll(ctx, store.PollConfirm, store.Person{ID: first.PersonID, Name: first.PersonName, Phone: first.Phone}, question, options, false)
}

func (b *Bot) alertPending(ctx context.Context, upcoming []store.AssignmentDetail, now time.Time) {
	type key struct {
		ministry int64
		date     string
		service  int64
	}
	pending := map[key][]string{}
	sample := map[key]store.AssignmentDetail{}
	var order []key
	for _, d := range upcoming {
		// NOTE: Counted in days, from 9h: a Sunday service is chased on Thursday morning, never at Thursday's service.
		if d.Status != store.AssignmentScheduled || d.Date > now.AddDate(0, 0, pendingDays).Format("2006-01-02") {
			continue
		}
		asked, ok := b.Store.NotifiedAt(ctx, confirmKey(d.MinistryID, d.PersonID, store.GroupKey(d)))
		if !ok || now.Sub(asked) < answerTime {
			continue
		}
		k := key{d.MinistryID, d.Date, d.ServiceID}
		if _, ok := pending[k]; !ok {
			order = append(order, k)
			sample[k] = d
		}
		if !contains(pending[k], d.PersonName) {
			pending[k] = append(pending[k], d.PersonName)
		}
	}
	for _, k := range order {
		text := pendingAlert(sample[k], pending[k])
		b.alertEachLeader(ctx, k.ministry, fmt.Sprintf("pending:%d:%s:%d", k.ministry, k.date, k.service), func(ctx context.Context, leader store.Person) bool {
			return b.send(ctx, leader.Phone, text)
		})
	}
}

/**
 * NOTE: A scheduled alert goes to each leader once, so a leader whose send failed still gets it on a later tick.
 */
func (b *Bot) alertEachLeader(ctx context.Context, ministryID int64, key string, send func(context.Context, store.Person) bool) {
	leaders, err := b.Store.Leaders(ctx, ministryID)
	if err != nil {
		b.Log.Error("buscar líderes", "erro", err)
		return
	}
	for _, leader := range leaders {
		b.once(ctx, fmt.Sprintf("%s:leader:%d", key, leader.ID), func(ctx context.Context) bool { return send(ctx, leader) })
	}
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

/**
 * NOTE: HandleVote applies a poll answer. A vote cast from the linked phone only counts on polls addressed to the
 * owner of that phone, so the leader browsing a member's chat cannot answer for them by accident.
 */
func (b *Bot) HandleVote(ctx context.Context, vote messenger.Vote) {
	if b.Settle <= 0 {
		b.applyVote(ctx, vote)
		return
	}
	waiting := store.PendingVote{PollID: string(vote.PollID), Voter: string(vote.From), Selected: vote.Selected, FromMe: vote.FromMe}
	if err := b.Store.SavePendingVote(ctx, waiting, b.now()); err != nil {
		b.Log.Error("guardar voto", "erro", err)
	}
	key := pendingKey{vote.PollID, vote.From}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending == nil {
		b.pending = map[pendingKey]*time.Timer{}
	}
	if timer, ok := b.pending[key]; ok {
		timer.Stop()
	}
	b.pending[key] = time.AfterFunc(b.Settle, func() {
		b.mu.Lock()
		delete(b.pending, key)
		b.mu.Unlock()
		ctx := context.Background()
		b.applyVote(ctx, vote)
		_ = b.Store.DeletePendingVote(ctx, waiting)
	})
}

/**
 * NOTE: Applies the votes that were still waiting when the server stopped. Every answer is safe to apply again, so a
 * vote that was also applied before the stop changes nothing and sends nothing.
 */
func (b *Bot) ReplayPendingVotes(ctx context.Context) {
	votes, err := b.Store.PendingVotes(ctx)
	if err != nil {
		b.Log.Error("ler votos pendentes", "erro", err)
		return
	}
	for _, v := range votes {
		b.applyVote(ctx, messenger.Vote{PollID: messenger.MessageID(v.PollID), Selected: v.Selected, FromMe: v.FromMe, From: messenger.Address(v.Voter)})
		_ = b.Store.DeletePendingVote(ctx, v)
	}
}

func (b *Bot) applyVote(ctx context.Context, vote messenger.Vote) {
	poll, err := b.Store.Poll(ctx, string(vote.PollID))
	if err != nil {
		return
	}
	var chosen []store.PollOption
	for _, option := range poll.Options {
		for _, hash := range vote.Selected {
			if bytes.Equal(hash, messenger.OptionHash(option.Label)) {
				chosen = append(chosen, option)
				break
			}
		}
	}
	choice := strings.Join(labelsOf(chosen), ", ")
	if poll.Kind == store.PollTest {
		if choice == "" {
			choice = "(voto retirado)"
		}
		b.Log.Info("voto na enquete de teste", "quem", vote.Sender, "escolha", choice, "do próprio celular", vote.FromMe)
		return
	}
	if len(chosen) == 0 {
		return
	}
	if hasAction(poll.Options, actionGapTake) {
		b.answerGroupCall(ctx, vote, chosen[len(chosen)-1])
		return
	}
	person, ok := b.person(ctx, poll.PersonID)
	if !ok {
		return
	}
	if vote.FromMe && (b.OwnPhone == nil || phone.Canonical(b.OwnPhone()) != person.Phone) {
		return
	}
	b.Log.Info("voto recebido", "enquete", poll.Kind, "pessoa", person.Name, "escolha", choice)

	switch {
	case poll.Kind == store.PollCover:
		b.answerCover(ctx, person, poll.Options, chosen)
		return
	case isChatPoll(poll.Options):
		b.answerChatPoll(ctx, person, vote.PollID, chosen)
		return
	case hasAction(poll.Options, actionAvailable):
		b.answerAvailability(ctx, person, vote.PollID, poll.Options, chosen)
		return
	case hasAction(poll.Options, actionPublish):
		b.answerApproval(ctx, person, vote.PollID, chosen[len(chosen)-1])
		return
	case hasAction(poll.Options, actionGapKeep):
		b.answerGap(ctx, person, vote.PollID, chosen[len(chosen)-1])
		return
	}
	option := chosen[len(chosen)-1]
	switch option.Action {
	case actionConfirm:
		for _, id := range option.Assignments {
			_ = b.Store.UnmarkNotified(ctx, pollDeclineKey(vote.PollID, id))
		}
		b.confirm(ctx, person, option.Assignments, true)
	case actionDecline:
		b.declineOwn(ctx, person, option.Assignments)
		for _, id := range option.Assignments {
			_, _ = b.Store.MarkNotified(ctx, pollDeclineKey(vote.PollID, id), b.now())
		}
		b.goingOnOtherDays(ctx, person, vote.PollID, otherSlots(poll.Options, option.Assignments))
	}
}

func pollDeclineKey(pollID messenger.MessageID, assignmentID int64) string {
	return fmt.Sprintf("poll-decline:%s:%d", pollID, assignmentID)
}

/**
 * NOTE: "Não consigo no domingo" also says the person goes on the other days of the poll. Those still unanswered are
 * confirmed, and a day declined earlier in this same poll is taken back. A day declined some other way (an absence in
 * the app, a message, a leader) stays declined: this answer is not about it.
 */
func (b *Bot) goingOnOtherDays(ctx context.Context, person store.Person, pollID messenger.MessageID, rest []int64) {
	var takenBack, unanswered []int64
	for _, id := range rest {
		key := pollDeclineKey(pollID, id)
		if _, ok := b.Store.NotifiedAt(ctx, key); ok {
			takenBack = append(takenBack, id)
			_ = b.Store.UnmarkNotified(ctx, key)
			continue
		}
		unanswered = append(unanswered, id)
	}
	if len(takenBack) > 0 {
		b.confirm(ctx, person, takenBack, false)
	}
	if ids := b.upcomingSlots(ctx, unanswered); len(ids) > 0 {
		if _, err := b.Store.SetAssignmentsStatus(ctx, person.ID, ids, store.AssignmentConfirmed, b.now()); err != nil {
			b.Log.Error("confirmar presença", "erro", err)
		}
	}
}

func otherSlots(options []store.PollOption, declined []int64) []int64 {
	var rest []int64
	for _, o := range options {
		if o.Action != actionConfirm {
			continue
		}
		for _, id := range o.Assignments {
			if !containsID(declined, id) && !containsID(rest, id) {
				rest = append(rest, id)
			}
		}
	}
	return rest
}

func (b *Bot) confirm(ctx context.Context, person store.Person, assignments []int64, reply bool) {
	if assignments = b.upcomingSlots(ctx, assignments); len(assignments) == 0 {
		return
	}
	changed, err := b.Store.SetAssignmentsStatus(ctx, person.ID, assignments, store.AssignmentConfirmed, b.now())
	if err != nil {
		b.Log.Error("confirmar presença", "erro", err)
		return
	}
	confirmed := len(changed) > 0
	for _, id := range assignments {
		if containsID(changed, id) {
			continue
		}
		result, err := b.Store.WithdrawDecline(ctx, id, person.ID, b.now())
		if errors.Is(err, store.ErrCoverClosed) {
			if d, err := b.Store.AssignmentDetail(ctx, id); err == nil && d.PersonID != person.ID {
				b.send(ctx, person.Phone, comebackTakenReply(d))
			}
			continue
		}
		if err != nil {
			b.Log.Error("voltar atrás na ausência", "erro", err)
			continue
		}
		confirmed = true
		d := result.Assignment
		for _, waiting := range result.Waiting {
			b.send(ctx, waiting.Phone, comebackReleased(waiting.Name, person.Name, d))
		}
		b.tellLeaders(ctx, d.MinistryID, comebackAlert(person.Name, d), person.ID)
	}
	if confirmed && reply {
		b.send(ctx, person.Phone, confirmedReply(person.Name))
	}
}

func containsID(list []int64, id int64) bool {
	for _, item := range list {
		if item == id {
			return true
		}
	}
	return false
}

type announcement int

const (
	withoutGroup announcement = iota
	announced
	announceLater
)

/**
 * NOTE: Posts the published month to the ministry group in the format the team already uses: one line per service
 * with the people in rotation (fixed members are implied), a 🍷 on Lord's Supper days and the missing roles.
 */
func (b *Bot) AnnouncePublished(ctx context.Context, ministryID int64, month string) {
	b.announce(ctx, ministryID, month)
}

/**
 * NOTE: A month goes to the group once. A post that fails is remembered and tried again by the next ticks while the
 * month is not over (retryAnnouncements), so the group never misses a published month.
 */
func (b *Bot) announce(ctx context.Context, ministryID int64, month string) announcement {
	ministry, err := b.Store.Ministry(ctx, ministryID)
	if err != nil || !b.allows(ministry.WhatsAppGroup) {
		return withoutGroup
	}
	key := fmt.Sprintf("announce:%d:%s", ministryID, month)
	b.once(ctx, key, func(ctx context.Context) bool {
		view, err := b.Store.ScheduleView(ctx, ministryID, month)
		if err != nil {
			b.Log.Error("ler escala para anunciar", "erro", err)
			return false
		}
		text, marked := summarize(ministry, view, true)
		return b.mention(ctx, ministry.WhatsAppGroup, text, marked...)
	})
	retry := "announce-retry:" + key
	if _, done := b.Store.NotifiedAt(ctx, key); done {
		_ = b.Store.UnmarkNotified(ctx, retry)
		return announced
	}
	_, _ = b.Store.MarkNotified(ctx, retry, b.now())
	return announceLater
}

func (b *Bot) retryAnnouncements(ctx context.Context, now time.Time) {
	keys, err := b.Store.NotificationKeys(ctx, "announce-retry:announce:")
	if err != nil {
		b.Log.Error("buscar escalas por anunciar", "erro", err)
		return
	}
	for _, key := range keys {
		var ministryID int64
		var month string
		if _, err := fmt.Sscanf(key, "announce-retry:announce:%d:%s", &ministryID, &month); err != nil || month < now.Format("2006-01") {
			_ = b.Store.UnmarkNotified(ctx, key)
			continue
		}
		b.announce(ctx, ministryID, month)
	}
}

/**
 * NOTE: Summary is the month in the format the team already uses, with plain names, as the leaders preview it.
 */
func Summary(ministry store.MinistryDetail, view store.ScheduleView) string {
	text, _ := summarize(ministry, view, false)
	return text
}

/**
 * NOTE: One line per service with the people in rotation (fixed members are implied), a 🍷 on Lord's Supper days and
 * the missing roles. With mark, people with a phone are written as mentions and returned to be marked.
 */
func summarize(ministry store.MinistryDetail, view store.ScheduleView, mark bool) (string, []store.Person) {
	phones := map[int64]string{}
	for _, m := range ministry.Members {
		phones[m.PersonID] = m.Phone
	}
	var marked []store.Person
	seen := map[int64]bool{}
	nameOf := func(slot store.ScheduleSlot) string {
		phone := phones[slot.PersonID]
		if !mark || phone == "" {
			return slot.PersonName
		}
		if !seen[slot.PersonID] {
			seen[slot.PersonID] = true
			marked = append(marked, store.Person{ID: slot.PersonID, Name: slot.PersonName, Phone: phone})
		}
		return "@" + phone
	}
	fixed := map[int64]map[int64]bool{}
	for _, m := range ministry.Members {
		fixed[m.PersonID] = map[int64]bool{}
		for _, r := range m.Roles {
			fixed[m.PersonID][r.RoleID] = r.Fixed
		}
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Paz, pessoal! 🎶 Escala do *%s* de %s:\n", ministry.Name, monthName(view.Month))
	for _, week := range view.Weeks {
		if len(week.Services) == 0 {
			continue
		}
		fmt.Fprintf(&out, "\n*Semana de %s*\n\n", fullDate(week.Services[0].Date))
		for _, sv := range week.Services {
			var names, missing []string
			for _, role := range sv.Roles {
				for _, slot := range role.Slots {
					switch {
					case slot.PersonID == 0:
						missing = append(missing, role.Name)
					case !fixed[slot.PersonID][role.RoleID] && !contains(names, nameOf(slot)):
						names = append(names, nameOf(slot))
					}
				}
			}
			line := fmt.Sprintf("• %s (%s) — %s", capitalize(weekday(sv.Date)), fullDate(sv.Date), joinNames(names))
			if len(missing) > 0 {
				sort.Strings(missing)
				line += " ⚠️ falta " + joinNames(missing)
			}
			out.WriteString(line + communionMark(sv.Communion) + "\n")
		}
	}
	return strings.TrimRight(out.String(), "\n"), marked
}

/**
 * NOTE: A poll to check the WhatsApp connection end to end; its votes only show up in the log.
 */
func (b *Bot) SendTestPoll(ctx context.Context, to messenger.Address) (messenger.MessageID, error) {
	if b.Messenger == nil {
		return "", errors.New("WhatsApp desligado")
	}
	options := []store.PollOption{
		{Label: optionGoingBoth, Action: actionTestPoll},
		{Label: "😕 Não consigo na quinta", Action: actionTestPoll},
		{Label: "😕 Não consigo no domingo", Action: actionTestPoll},
		{Label: optionNotGoingAny, Action: actionTestPoll},
	}
	question := "Teste da louvadeira 🎶 Imagine que esta é a confirmação da escala da semana. Responda, depois mude a resposta e depois tire a resposta."
	id, err := b.Messenger.SendPoll(ctx, to, messenger.Poll{Question: question, Options: labelsOf(options)})
	if err != nil {
		return "", err
	}
	return id, b.Store.SavePoll(ctx, store.Poll{MessageID: string(id), Kind: store.PollTest, Options: options}, b.now())
}
