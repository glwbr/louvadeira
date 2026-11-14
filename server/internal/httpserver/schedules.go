package httpserver

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"louvadeira/internal/store"
)

var monthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

func pathMonth(w http.ResponseWriter, r *http.Request) (string, bool) {
	month := r.PathValue("month")
	if !monthPattern.MatchString(month) {
		writeError(w, http.StatusBadRequest, "mês inválido: use AAAA-MM")
		return "", false
	}
	return month, true
}

func (d Deps) getSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	month, okMonth := pathMonth(w, r)
	if !ok || !okMonth {
		return
	}
	view, err := d.Store.ScheduleView(r.Context(), id, month)
	d.respond(w, http.StatusOK, view, err)
}

func (d Deps) generateSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	month, okMonth := pathMonth(w, r)
	if !ok || !okMonth {
		return
	}
	if err := d.Store.GenerateDraft(r.Context(), id, month, d.now()); err != nil {
		d.fail(w, err)
		return
	}
	view, err := d.Store.ScheduleView(r.Context(), id, month)
	d.respond(w, http.StatusOK, view, err)
}

func (d Deps) discardSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	month, okMonth := pathMonth(w, r)
	if !ok || !okMonth {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.DiscardDraft(r.Context(), id, month))
}

type assignmentRequest struct {
	PersonID *int64 `json:"personId"`
}

func (d Deps) setAssignment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	month, okMonth := pathMonth(w, r)
	assignmentID, okAssignment := pathID(w, r, "assignmentId")
	var req assignmentRequest
	if !ok || !okMonth || !okAssignment || !decodeJSON(w, r, &req) {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.SetAssignment(r.Context(), id, month, assignmentID, req.PersonID, d.now()))
}

func (d Deps) publishSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	month, okMonth := pathMonth(w, r)
	if !ok || !okMonth {
		return
	}
	if err := d.Store.PublishSchedule(r.Context(), id, month, d.now()); err != nil {
		d.fail(w, err)
		return
	}
	if d.Bot != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			d.Bot.AnnouncePublished(ctx, id, month)
		}()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d Deps) declineAssignment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	month, okMonth := pathMonth(w, r)
	assignmentID, okAssignment := pathID(w, r, "assignmentId")
	if !ok || !okMonth || !okAssignment {
		return
	}
	if d.Bot == nil {
		writeError(w, http.StatusServiceUnavailable, "trocas indisponíveis")
		return
	}
	if err := d.Store.CheckAssignment(r.Context(), id, month, assignmentID); err != nil {
		d.fail(w, err)
		return
	}
	requests, err := d.Bot.Decline(r.Context(), []int64{assignmentID}, false)
	if err != nil {
		d.fail(w, err)
		return
	}
	d.respond(w, http.StatusOK, requests[0], nil)
}

func (d Deps) listCovers(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	month, okMonth := pathMonth(w, r)
	if !ok || !okMonth {
		return
	}
	covers, err := d.Store.CoverRequestsForSchedule(r.Context(), id, month)
	d.respond(w, http.StatusOK, covers, err)
}

type reorderRolesRequest struct {
	RoleIDs []int64 `json:"roleIds"`
}

func (d Deps) reorderRoles(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	var req reorderRolesRequest
	if !ok || !decodeJSON(w, r, &req) {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.ReorderRoles(r.Context(), id, req.RoleIDs))
}

func (d Deps) listUnavailabilities(w http.ResponseWriter, r *http.Request) {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if !datePattern.MatchString(from) || !datePattern.MatchString(to) {
		writeError(w, http.StatusBadRequest, "informe from e to como AAAA-MM-DD")
		return
	}
	list, err := d.Store.ListUnavailabilities(r.Context(), from, to)
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

/**
 * NOTE: Scheduled counts the slots the person still holds during the absence, so the leader can open swaps for them.
 */
type unavailabilityView struct {
	store.Unavailability
	Scheduled int `json:"scheduled"`
}

func (d Deps) viewUnavailability(ctx context.Context, u store.Unavailability) (unavailabilityView, error) {
	view := unavailabilityView{Unavailability: u}
	if d.Bot == nil {
		return view, nil
	}
	slots, err := d.Bot.AffectedBy(ctx, u)
	view.Scheduled = len(slots)
	return view, err
}

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func (d Deps) createUnavailability(w http.ResponseWriter, r *http.Request) {
	var in store.UnavailabilityInput
	if !decodeJSON(w, r, &in) {
		return
	}
	u, err := d.Store.CreateUnavailability(r.Context(), in)
	if err != nil {
		d.fail(w, err)
		return
	}
	view, err := d.viewUnavailability(r.Context(), u)
	d.respond(w, http.StatusCreated, view, err)
}

type affectedSlot struct {
	AssignmentID int64  `json:"assignmentId"`
	MinistryID   int64  `json:"ministryId"`
	MinistryName string `json:"ministryName"`
	Date         string `json:"date"`
	StartTime    string `json:"startTime"`
	ServiceName  string `json:"serviceName"`
	RoleName     string `json:"roleName"`
	Status       string `json:"status"`
}

func (d Deps) unavailabilityAssignments(w http.ResponseWriter, r *http.Request) {
	u, ok := d.unavailability(w, r)
	if !ok {
		return
	}
	d.writeAffected(w, r, u)
}

func (d Deps) writeAffected(w http.ResponseWriter, r *http.Request, u store.Unavailability) {
	slots, err := d.Bot.AffectedBy(r.Context(), u)
	list := make([]affectedSlot, len(slots))
	for i, s := range slots {
		list[i] = affectedSlot{s.AssignmentID, s.MinistryID, s.MinistryName, s.Date, s.StartTime, s.ServiceName, s.RoleName, s.Status}
	}
	d.respond(w, http.StatusOK, list, err)
}

func (d Deps) openUnavailabilityCovers(w http.ResponseWriter, r *http.Request) {
	u, ok := d.unavailability(w, r)
	if !ok {
		return
	}
	requests, err := d.Bot.DeclinePeriod(r.Context(), u)
	d.respond(w, http.StatusOK, requests, err)
}

func (d Deps) unavailability(w http.ResponseWriter, r *http.Request) (store.Unavailability, bool) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return store.Unavailability{}, false
	}
	if d.Bot == nil {
		writeError(w, http.StatusServiceUnavailable, "trocas indisponíveis")
		return store.Unavailability{}, false
	}
	u, err := d.Store.Unavailability(r.Context(), id)
	if err != nil {
		d.fail(w, err)
		return store.Unavailability{}, false
	}
	return u, true
}

func (d Deps) deleteUnavailability(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.DeleteUnavailability(r.Context(), id))
}
