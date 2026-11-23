package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"louvadeira/internal/schedule"
)

const (
	AssignmentScheduled = "scheduled"
	AssignmentConfirmed = "confirmed"
	AssignmentDeclined  = "declined"

	PollConfirm = "confirm"
	PollCover   = "cover"
	PollTest    = "test"

	CoverOpen      = "open"
	CoverCovered   = "covered"
	CoverUncovered = "uncovered"
)

var (
	ErrCoverClosed    = errors.New("a troca já foi resolvida")
	ErrAlreadyServing = errors.New("a pessoa já está nesse culto")
)

/**
 * NOTE: Everything the messages need about one person in one role at one service of a published schedule.
 */
type AssignmentDetail struct {
	AssignmentID  int64
	MinistryID    int64
	MinistryName  string
	WhatsAppGroup string
	RotationUnit  RotationUnit
	ServiceID     int64
	ServiceName   string
	Date          string
	StartTime     string
	Communion     bool
	RoleID        int64
	RoleName      string
	RoleFixed     bool
	PersonID      int64
	PersonName    string
	Phone         string
	Status        string
}

const assignmentDetailQuery = `
	SELECT a.id, m.id, m.name, coalesce(m.whatsapp_group, ''), m.rotation_unit,
	       sv.id, sv.name, a.service_date, sv.start_time, sv.communion_week,
	       r.id, r.name, coalesce(mr.fixed, 0),
	       coalesce(a.person_id, 0), coalesce(p.name, ''), coalesce(p.phone, ''), a.status
	FROM assignments a
	JOIN schedules sc ON sc.id = a.schedule_id
	JOIN ministries m ON m.id = sc.ministry_id
	JOIN services sv ON sv.id = a.service_id
	JOIN roles r ON r.id = a.role_id
	LEFT JOIN people p ON p.id = a.person_id
	LEFT JOIN member_roles mr ON mr.person_id = a.person_id AND mr.role_id = a.role_id`

func scanAssignmentDetail(row interface{ Scan(...any) error }) (AssignmentDetail, error) {
	var d AssignmentDetail
	var communionWeek sql.NullInt64
	err := row.Scan(&d.AssignmentID, &d.MinistryID, &d.MinistryName, &d.WhatsAppGroup, &d.RotationUnit,
		&d.ServiceID, &d.ServiceName, &d.Date, &d.StartTime, &communionWeek,
		&d.RoleID, &d.RoleName, &d.RoleFixed,
		&d.PersonID, &d.PersonName, &d.Phone, &d.Status)
	d.Communion = communionWeek.Valid && nthWeekdayOfMonth(d.Date) == int(communionWeek.Int64)
	return d, err
}

func (s *Store) AssignmentDetail(ctx context.Context, assignmentID int64) (AssignmentDetail, error) {
	d, err := scanAssignmentDetail(s.db.QueryRowContext(ctx, assignmentDetailQuery+" WHERE a.id = ?", assignmentID))
	return d, notFoundIfNoRows(err)
}

/**
 * NOTE: Filled slots of published schedules with a service date in [from, to], ordered by date and time.
 */
