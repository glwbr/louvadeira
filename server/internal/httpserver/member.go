package httpserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"louvadeira/internal/notify"
	"louvadeira/internal/store"
)

const (
	memberLinkTTL = time.Hour
	memberAhead   = 120 * 24 * time.Hour
)

type requestLinkRequest struct {
	Phone string `json:"phone"`
}

/**
 * NOTE: Anyone can ask for an access link by phone. The answer is the same whether or not the number is registered,
 * so the form does not reveal who is on the team.
 *
 * CONSTRAINT: While the last link still works, asking again sends nothing: the person already has a link to use, and
 * a stranger who knows the number can make the bot write to it at most once per link lifetime.
 */
func (d Deps) requestLink(w http.ResponseWriter, r *http.Request) {
	var req requestLinkRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	accepted := map[string]bool{"ok": true}
	person, err := d.Store.PersonByPhone(r.Context(), req.Phone)
	var validation store.ValidationError
	if errors.As(err, &validation) {
		writeError(w, http.StatusBadRequest, "confira o número com DDD")
		return
	}
	if err != nil || d.Bot == nil {
		writeJSON(w, http.StatusAccepted, accepted)
		return
	}
	live, err := d.Store.HasLiveLoginToken(r.Context(), person.ID, d.now())
	if err != nil {
		d.Log.Error("ver link de acesso", "erro", err)
	}
	if err != nil || live {
		writeJSON(w, http.StatusAccepted, accepted)
		return
	}
	// NOTE: The link goes out after the answer, so a registered number is answered as fast as any other.
	d.Bot.InBackground(r.Context(), func(ctx context.Context) {
		if err := d.sendLink(ctx, person); err != nil && !errors.Is(err, notify.ErrTooSoon) {
			d.Log.Error("enviar link de acesso", "erro", err)
		}
	})
	writeJSON(w, http.StatusAccepted, accepted)
}

