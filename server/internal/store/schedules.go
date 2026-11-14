package store

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"louvadeira/internal/schedule"
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"

	historyWeeks = 8
)

var ErrPublished = ValidationError{Message: "esta escala já foi publicada; mudanças agora são trocas"}

type SlotWarning string

const (
	WarningEmpty        SlotWarning = "empty"
	WarningUnavailable  SlotWarning = "unavailable"
	WarningDoubleBooked SlotWarning = "doubleBooked"
	WarningNotMember    SlotWarning = "notMember"
)

type ScheduleView struct {
	ID          int64          `json:"id"`
	MinistryID  int64          `json:"ministryId"`
	Month       string         `json:"month"`
	Status      string         `json:"status"`
	GeneratedAt string         `json:"generatedAt"`
	PublishedAt string         `json:"publishedAt"`
	Warnings    int            `json:"warnings"`
	Weeks       []ScheduleWeek `json:"weeks"`
}

type ScheduleWeek struct {
	Start    string            `json:"start"`
	Services []ScheduleService `json:"services"`
}

type ScheduleService struct {
	ServiceID int64          `json:"serviceId"`
	Date      string         `json:"date"`
	Name      string         `json:"name"`
	StartTime string         `json:"startTime"`
	Communion bool           `json:"communion"`
	Roles     []ScheduleRole `json:"roles"`
}

type ScheduleRole struct {
	RoleID int64          `json:"roleId"`
	Name   string         `json:"name"`
	Slots  []ScheduleSlot `json:"slots"`
}

type ScheduleSlot struct {
	AssignmentID int64       `json:"assignmentId"`
	PersonID     int64       `json:"personId"`
	PersonName   string      `json:"personName"`
	Status       string      `json:"status"`
	CoverOpen    bool        `json:"coverOpen"`
	Warning      SlotWarning `json:"warning,omitempty"`
}

/**
 * NOTE: Builds the generator input from the ministry's current team, its services, the unavailabilities in the
 * period and the published history of the weeks before it, then replaces the draft with the result.
 */
func (s *Store) GenerateDraft(ctx context.Context, ministryID int64, month string, now time.Time) error {
	ministry, err := s.Ministry(ctx, ministryID)
	if err != nil {
		return err
	}
	var services []schedule.Service
	all, err := s.ListServices(ctx)
	if err != nil {
		return err
	}
	for _, sv := range all {
		if sv.Active && containsID(ministry.ServiceIDs, sv.ID) {
			services = append(services, schedule.Service{ID: sv.ID, Weekday: time.Weekday(sv.Weekday), StartTime: sv.StartTime})
		}
	}
	if len(services) == 0 {
		return invalid("marque na aba Cultos em quais cultos este ministério serve")
	}
	if len(ministry.Roles) == 0 {
		return invalid("cadastre as funções do ministério antes de gerar a escala")
	}
	groups, err := schedule.Period(month, services, schedule.Unit(ministry.RotationUnit))
	if err != nil {
		return invalid("%s", err.Error())
	}
	if len(groups) == 0 {
		return invalid("não há cultos deste ministério neste mês")
	}
	first := groups[0].Occurrences[0].Date
	lastGroup := groups[len(groups)-1].Occurrences
	last := lastGroup[len(lastGroup)-1].Date

	in := schedule.Input{Unit: schedule.Unit(ministry.RotationUnit), Groups: groups}
	for _, role := range ministry.Roles {
		in.Roles = append(in.Roles, schedule.Role{ID: role.ID, Slots: role.Slots})
	}
	for _, member := range ministry.Members {
		roles := map[int64]bool{}
		for _, role := range member.Roles {
			roles[role.RoleID] = role.Fixed
		}
		in.Members = append(in.Members, schedule.Member{PersonID: member.PersonID, Roles: roles})
	}
	unavailable, err := s.ListUnavailabilities(ctx, first, last)
	if err != nil {
		return err
	}
	for _, u := range unavailable {
		for _, group := range groups {
			for _, o := range group.Occurrences {
				if u.Covers(o.Date) {
					in.Unavailable = append(in.Unavailable, schedule.Unavailability{PersonID: u.PersonID, Start: o.Date, End: o.Date})
				}
			}
		}
	}
	if in.History, err = s.history(ctx, ministryID, first); err != nil {
		return err
	}

	return s.saveDraft(ctx, ministryID, month, now, schedule.Generate(in))
}