func (s *Store) UpcomingAssignments(ctx context.Context, from, to string) ([]AssignmentDetail, error) {
	rows, err := s.db.QueryContext(ctx, assignmentDetailQuery+`
		WHERE sc.status = 'published' AND a.person_id IS NOT NULL AND a.service_date BETWEEN ? AND ?
		ORDER BY a.service_date, sv.start_time, r.position, a.slot`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []AssignmentDetail
	for rows.Next() {
		d, err := scanAssignmentDetail(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

/**
 * NOTE: Slots a person still holds in published schedules with a service date in [from, to], leaving out the ones
 * they already said they cannot serve.
 */
func (s *Store) PersonAssignments(ctx context.Context, personID int64, from, to string) ([]AssignmentDetail, error) {
	rows, err := s.db.QueryContext(ctx, assignmentDetailQuery+`
		WHERE sc.status = 'published' AND a.person_id = ? AND a.status != 'declined' AND a.service_date BETWEEN ? AND ?
		ORDER BY a.service_date, sv.start_time, r.position, a.slot`, personID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []AssignmentDetail
	for rows.Next() {
		d, err := scanAssignmentDetail(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

/**
 * NOTE: Returns true only the first time a key is marked, so callers send each scheduled message once.
 */
func (s *Store) MarkNotified(ctx context.Context, key string, now time.Time) (bool, error) {
	result, err := s.db.ExecContext(ctx, "INSERT INTO notifications (key, sent_at) VALUES (?, ?) ON CONFLICT DO NOTHING", key, formatTime(now))
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (s *Store) NotifiedAt(ctx context.Context, key string) (time.Time, bool) {
	var sentAt string
	if err := s.db.QueryRowContext(ctx, "SELECT sent_at FROM notifications WHERE key = ?", key).Scan(&sentAt); err != nil {
		return time.Time{}, false
	}
	t, err := time.Parse(timeLayout, sentAt)
	return t, err == nil
}

func (s *Store) UnmarkNotified(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM notifications WHERE key = ?", key)
	return err
}

func (s *Store) NotificationKeys(ctx context.Context, prefix string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key FROM notifications WHERE substr(key, 1, ?) = ? ORDER BY sent_at", len(prefix), prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

/**
 * NOTE: ClaimNotification records a scheduled message as on its way, unless it was already recorded, and reports
 * whether this call recorded it. SettleNotification marks it sent once WhatsApp takes it; UnmarkNotified drops it
 * when the send fails.
 */
func (s *Store) ClaimNotification(ctx context.Context, key string, now time.Time) (bool, error) {
	result, err := s.db.ExecContext(ctx, "INSERT INTO notifications (key, sent_at, sending) VALUES (?, ?, 1) ON CONFLICT DO NOTHING", key, formatTime(now))
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (s *Store) SettleNotification(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE notifications SET sending = 0 WHERE key = ?", key)
	return err
}

/**
 * NOTE: DropUnsentClaims forgets the messages a restart or a power cut interrupted on their way out, so the next tick
 * sends them. It must run before anything sends, since a claim in flight looks the same.
 */
func (s *Store) DropUnsentClaims(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM notifications WHERE sending = 1")
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

/**
 * CONSTRAINT: Only slots still held by the person and not yet declined change status, so a late vote cannot touch a
 * slot someone else already covered.
 */
func (s *Store) SetAssignmentsStatus(ctx context.Context, personID int64, assignmentIDs []int64, status string, now time.Time) ([]int64, error) {
	var changed []int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		for _, id := range assignmentIDs {
			result, err := tx.ExecContext(ctx, `
				UPDATE assignments SET status = ?, responded_at = ?
				WHERE id = ? AND person_id = ? AND status != 'declined' AND status != ?`, status, formatTime(now), id, personID, status)
			if err != nil {
				return err
			}
			if n, _ := result.RowsAffected(); n == 1 {
				changed = append(changed, id)
			}
		}
		return nil
	})
	return changed, err
}

type PollOption struct {
	Label       string  `json:"label"`
	Action      string  `json:"action"`
	Assignments []int64 `json:"assignments,omitempty"`
	Request     int64   `json:"request,omitempty"`
	// NOTE: Requests is set on an option that answers several swaps at once ("não posso em nenhum").
	Requests []int64 `json:"requests,omitempty"`
	// NOTE: Ranges is set on an option that registers an absence the person announced by message.
	Ranges []DateRange `json:"ranges,omitempty"`
	// NOTE: Ministry and Month identify the schedule an availability or approval poll is about.
	Ministry int64  `json:"ministry,omitempty"`
	Month    string `json:"month,omitempty"`
	// NOTE: Service, Date and Time identify the service a poll about missing people is about; Role, the role offered.
	Service int64  `json:"service,omitempty"`
	Date    string `json:"date,omitempty"`
	Time    string `json:"time,omitempty"`
	Role    int64  `json:"role,omitempty"`
}

type DateRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Poll struct {
	MessageID string
	Kind      string
	PersonID  int64
	Options   []PollOption
}

func (s *Store) SavePoll(ctx context.Context, poll Poll, now time.Time) error {
	options, err := json.Marshal(poll.Options)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO polls (message_id, kind, person_id, options, created_at) VALUES (?, ?, ?, ?, ?)",
		poll.MessageID, poll.Kind, sql.NullInt64{Int64: poll.PersonID, Valid: poll.PersonID != 0}, string(options), formatTime(now))
	return err
}

/**
 * NOTE: The latest cover polls sent to a person, newest first, to fold a new swap into an invitation they already
 * have instead of sending another one.
 */
func (s *Store) PersonCoverPolls(ctx context.Context, personID int64) ([]Poll, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT message_id, kind, person_id, options FROM polls
		WHERE person_id = ? AND kind = 'cover' ORDER BY created_at DESC, rowid DESC LIMIT 20`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var polls []Poll
	for rows.Next() {
		var poll Poll
		var options string
		if err := rows.Scan(&poll.MessageID, &poll.Kind, &poll.PersonID, &options); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(options), &poll.Options); err != nil {
			return nil, err
		}
		polls = append(polls, poll)
	}
	return polls, rows.Err()
}

/**
 * NOTE: Polls sent to a person since a moment, newest first.
 */
func (s *Store) PersonRecentPolls(ctx context.Context, personID int64, since time.Time) ([]Poll, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT message_id, kind, person_id, options FROM polls
		WHERE person_id = ? AND created_at >= ? ORDER BY created_at DESC, rowid DESC LIMIT 10`, personID, formatTime(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var polls []Poll
	for rows.Next() {
		var poll Poll
		var options string
		if err := rows.Scan(&poll.MessageID, &poll.Kind, &poll.PersonID, &options); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(options), &poll.Options); err != nil {
			return nil, err
		}
		polls = append(polls, poll)
	}
	return polls, rows.Err()
}

func (s *Store) UpdatePollOptions(ctx context.Context, messageID string, options []PollOption) error {
	encoded, err := json.Marshal(options)
	if err != nil {
		return err
	}
	return requireAffected(s.db.ExecContext(ctx, "UPDATE polls SET options = ? WHERE message_id = ?", string(encoded), messageID))
}

/**
 * NOTE: How a person answered an invitation: "yes", "no", or "" while they have not answered.
 */
func (s *Store) InviteAnswer(ctx context.Context, requestID, personID int64) string {
	var answer sql.NullString
	_ = s.db.QueryRowContext(ctx, "SELECT answer FROM cover_invites WHERE request_id = ? AND person_id = ?", requestID, personID).Scan(&answer)
	return answer.String
}

func (s *Store) Poll(ctx context.Context, messageID string) (Poll, error) {
	var poll Poll
	var options string
	var person sql.NullInt64
	err := s.db.QueryRowContext(ctx, "SELECT message_id, kind, person_id, options FROM polls WHERE message_id = ?", messageID).
		Scan(&poll.MessageID, &poll.Kind, &person, &options)
	if err != nil {
		return Poll{}, notFoundIfNoRows(err)
	}
	poll.PersonID = person.Int64
	return poll, json.Unmarshal([]byte(options), &poll.Options)
}

/**
 * NOTE: A vote waiting to count. Selected holds the SHA-256 of each chosen option, as WhatsApp sends them.
 */
type PendingVote struct {
	PollID   string
	Voter    string
	Selected [][]byte
	FromMe   bool
}

func (s *Store) SavePendingVote(ctx context.Context, vote PendingVote, now time.Time) error {
	selected, err := json.Marshal(vote.Selected)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO pending_votes (poll_id, voter, selected, from_me, received_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (poll_id, voter) DO UPDATE SET selected = excluded.selected, from_me = excluded.from_me, received_at = excluded.received_at`,
		vote.PollID, vote.Voter, string(selected), boolInt(vote.FromMe), formatTime(now))
	return err
}

func (s *Store) PendingVotes(ctx context.Context) ([]PendingVote, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT poll_id, voter, selected, from_me FROM pending_votes ORDER BY received_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var votes []PendingVote
	for rows.Next() {
		var vote PendingVote
		var selected string
		if err := rows.Scan(&vote.PollID, &vote.Voter, &selected, &vote.FromMe); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(selected), &vote.Selected); err != nil {
			return nil, err
		}
		votes = append(votes, vote)
	}
	return votes, rows.Err()
}

/**
 * NOTE: Removes the vote only if it is still the one that was applied, so a newer answer that arrived meanwhile stays.
 */
func (s *Store) DeletePendingVote(ctx context.Context, vote PendingVote) error {
	selected, err := json.Marshal(vote.Selected)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "DELETE FROM pending_votes WHERE poll_id = ? AND voter = ? AND selected = ?",
		vote.PollID, vote.Voter, string(selected))
	return err
}

/**
 * NOTE: Leaders of the ministry who have a phone; when the ministry has no leader, the administrators.
 */
func (s *Store) Leaders(ctx context.Context, ministryID int64) ([]Person, error) {
	query := `SELECT ` + personColumnsP + ` FROM people p
		JOIN ministry_members mm ON mm.person_id = p.id
		WHERE mm.ministry_id = ? AND mm.is_leader = 1 AND p.phone IS NOT NULL AND p.archived_at IS NULL
		ORDER BY p.name`
	leaders, err := s.queryPeople(ctx, query, ministryID)
	if err != nil || len(leaders) > 0 {
		return leaders, err
	}
	return s.queryPeople(ctx, `SELECT `+personColumns+` FROM people
		WHERE is_admin = 1 AND phone IS NOT NULL AND archived_at IS NULL ORDER BY name`)
}

func (s *Store) queryPeople(ctx context.Context, query string, args ...any) ([]Person, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var people []Person
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		people = append(people, p)
	}
	return people, rows.Err()
}

/**
 * NOTE: Who can cover a slot: members of the ministry with the same role and a phone, who are not already serving in
 * that service, did not decline it, and are not unavailable on that date. Least busy in the last weeks first.
 */
func (s *Store) CoverCandidates(ctx context.Context, assignmentID int64) ([]Person, error) {
	d, err := s.AssignmentDetail(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	since, _ := time.Parse("2006-01-02", d.Date)
	weekday, week := int(since.Weekday()), nthWeekdayOfMonth(d.Date)
	return s.queryPeople(ctx, `
		SELECT `+personColumnsP+` FROM people p
		JOIN member_roles mr ON mr.person_id = p.id AND mr.role_id = ?
		WHERE p.phone IS NOT NULL AND p.archived_at IS NULL AND p.id != ?
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
		ORDER BY (
		      SELECT count(*) FROM assignments a2 JOIN schedules s2 ON s2.id = a2.schedule_id
		      WHERE a2.person_id = p.id AND s2.status = 'published'
		        AND a2.service_date BETWEEN ? AND ?), p.name`,
		d.RoleID, d.PersonID, d.ServiceID, d.Date, d.Date, d.Date, weekday, week, d.ServiceID, d.Date,
		since.AddDate(0, 0, -7*historyWeeks).Format("2006-01-02"), d.Date)
}

/**
 * NOTE: Marks the slot as declined and opens a cover request for it. The person stays in the slot until someone
 * accepts, so the schedule never shows a gap nobody is handling.
 */
func (s *Store) OpenCoverRequest(ctx context.Context, assignmentID int64, late bool, now time.Time) (int64, error) {
	var id int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var person sql.NullInt64
		var status string
		if err := tx.QueryRowContext(ctx, "SELECT person_id, status FROM assignments WHERE id = ?", assignmentID).Scan(&person, &status); err != nil {
			return notFoundIfNoRows(err)
		}
		if !person.Valid {
			return invalid("essa vaga está vazia")
		}
		var open bool
		if err := tx.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM cover_requests WHERE assignment_id = ? AND status = 'open')", assignmentID).Scan(&open); err != nil {
			return err
		}
		if open {
			return invalid("já existe uma troca em andamento para essa vaga")
		}
		if _, err := tx.ExecContext(ctx, "UPDATE assignments SET status = 'declined', responded_at = ? WHERE id = ?", formatTime(now), assignmentID); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, `
			INSERT INTO cover_requests (assignment_id, absent_person_id, late, created_at) VALUES (?, ?, ?, ?)
			RETURNING id`, assignmentID, person.Int64, boolInt(late), formatTime(now)).Scan(&id)
	})
	return id, err
}

func (s *Store) AddCoverInvite(ctx context.Context, requestID, personID int64) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO cover_invites (request_id, person_id) VALUES (?, ?) ON CONFLICT DO NOTHING", requestID, personID)
	return err
}

type OpenRequest struct {
	ID           int64
	AssignmentID int64
	Late         bool
}

/**
 * NOTE: Swaps still open with nobody invited, because every invitation failed to send; opened before the given time.
 */
func (s *Store) UninvitedRequests(ctx context.Context, before time.Time) ([]OpenRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cr.id, cr.assignment_id, cr.late FROM cover_requests cr
		WHERE cr.status = 'open' AND cr.created_at <= ?
		  AND NOT EXISTS (SELECT 1 FROM cover_invites ci WHERE ci.request_id = cr.id)
		ORDER BY cr.created_at`, formatTime(before))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []OpenRequest
	for rows.Next() {
		var r OpenRequest
		if err := rows.Scan(&r.ID, &r.AssignmentID, &r.Late); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

func (s *Store) CloseCoverRequest(ctx context.Context, requestID int64, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE cover_requests SET status = 'uncovered', resolved_at = ? WHERE id = ? AND status = 'open'", formatTime(now), requestID)
	return err
}

type CoverResult struct {
	Request    CoverRequest
	Assignment AssignmentDetail
	Absent     Person
	Waiting    []Person
}

/**
 * INVARIANT: The first acceptance wins. Accepting moves the slot to the new person (already confirmed) in the same
 * transaction that closes the request, so two people can never both get it, and nobody takes two slots of one
 * service when two people of the same service are away.
 */
func (s *Store) AcceptCover(ctx context.Context, requestID, personID int64, now time.Time) (CoverResult, error) {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var busy bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM cover_requests cr
				JOIN assignments target ON target.id = cr.assignment_id
				JOIN assignments other ON other.service_id = target.service_id AND other.service_date = target.service_date
				  AND other.id != target.id
				WHERE cr.id = ? AND other.person_id = ?)`, requestID, personID).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return ErrAlreadyServing
		}
		var assignmentID int64
		err := tx.QueryRowContext(ctx, `
			UPDATE cover_requests SET status = 'covered', covered_by = ?, resolved_at = ?
			WHERE id = ? AND status = 'open'
			  AND EXISTS (SELECT 1 FROM cover_invites WHERE request_id = ? AND person_id = ?)
			RETURNING assignment_id`, personID, formatTime(now), requestID, requestID, personID).Scan(&assignmentID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCoverClosed
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE cover_invites SET answer = 'yes', answered_at = ? WHERE request_id = ? AND person_id = ?", formatTime(now), requestID, personID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			"UPDATE assignments SET person_id = ?, status = 'confirmed', responded_at = ? WHERE id = ?", personID, formatTime(now), assignmentID)
		return err
	})
	if err != nil {
		return CoverResult{}, err
	}
	return s.coverResult(ctx, requestID)
}

/**
 * NOTE: Records a refusal and reports whether every invited person has now refused.
 */
func (s *Store) RefuseCover(ctx context.Context, requestID, personID int64, now time.Time) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE cover_invites SET answer = 'no', answered_at = ?
		WHERE request_id = ? AND person_id = ? AND answer IS NULL
		  AND EXISTS (SELECT 1 FROM cover_requests WHERE id = ? AND status = 'open')`,
		formatTime(now), requestID, personID, requestID)
	if err != nil {
		return false, err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return false, ErrCoverClosed
	}
	var pending int
	err = s.db.QueryRowContext(ctx,
		"SELECT count(*) FROM cover_invites WHERE request_id = ? AND (answer IS NULL OR answer = 'yes')", requestID).Scan(&pending)
	return pending == 0, err
}

/**
 * NOTE: Someone who declined and changes their mind gets the slot back while nobody has accepted yet. The request
 * is closed as covered by the same person, so the history shows they came back.
 */
func (s *Store) WithdrawDecline(ctx context.Context, assignmentID, personID int64, now time.Time) (CoverResult, error) {
	var requestID int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			UPDATE cover_requests SET status = 'covered', covered_by = ?, resolved_at = ?
			WHERE assignment_id = ? AND absent_person_id = ? AND status IN ('open', 'uncovered')
			  AND EXISTS (SELECT 1 FROM assignments WHERE id = ? AND person_id = ?)
			RETURNING id`, personID, formatTime(now), assignmentID, personID, assignmentID, personID).Scan(&requestID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCoverClosed
		}
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			"UPDATE assignments SET status = 'confirmed', responded_at = ? WHERE id = ?", formatTime(now), assignmentID)
		return err
	})
	if err != nil {
		return CoverResult{}, err
	}
	return s.coverResult(ctx, requestID)
}

type CoverRequest struct {
	ID           int64        `json:"id"`
	AssignmentID int64        `json:"assignmentId"`
	Date         string       `json:"date"`
	ServiceName  string       `json:"serviceName"`
	StartTime    string       `json:"startTime"`
	RoleName     string       `json:"roleName"`
	AbsentName   string       `json:"absentName"`
	Status       string       `json:"status"`
	Late         bool         `json:"late"`
	CoveredBy    string       `json:"coveredBy"`
	CreatedAt    string       `json:"createdAt"`
	Invites      []CoverGuest `json:"invites"`
}

type CoverGuest struct {
	PersonID int64  `json:"personId"`
	Name     string `json:"name"`
	Answer   string `json:"answer"`
}

const coverRequestQuery = `
	SELECT cr.id, cr.assignment_id, a.service_date, sv.name, sv.start_time, r.name, absent.name, cr.status, cr.late,
	       coalesce(cover.name, ''), cr.created_at
	FROM cover_requests cr
	JOIN assignments a ON a.id = cr.assignment_id
	JOIN schedules sc ON sc.id = a.schedule_id
	JOIN services sv ON sv.id = a.service_id
	JOIN roles r ON r.id = a.role_id
	JOIN people absent ON absent.id = cr.absent_person_id
	LEFT JOIN people cover ON cover.id = cr.covered_by`

func (s *Store) coverRequests(ctx context.Context, where string, args ...any) ([]CoverRequest, error) {
	rows, err := s.db.QueryContext(ctx, coverRequestQuery+" WHERE "+where+" ORDER BY a.service_date, cr.created_at", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []CoverRequest{}
	for rows.Next() {
		var c CoverRequest
		if err := rows.Scan(&c.ID, &c.AssignmentID, &c.Date, &c.ServiceName, &c.StartTime, &c.RoleName, &c.AbsentName,
			&c.Status, &c.Late, &c.CoveredBy, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range list {
		guests, err := s.coverGuests(ctx, list[i].ID)
		if err != nil {
			return nil, err
		}
		list[i].Invites = guests
	}
	return list, nil
}

func (s *Store) coverGuests(ctx context.Context, requestID int64) ([]CoverGuest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.name, coalesce(ci.answer, '') FROM cover_invites ci JOIN people p ON p.id = ci.person_id
		WHERE ci.request_id = ? ORDER BY p.name`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	guests := []CoverGuest{}
	for rows.Next() {
		var g CoverGuest
		if err := rows.Scan(&g.PersonID, &g.Name, &g.Answer); err != nil {
			return nil, err
		}
		guests = append(guests, g)
	}
	return guests, rows.Err()
}

func (s *Store) CoverRequestsForSchedule(ctx context.Context, ministryID int64, month string) ([]CoverRequest, error) {
	return s.coverRequests(ctx, "sc.ministry_id = ? AND sc.month = ?", ministryID, month)
}

func (s *Store) coverResult(ctx context.Context, requestID int64) (CoverResult, error) {
	requests, err := s.coverRequests(ctx, "cr.id = ?", requestID)
	if err != nil {
		return CoverResult{}, err
	}
	if len(requests) == 0 {
		return CoverResult{}, ErrNotFound
	}
	result := CoverResult{Request: requests[0]}
	if result.Assignment, err = s.AssignmentDetail(ctx, result.Request.AssignmentID); err != nil {
		return CoverResult{}, err
	}
	var absentID int64
	if err := s.db.QueryRowContext(ctx, "SELECT absent_person_id FROM cover_requests WHERE id = ?", requestID).Scan(&absentID); err != nil {
		return CoverResult{}, err
	}
	if result.Absent, err = scanPerson(s.db.QueryRowContext(ctx, "SELECT "+personColumns+" FROM people WHERE id = ?", absentID)); err != nil {
		return CoverResult{}, err
	}
	result.Waiting, err = s.queryPeople(ctx, `
		SELECT `+personColumnsP+` FROM cover_invites ci JOIN people p ON p.id = ci.person_id
		WHERE ci.request_id = ? AND ci.answer IS NULL`, requestID)
	return result, err
}

func (s *Store) CoverRequestResult(ctx context.Context, requestID int64) (CoverResult, error) {
	return s.coverResult(ctx, requestID)
}

/**
 * NOTE: The group key a slot belongs to for confirmations: its week (Monday) when the ministry rotates weekly, the
 * service itself otherwise.
 */
func GroupKey(d AssignmentDetail) string {
	if d.RotationUnit == RotationService {
		return d.Date + "#" + d.StartTime
	}
	return schedule.WeekStart(d.Date)
}
