package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrNotMember      = errors.New("a pessoa não faz parte do ministério")
	ErrAway           = errors.New("a pessoa avisou que não pode nesse dia")
	ErrNothingMissing = errors.New("não falta mais ninguém nessa função")
)

/**
 * NOTE: A role of one service in a published schedule that has fewer people than its minimum. Slots counts the places
 * the schedule has for it and Filled the ones someone still holds; Away names who said they cannot go and Free the
 * members of the role who could still step in.
 */
type ServiceGap struct {
	MinistryID    int64
	MinistryName  string
	WhatsAppGroup string
	ServiceID     int64
	Date          string
	StartTime     string
	RoleID        int64
	RoleName      string
	Slots         int
	Filled        int
	Minimum       int
	Away          []string
	Free          []string
}

/**
 * NOTE: Roles below their minimum in services dated in [from, to], ordered by date, time and role order. A declined
 * place counts as empty: the person stays in it only until someone covers.
 */
func (s *Store) ServiceGaps(ctx context.Context, from, to string) ([]ServiceGap, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.name, coalesce(m.whatsapp_group, ''), sv.id, a.service_date, sv.start_time,
		       r.id, r.name, r.min_slots, a.person_id IS NOT NULL AND a.status != 'declined',
		       CASE WHEN a.status = 'declined' THEN coalesce(p.name, '') ELSE '' END
		FROM assignments a
		JOIN schedules sc ON sc.id = a.schedule_id
		JOIN ministries m ON m.id = sc.ministry_id
		JOIN services sv ON sv.id = a.service_id
		JOIN roles r ON r.id = a.role_id
		LEFT JOIN people p ON p.id = a.person_id
		WHERE sc.status = 'published' AND m.archived_at IS NULL AND a.service_date BETWEEN ? AND ?
		ORDER BY a.service_date, sv.start_time, m.id, r.position, a.slot`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all []ServiceGap
	for rows.Next() {
		var g ServiceGap
		var minimum *int
		var filled bool
		var away string
		if err := rows.Scan(&g.MinistryID, &g.MinistryName, &g.WhatsAppGroup, &g.ServiceID, &g.Date, &g.StartTime,
			&g.RoleID, &g.RoleName, &minimum, &filled, &away); err != nil {
			return nil, err
		}
		n := len(all)
		if n == 0 || !sameGap(all[n-1], g) {
			g.Minimum = -1
			if minimum != nil {
				g.Minimum = *minimum
			}
			all = append(all, g)
			n++
		}
		last := &all[n-1]
		last.Slots++
		if filled {
			last.Filled++
		}
		if away != "" {
			last.Away = append(last.Away, away)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var gaps []ServiceGap
	for _, g := range all {
		if g.Minimum < 0 || g.Minimum > g.Slots {
			g.Minimum = g.Slots
		}
		if g.Filled >= g.Minimum {
			continue
		}
		if g.Free, err = s.freeForRole(ctx, g); err != nil {
			return nil, err
		}
		gaps = append(gaps, g)
	}
	return gaps, nil
}

func sameGap(a, b ServiceGap) bool {
	return a.MinistryID == b.MinistryID && a.ServiceID == b.ServiceID && a.Date == b.Date && a.RoleID == b.RoleID
}

/**
 * NOTE: Members who hold the role and could still step in: not in that service already (a declined place still
 * counts as theirs), not away that day, and not the absent person of a swap for it.
 */
func (s *Store) freeForRole(ctx context.Context, g ServiceGap) ([]string, error) {
	day, _ := time.Parse("2006-01-02", g.Date)
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.name FROM people p
		JOIN member_roles mr ON mr.person_id = p.id AND mr.role_id = ? AND mr.ministry_id = ?
		WHERE p.archived_at IS NULL
		  AND p.id NOT IN (
		      SELECT a.person_id FROM assignments a
		      WHERE a.service_id = ? AND a.service_date = ? AND a.person_id IS NOT NULL)
		  AND p.id NOT IN (
		      SELECT u.person_id FROM unavailabilities u WHERE u.start_date <= ? AND u.end_date >= ?
		        AND (u.repeat = 'none' OR (u.weekday = ? AND (u.repeat = 'weekly' OR u.week_of_month = ?))))
		  AND p.id NOT IN (
		      SELECT cr.absent_person_id FROM cover_requests cr
		      JOIN assignments a ON a.id = cr.assignment_id
		      WHERE a.service_id = ? AND a.service_date = ?)
		ORDER BY p.name`,
		g.RoleID, g.MinistryID, g.ServiceID, g.Date, g.Date, g.Date, int(day.Weekday()), nthWeekdayOfMonth(g.Date),
		g.ServiceID, g.Date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

/**
 * NOTE: A place a volunteer took. Absent is who had said they could not go, nil when the place was empty, and Waiting
 * the people still invited to cover it.
 */
type FilledGap struct {
	Assignment AssignmentDetail
	Absent     *Person
	Waiting    []Person
}

const awayOn = `
	SELECT 1 FROM unavailabilities u WHERE u.person_id = ? AND u.start_date <= ? AND u.end_date >= ?
	  AND (u.repeat = 'none' OR (u.weekday = ? AND (u.repeat = 'weekly' OR u.week_of_month = ?)))`

/**
 * INVARIANT: The first volunteer wins each place. Checking the person, taking an empty place of the role (or else one
 * somebody declined) and closing its swap happen in one transaction, so two volunteers never get the same place and
 * nobody serves twice in one service.
 */
func (s *Store) FillGap(ctx context.Context, ministryID, serviceID int64, date string, roleID, personID int64, now time.Time) (FilledGap, error) {
	day, _ := time.Parse("2006-01-02", date)
	var assignmentID, requestID int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var member, busy, away bool
		if err := tx.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM ministry_members WHERE ministry_id = ? AND person_id = ?)", ministryID, personID).Scan(&member); err != nil {
			return err
		}
		if !member {
			return ErrNotMember
		}
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM assignments WHERE service_id = ? AND service_date = ? AND person_id = ?)`,
			serviceID, date, personID).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return ErrAlreadyServing
		}
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS ("+awayOn+")",
			personID, date, date, int(day.Weekday()), nthWeekdayOfMonth(date)).Scan(&away); err != nil {
			return err
		}
		if away {
			return ErrAway
		}
		var holder sql.NullInt64
		err := tx.QueryRowContext(ctx, `
			SELECT a.id, a.person_id FROM assignments a JOIN schedules sc ON sc.id = a.schedule_id
			WHERE sc.ministry_id = ? AND sc.status = 'published' AND a.service_id = ? AND a.service_date = ? AND a.role_id = ?
			  AND (a.person_id IS NULL OR a.status = 'declined')
			ORDER BY a.person_id IS NOT NULL, a.slot LIMIT 1`, ministryID, serviceID, date, roleID).Scan(&assignmentID, &holder)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNothingMissing
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE assignments SET person_id = ?, status = 'confirmed', responded_at = ? WHERE id = ?", personID, formatTime(now), assignmentID); err != nil {
			return err
		}
		if !holder.Valid {
			return nil
		}
		err = tx.QueryRowContext(ctx, `
			UPDATE cover_requests SET status = 'covered', covered_by = ?, resolved_at = ?
			WHERE id = (SELECT id FROM cover_requests WHERE assignment_id = ? AND status IN ('open', 'uncovered')
			            ORDER BY created_at DESC LIMIT 1)
			RETURNING id`, personID, formatTime(now), assignmentID).Scan(&requestID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	})
	if err != nil {
		return FilledGap{}, err
	}
	if requestID != 0 {
		result, err := s.coverResult(ctx, requestID)
		if err != nil {
			return FilledGap{}, err
		}
		return FilledGap{Assignment: result.Assignment, Absent: &result.Absent, Waiting: result.Waiting}, nil
	}
	d, err := s.AssignmentDetail(ctx, assignmentID)
	return FilledGap{Assignment: d}, err
}