func (s *Store) history(ctx context.Context, ministryID int64, before string) ([]schedule.PastAssignment, error) {
	start, err := time.Parse("2006-01-02", before)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.person_id, a.role_id, a.service_date
		FROM assignments a JOIN schedules sc ON sc.id = a.schedule_id
		WHERE sc.ministry_id = ? AND sc.status = 'published' AND a.person_id IS NOT NULL
		  AND a.service_date >= ? AND a.service_date < ?
		ORDER BY a.service_date`,
		ministryID, start.AddDate(0, 0, -7*historyWeeks).Format("2006-01-02"), before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var history []schedule.PastAssignment
	for rows.Next() {
		var past schedule.PastAssignment
		if err := rows.Scan(&past.PersonID, &past.RoleID, &past.Date); err != nil {
			return nil, err
		}
		history = append(history, past)
	}
	return history, rows.Err()
}

func (s *Store) saveDraft(ctx context.Context, ministryID int64, month string, now time.Time, generated []schedule.Assignment) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var id int64
		var status string
		err := tx.QueryRowContext(ctx, "SELECT id, status FROM schedules WHERE ministry_id = ? AND month = ?", ministryID, month).Scan(&id, &status)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			err = tx.QueryRowContext(ctx,
				"INSERT INTO schedules (ministry_id, month, generated_at) VALUES (?, ?, ?) RETURNING id",
				ministryID, month, formatTime(now)).Scan(&id)
			if err != nil {
				return err
			}
		case err != nil:
			return err
		case status == StatusPublished:
			return ErrPublished
		default:
			if _, err := tx.ExecContext(ctx, "UPDATE schedules SET generated_at = ? WHERE id = ?", formatTime(now), id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "DELETE FROM assignments WHERE schedule_id = ?", id); err != nil {
				return err
			}
		}
		insert, err := tx.PrepareContext(ctx, `
			INSERT INTO assignments (schedule_id, service_id, service_date, role_id, slot, person_id)
			VALUES (?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer insert.Close()
		for _, a := range generated {
			person := sql.NullInt64{Int64: a.PersonID, Valid: a.PersonID != 0}
			if _, err := insert.ExecContext(ctx, id, a.ServiceID, a.Date, a.RoleID, a.Slot, person); err != nil {
				return err
			}
		}
		return nil
	})
}

/**
 * NOTE: Fails with ErrNotFound unless the slot belongs to the ministry's schedule for that month, and with a
 * validation error while the schedule is still a draft (swaps only exist once it is published).
 */
func (s *Store) CheckAssignment(ctx context.Context, ministryID int64, month string, assignmentID int64) error {
	var status string
	err := s.db.QueryRowContext(ctx, `
		SELECT sc.status FROM assignments a JOIN schedules sc ON sc.id = a.schedule_id
		WHERE a.id = ? AND sc.ministry_id = ? AND sc.month = ?`, assignmentID, ministryID, month).Scan(&status)
	if err != nil {
		return notFoundIfNoRows(err)
	}
	if status != StatusPublished {
		return invalid("publique a escala antes de registrar trocas")
	}
	return nil
}

/**
 * CONSTRAINT: Only members of the ministry can be put in a slot; a nil person empties it. A manual change settles any
 * swap still open for the slot and the new person has to confirm again.
 */
