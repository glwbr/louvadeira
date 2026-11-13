package httpserver

import (
	"net/http"
	"regexp"

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
	d.respond(w, http.StatusNoContent, nil, d.Store.PublishSchedule(r.Context(), id, month, d.now()))
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
	d.respond(w, http.StatusOK, list, err)
}

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func (d Deps) createUnavailability(w http.ResponseWriter, r *http.Request) {
	var in store.UnavailabilityInput
	if !decodeJSON(w, r, &in) {
		return
	}
	u, err := d.Store.CreateUnavailability(r.Context(), in)
	d.respond(w, http.StatusCreated, u, err)
}

func (d Deps) deleteUnavailability(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.DeleteUnavailability(r.Context(), id))
}
