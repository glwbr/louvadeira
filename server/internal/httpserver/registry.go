package httpserver

import (
	"net/http"

	"louvadeira/internal/store"
)

func (d Deps) listPeople(w http.ResponseWriter, r *http.Request) {
	people, err := d.Store.ListPeople(r.Context())
	d.respond(w, http.StatusOK, people, err)
}

func (d Deps) createPerson(w http.ResponseWriter, r *http.Request) {
	var in store.PersonInput
	if !decodeJSON(w, r, &in) {
		return
	}
	person, err := d.Store.CreatePerson(r.Context(), in)
	d.respond(w, http.StatusCreated, person, err)
}

func (d Deps) updatePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	var in store.PersonInput
	if !ok || !decodeJSON(w, r, &in) {
		return
	}
	person, err := d.Store.UpdatePerson(r.Context(), id, in)
	d.respond(w, http.StatusOK, person, err)
}

func (d Deps) archivePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if me, _ := personFrom(r.Context()); me.ID == id {
		writeError(w, http.StatusBadRequest, "você não pode arquivar a si mesmo")
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.ArchivePerson(r.Context(), id))
}

func (d Deps) listServices(w http.ResponseWriter, r *http.Request) {
	services, err := d.Store.ListServices(r.Context())
	d.respond(w, http.StatusOK, services, err)
}

func (d Deps) createService(w http.ResponseWriter, r *http.Request) {
	var in store.ServiceInput
	if !decodeJSON(w, r, &in) {
		return
	}
	service, err := d.Store.CreateService(r.Context(), in)
	d.respond(w, http.StatusCreated, service, err)
}

func (d Deps) updateService(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	var in store.ServiceInput
	if !ok || !decodeJSON(w, r, &in) {
		return
	}
	service, err := d.Store.UpdateService(r.Context(), id, in, d.today())
	d.respond(w, http.StatusOK, service, err)
}

type communionRequest struct {
	ServiceID *int64 `json:"serviceId"`
	Week      *int   `json:"week"`
}

func (d Deps) setCommunion(w http.ResponseWriter, r *http.Request) {
	var req communionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.SetCommunion(r.Context(), req.ServiceID, req.Week))
}

func (d Deps) listMinistries(w http.ResponseWriter, r *http.Request) {
	ministries, err := d.Store.ListMinistries(r.Context())
	d.respond(w, http.StatusOK, ministries, err)
}

func (d Deps) createMinistry(w http.ResponseWriter, r *http.Request) {
	var in store.MinistryInput
	if !decodeJSON(w, r, &in) {
		return
	}
	ministry, err := d.Store.CreateMinistry(r.Context(), in)
	d.respond(w, http.StatusCreated, ministry, err)
}

func (d Deps) getMinistry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ministry, err := d.Store.Ministry(r.Context(), id)
	d.respond(w, http.StatusOK, ministry, err)
}

func (d Deps) updateMinistry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	var in store.MinistryInput
	if !ok || !decodeJSON(w, r, &in) {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.UpdateMinistry(r.Context(), id, in))
}

func (d Deps) archiveMinistry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.ArchiveMinistry(r.Context(), id))
}

func (d Deps) createRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	var in store.RoleInput
	if !ok || !decodeJSON(w, r, &in) {
		return
	}
	role, err := d.Store.CreateRole(r.Context(), id, in)
	d.respond(w, http.StatusCreated, role, err)
}

func (d Deps) updateRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	roleID, okRole := pathID(w, r, "roleId")
	var in store.RoleInput
	if !ok || !okRole || !decodeJSON(w, r, &in) {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.UpdateRole(r.Context(), id, roleID, in))
}

func (d Deps) deleteRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	roleID, okRole := pathID(w, r, "roleId")
	if !ok || !okRole {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.DeleteRole(r.Context(), id, roleID, d.today()))
}

func (d Deps) putMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	personID, okPerson := pathID(w, r, "personId")
	var in store.MemberInput
	if !ok || !okPerson || !decodeJSON(w, r, &in) {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.PutMember(r.Context(), id, personID, in))
}

func (d Deps) removeMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	personID, okPerson := pathID(w, r, "personId")
	if !ok || !okPerson {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.RemoveMember(r.Context(), id, personID))
}

type ministryServicesRequest struct {
	ServiceIDs []int64 `json:"serviceIds"`
}

func (d Deps) setMinistryServices(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	var req ministryServicesRequest
	if !ok || !decodeJSON(w, r, &req) {
		return
	}
	d.respond(w, http.StatusNoContent, nil, d.Store.SetMinistryServices(r.Context(), id, req.ServiceIDs))
}