func (d Deps) sendPersonLink(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if d.Bot == nil {
		writeError(w, http.StatusServiceUnavailable, "WhatsApp desligado")
		return
	}
	person, err := d.Store.Person(r.Context(), id)
	if err != nil {
		d.fail(w, err)
		return
	}
	err = d.sendLink(r.Context(), person)
	switch {
	case errors.Is(err, notify.ErrTooSoon):
		writeError(w, http.StatusTooManyRequests, err.Error())
	case err != nil:
		writeError(w, http.StatusBadGateway, err.Error())
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (d Deps) sendLink(ctx context.Context, person store.Person) error {
	token, err := d.Store.CreateLoginToken(ctx, person.ID, d.now(), memberLinkTTL)
	if err != nil {
		return err
	}
	if err := d.Bot.SendLoginLink(ctx, person, d.PublicURL+"/entrar?token="+token); err != nil {
		if discardErr := d.Store.DiscardLoginToken(ctx, token); discardErr != nil {
			d.Log.Error("descartar link de acesso", "erro", discardErr)
		}
		return err
	}
	return nil
}

func (d Deps) member(w http.ResponseWriter, r *http.Request) (store.Person, bool) {
	person, _ := personFrom(r.Context())
	if d.Bot == nil {
		writeError(w, http.StatusServiceUnavailable, "escalas indisponíveis")
		return person, false
	}
	return person, true
}

func (d Deps) mySlots(w http.ResponseWriter, r *http.Request) {
	person, ok := d.member(w, r)
	if !ok {
		return
	}
	today := d.Bot.Today()
	until := d.now().Add(memberAhead).Format("2006-01-02")
	slots, err := d.Store.MemberSlots(r.Context(), person.ID, today, until)
	d.respond(w, http.StatusOK, slots, err)
}

type assignmentsRequest struct {
	AssignmentIDs []int64 `json:"assignmentIds"`
}

/**
 * NOTE: Reads the slots a member is answering for and keeps only theirs that have not happened yet.
 */
func (d Deps) ownSlots(w http.ResponseWriter, r *http.Request, person store.Person) ([]store.AssignmentDetail, bool) {
	var req assignmentsRequest
	if !decodeJSON(w, r, &req) {
		return nil, false
	}
	if len(req.AssignmentIDs) == 0 {
		writeError(w, http.StatusBadRequest, "escolha o culto")
		return nil, false
	}
	var slots []store.AssignmentDetail
	for _, id := range req.AssignmentIDs {
		slot, err := d.Store.MemberAssignment(r.Context(), person.ID, id)
		if err != nil {
			d.fail(w, err)
			return nil, false
		}
		if !d.Bot.Upcoming(slot) {
			writeError(w, http.StatusBadRequest, "esse culto já passou")
			return nil, false
		}
		slots = append(slots, slot)
	}
	return slots, true
}

func (d Deps) confirmMine(w http.ResponseWriter, r *http.Request) {
	person, ok := d.member(w, r)
	if !ok {
		return
	}
	slots, ok := d.ownSlots(w, r, person)
	if !ok {
		return
	}
	ids := make([]int64, len(slots))
	for i, slot := range slots {
		ids[i] = slot.AssignmentID
	}
	d.Bot.ConfirmFromApp(r.Context(), person, ids)
	w.WriteHeader(http.StatusNoContent)
}

func (d Deps) declineMine(w http.ResponseWriter, r *http.Request) {
	person, ok := d.member(w, r)
	if !ok {
		return
	}
	slots, ok := d.ownSlots(w, r, person)
	if !ok {
		return
	}
	var ids []int64
	for _, slot := range slots {
		if slot.Status != store.AssignmentDeclined {
			ids = append(ids, slot.AssignmentID)
		}
	}
	if len(ids) == 0 {
		writeError(w, http.StatusBadRequest, "já estamos procurando substituto para esse culto")
		return
	}
	requests, err := d.Bot.Decline(r.Context(), ids, false)
	d.respond(w, http.StatusOK, requests, err)
}

func (d Deps) myAbsences(w http.ResponseWriter, r *http.Request) {
	person, ok := d.member(w, r)
	if !ok {
		return
	}
	list, err := d.Store.PersonUnavailabilities(r.Context(), person.ID, d.Bot.Today())
	if err != nil {
		d.fail(w, err)
		return
	}
	views := make([]unavailabilityView, len(list))
	for i, u := range list {
		if views[i], err = d.viewUnavailability(r.Context(), u); err != nil {
			d.fail(w, err)
			return
		}
	}
	d.respond(w, http.StatusOK, views, nil)
}

func (d Deps) createMyAbsence(w http.ResponseWriter, r *http.Request) {
	person, ok := d.member(w, r)
	if !ok {
		return
	}
	var in store.UnavailabilityInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.PersonID = person.ID
	u, err := d.Store.CreateUnavailability(r.Context(), in)
	if err != nil {
		d.fail(w, err)
		return
	}
	d.respondCreatedAbsence(w, r, u)
}

func (d Deps) ownAbsence(w http.ResponseWriter, r *http.Request) (store.Unavailability, bool) {
	person, ok := d.member(w, r)
	if !ok {
		return store.Unavailability{}, false
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return store.Unavailability{}, false
	}
	u, err := d.Store.Unavailability(r.Context(), id)
	if err == nil && u.PersonID != person.ID {
		err = store.ErrNotFound
	}
	if err != nil {
		d.fail(w, err)
		return store.Unavailability{}, false
	}
	return u, true
}

func (d Deps) deleteMyAbsence(w http.ResponseWriter, r *http.Request) {
	if u, ok := d.ownAbsence(w, r); ok {
		d.respond(w, http.StatusNoContent, nil, d.Store.DeleteUnavailability(r.Context(), u.ID))
	}
}

func (d Deps) myAbsenceSlots(w http.ResponseWriter, r *http.Request) {
	if u, ok := d.ownAbsence(w, r); ok {
		d.writeAffected(w, r, u)
	}
}

func (d Deps) openMyCovers(w http.ResponseWriter, r *http.Request) {
	if u, ok := d.ownAbsence(w, r); ok {
		requests, err := d.Bot.DeclinePeriod(r.Context(), u)
		d.respond(w, http.StatusOK, requests, err)
	}
}

/**
 * NOTE: Gives someone back the services they said they could not serve while nobody has taken them yet, as if they
 * had answered on WhatsApp: whoever was invited is released and the leaders hear about it.
 */
func (d Deps) comeback(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if d.Bot == nil {
		writeError(w, http.StatusServiceUnavailable, "escalas indisponíveis")
		return
	}
	person, err := d.Store.Person(r.Context(), id)
	if err != nil {
		d.fail(w, err)
		return
	}
	slots, ok := d.ownSlots(w, r, person)
	if !ok {
		return
	}
	ids := make([]int64, len(slots))
	for i, slot := range slots {
		ids[i] = slot.AssignmentID
	}
	d.Bot.ConfirmFromApp(r.Context(), person, ids)
	w.WriteHeader(http.StatusNoContent)
}