func (s *Store) SetAssignment(ctx context.Context, ministryID int64, month string, assignmentID int64, personID *int64, now time.Time) error {
	var scheduleID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT sc.id FROM assignments a JOIN schedules sc ON sc.id = a.schedule_id
		WHERE a.id = ? AND sc.ministry_id = ? AND sc.month = ?`, assignmentID, ministryID, month).Scan(&scheduleID)
	if err != nil {
		return notFoundIfNoRows(err)
	}
	if personID != nil {
		var member bool
		if err := s.db.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM ministry_members WHERE ministry_id = ? AND person_id = ?)", ministryID, *personID).Scan(&member); err != nil {
			return err
		}
		if !member {
			return invalid("essa pessoa não faz parte do ministério")
		}
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			"UPDATE assignments SET person_id = ?, status = 'scheduled', responded_at = NULL WHERE id = ?", personID, assignmentID); err != nil {
			return err
		}
		outcome := CoverUncovered
		if personID != nil {
			outcome = CoverCovered
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE cover_requests SET status = ?, covered_by = ?, resolved_at = ?
			WHERE assignment_id = ? AND status = 'open'`, outcome, personID, formatTime(now), assignmentID)
		return err
	})
}

/**
 * NOTE: Only drafts can be discarded; a published schedule is history and changes to it are swaps.
 */
func (s *Store) DiscardDraft(ctx context.Context, ministryID int64, month string) error {
	var status string
	err := s.db.QueryRowContext(ctx, "SELECT status FROM schedules WHERE ministry_id = ? AND month = ?", ministryID, month).Scan(&status)
	if err != nil {
		return notFoundIfNoRows(err)
	}
	if status == StatusPublished {
		return ErrPublished
	}
	_, err = s.db.ExecContext(ctx, "DELETE FROM schedules WHERE ministry_id = ? AND month = ? AND status = 'draft'", ministryID, month)
	return err
}

func (s *Store) PublishSchedule(ctx context.Context, ministryID int64, month string, now time.Time) error {
	return requireAffected(s.db.ExecContext(ctx, `
		UPDATE schedules SET status = 'published', published_at = ?
		WHERE ministry_id = ? AND month = ? AND status = 'draft'`, formatTime(now), ministryID, month))
}

