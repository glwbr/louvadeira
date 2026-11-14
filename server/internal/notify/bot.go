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
	"time"

	"louvadeira/internal/messenger"
	"louvadeira/internal/phone"
	"louvadeira/internal/store"
)

const (
	confirmAhead   = 7 * 24 * time.Hour
	pendingAhead   = 3 * 24 * time.Hour
	answerTime     = 24 * time.Hour
	lateNotice     = 72 * time.Hour
	firstHour      = 8
	lastHour       = 21
	reminderHour   = 10
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

	mu      sync.Mutex
	pending map[messenger.MessageID]*time.Timer
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

func (b *Bot) serviceTime(d store.AssignmentDetail) time.Time {
	loc := b.Location
	if loc == nil {
		loc = time.UTC
	}
	t, _ := time.ParseInLocation("2006-01-02 15:04", d.Date+" "+d.StartTime, loc)
	return t
}

func (b *Bot) send(ctx context.Context, to string, text string) bool {
	if to == "" || b.Messenger == nil {
		return false
	}
	if _, err := b.Messenger.SendText(ctx, messenger.Address(to), text); err != nil {
		b.logSendError(to, err)
		return false
	}
	return true
}

func (b *Bot) sendPoll(ctx context.Context, kind string, person store.Person, question string, options []store.PollOption, multiple bool) bool {
	if person.Phone == "" || b.Messenger == nil {
		return false
	}
	id, err := b.Messenger.SendPoll(ctx, messenger.Address(person.Phone), messenger.Poll{Question: question, Options: labelsOf(options), Multiple: multiple})
	if err != nil {
		b.logSendError(person.Phone, err)
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

func (b *Bot) tellLeaders(ctx context.Context, ministryID int64, text string) {
	leaders, err := b.Store.Leaders(ctx, ministryID)
	if err != nil {
		b.Log.Error("buscar líderes", "erro", err)
		return
	}
	for _, leader := range leaders {
		b.send(ctx, leader.Phone, text)
	}
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
	now := b.now()
	if now.Hour() < firstHour || now.Hour() >= lastHour {
		return
	}
	today := now.Format("2006-01-02")
	// NOTE: Reads two weeks so a week that only starts inside the 7-day window is asked about as a whole.
	upcoming, err := b.Store.UpcomingAssignments(ctx, today, now.Add(2*confirmAhead).Format("2006-01-02"))
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
	b.sendConfirmations(ctx, future, now)
	if now.Hour() >= reminderHour {
		b.sendReminders(ctx, future, now.AddDate(0, 0, 1).Format("2006-01-02"))
	}
	if now.Hour() >= pendingHour {
		b.alertPending(ctx, future, now)
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
		if !pending || !b.allows(slots[0].Phone) || b.serviceTime(slots[0]).Sub(now) > confirmAhead {
			continue
		}
		key := confirmKey(k.ministryID, k.personID, k.key)
		if first, err := b.Store.MarkNotified(ctx, key, b.now()); err != nil || !first {
			continue
		}
		if !b.askConfirmation(ctx, slots) {
			_ = b.Store.UnmarkNotified(ctx, key)
			continue
		}
		asked = append(asked, slots[0].PersonName)
	}
	return asked
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
	var lines []string
	var all []int64
	for _, o := range occurrences {
		lines = append(lines, confirmLine(o))
		all = append(all, o.assignments...)
	}
	question := confirmQuestion(first.PersonName, first.MinistryName, lines)

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

func (b *Bot) sendReminders(ctx context.Context, upcoming []store.AssignmentDetail, tomorrow string) {
	type key struct {
		person  int64
		service int64
	}
	byKey := map[key][]store.AssignmentDetail{}
	var order []key
	for _, d := range upcoming {
		if d.Date != tomorrow || d.Status == store.AssignmentDeclined {
			continue
		}
		k := key{d.PersonID, d.ServiceID}
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
		mark := fmt.Sprintf("remind:%d:%s:%d", k.person, d.Date, k.service)
		if first, err := b.Store.MarkNotified(ctx, mark, b.now()); err != nil || !first {
			continue
		}
		roles := make([]string, len(slots))
		for i, slot := range slots {
			roles[i] = slot.RoleName
		}
		if !b.send(ctx, d.Phone, reminderText(d, roles)) {
			_ = b.Store.UnmarkNotified(ctx, mark)
		}
	}
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
		if d.Status != store.AssignmentScheduled || b.serviceTime(d).Sub(now) > pendingAhead {
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
		mark := fmt.Sprintf("pending:%d:%s:%d", k.ministry, k.date, k.service)
		if first, err := b.Store.MarkNotified(ctx, mark, b.now()); err != nil || !first {
			continue
		}
		d := sample[k]
		b.tellLeaders(ctx, k.ministry, pendingAlert(d, pending[k]))
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
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending == nil {
		b.pending = map[messenger.MessageID]*time.Timer{}
	}
	if timer, ok := b.pending[vote.PollID]; ok {
		timer.Stop()
	}
	b.pending[vote.PollID] = time.AfterFunc(b.Settle, func() {
		b.mu.Lock()
		delete(b.pending, vote.PollID)
		b.mu.Unlock()
		b.applyVote(context.Background(), vote)
	})
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
	person, ok := b.person(ctx, poll.PersonID)
	if !ok {
		return
	}
	if vote.FromMe && (b.OwnPhone == nil || phone.Canonical(b.OwnPhone()) != person.Phone) {
		return
	}
	b.Log.Info("voto recebido", "enquete", poll.Kind, "pessoa", person.Name, "escolha", choice)

	if poll.Kind == store.PollCover {
		b.answerCover(ctx, person, poll.Options, chosen)
		return
	}
	option := chosen[len(chosen)-1]
	switch option.Action {
	case actionConfirm:
		b.confirm(ctx, person, option.Assignments)
	case actionDecline:
		var ids []int64
		for _, id := range option.Assignments {
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
}

func (b *Bot) confirm(ctx context.Context, person store.Person, assignments []int64) {
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
		b.tellLeaders(ctx, d.MinistryID, comebackAlert(person.Name, d))
	}
	if confirmed {
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

/**
 * NOTE: Posts the published month to the ministry group in the format the team already uses: one line per service
 * with the people in rotation (fixed members are implied), a 🍷 on Lord's Supper days and the missing roles.
 */
func (b *Bot) AnnouncePublished(ctx context.Context, ministryID int64, month string) {
	ministry, err := b.Store.Ministry(ctx, ministryID)
	if err != nil || ministry.WhatsAppGroup == "" {
		return
	}
	view, err := b.Store.ScheduleView(ctx, ministryID, month)
	if err != nil {
		b.Log.Error("ler escala para anunciar", "erro", err)
		return
	}
	b.send(ctx, ministry.WhatsAppGroup, Summary(ministry, view))
}

func Summary(ministry store.MinistryDetail, view store.ScheduleView) string {
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
					case !fixed[slot.PersonID][role.RoleID] && !contains(names, slot.PersonName):
						names = append(names, slot.PersonName)
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
	return strings.TrimRight(out.String(), "\n")
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