func (s *Store) ScheduleView(ctx context.Context, ministryID int64, month string) (ScheduleView, error) {
	view := ScheduleView{MinistryID: ministryID, Month: month}
	var publishedAt sql.NullString
	err := s.db.QueryRowContext(ctx,
		"SELECT id, status, generated_at, published_at FROM schedules WHERE ministry_id = ? AND month = ?", ministryID, month,
	).Scan(&view.ID, &view.Status, &view.GeneratedAt, &publishedAt)
	if err != nil {
		return ScheduleView{}, notFoundIfNoRows(err)
	}
	view.PublishedAt = publishedAt.String

	ministry, err := s.Ministry(ctx, ministryID)
	if err != nil {
		return ScheduleView{}, err
	}
	fixed := map[int64]map[int64]bool{}
	for _, member := range ministry.Members {
		fixed[member.PersonID] = map[int64]bool{}
		for _, role := range member.Roles {
			fixed[member.PersonID][role.RoleID] = role.Fixed
		}
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.service_id, a.service_date, sv.name, sv.start_time, sv.communion_week, a.role_id, r.name, coalesce(a.person_id, 0), coalesce(p.name, ''),
		       a.status, EXISTS (SELECT 1 FROM cover_requests cr WHERE cr.assignment_id = a.id AND cr.status = 'open')
		FROM assignments a
		JOIN services sv ON sv.id = a.service_id
		JOIN roles r ON r.id = a.role_id
		LEFT JOIN people p ON p.id = a.person_id
		WHERE a.schedule_id = ?
		ORDER BY a.service_date, sv.start_time, r.position, r.id, a.slot`, view.ID)
	if err != nil {
		return ScheduleView{}, err
	}
	defer rows.Close()

	type occurrenceKey struct {
		date    string
		service int64
	}
	var services []*ScheduleService
	byKey := map[occurrenceKey]*ScheduleService{}
	for rows.Next() {
		var slot ScheduleSlot
		var occurrence ScheduleService
		var roleID int64
		var roleName string
		var communionWeek sql.NullInt64
		if err := rows.Scan(&slot.AssignmentID, &occurrence.ServiceID, &occurrence.Date, &occurrence.Name, &occurrence.StartTime,
			&communionWeek, &roleID, &roleName, &slot.PersonID, &slot.PersonName, &slot.Status, &slot.CoverOpen); err != nil {
			return ScheduleView{}, err
		}
		occurrence.Communion = communionWeek.Valid && nthWeekdayOfMonth(occurrence.Date) == int(communionWeek.Int64)
		key := occurrenceKey{occurrence.Date, occurrence.ServiceID}
		current := byKey[key]
		if current == nil {
			current = &occurrence
			byKey[key] = current
			services = append(services, current)
		}
		if n := len(current.Roles); n == 0 || current.Roles[n-1].RoleID != roleID {
			current.Roles = append(current.Roles, ScheduleRole{RoleID: roleID, Name: roleName})
		}
		role := &current.Roles[len(current.Roles)-1]
		role.Slots = append(role.Slots, slot)
	}
	if err := rows.Err(); err != nil {
		return ScheduleView{}, err
	}

	if len(services) > 0 {
		unavailable, err := s.ListUnavailabilities(ctx, services[0].Date, services[len(services)-1].Date)
		if err != nil {
			return ScheduleView{}, err
		}
		for _, sv := range services {
			view.Warnings += markWarnings(sv, fixed, unavailable)
		}
	}

	weekIndex := map[string]int{}
	for _, sv := range services {
		start := schedule.WeekStart(sv.Date)
		i, ok := weekIndex[start]
		if !ok {
			i = len(view.Weeks)
			weekIndex[start] = i
			view.Weeks = append(view.Weeks, ScheduleWeek{Start: start})
		}
		view.Weeks[i].Services = append(view.Weeks[i].Services, *sv)
	}
	sort.SliceStable(view.Weeks, func(i, j int) bool { return view.Weeks[i].Start < view.Weeks[j].Start })
	if view.Weeks == nil {
		view.Weeks = []ScheduleWeek{}
	}
	return view, nil
}

func markWarnings(sv *ScheduleService, fixed map[int64]map[int64]bool, unavailable []Unavailability) int {
	rolesOf := map[int64][]int64{}
	for _, role := range sv.Roles {
		for _, slot := range role.Slots {
			if slot.PersonID != 0 {
				rolesOf[slot.PersonID] = append(rolesOf[slot.PersonID], role.RoleID)
			}
		}
	}
	count := 0
	for r := range sv.Roles {
		for i := range sv.Roles[r].Slots {
			slot := &sv.Roles[r].Slots[i]
			switch {
			case slot.PersonID == 0:
				slot.Warning = WarningEmpty
			case fixed[slot.PersonID] == nil:
				slot.Warning = WarningNotMember
			case isUnavailable(slot.PersonID, sv.Date, unavailable):
				slot.Warning = WarningUnavailable
			case len(rolesOf[slot.PersonID]) > 1 && !allFixed(fixed[slot.PersonID], rolesOf[slot.PersonID]):
				slot.Warning = WarningDoubleBooked
			}
			if slot.Warning != "" {
				count++
			}
		}
	}
	return count
}

func isUnavailable(person int64, date string, list []Unavailability) bool {
	for _, u := range list {
		if u.PersonID == person && u.Covers(date) {
			return true
		}
	}
	return false
}

func allFixed(fixed map[int64]bool, roles []int64) bool {
	for _, role := range roles {
		if !fixed[role] {
			return false
		}
	}
	return true
}

func containsID(ids []int64, id int64) bool {
	for _, item := range ids {
		if item == id {
			return true
		}
	}
	return false
}

func nthWeekdayOfMonth(date string) int {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0
	}
	return (t.Day()-1)/7 + 1
}
