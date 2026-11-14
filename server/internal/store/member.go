package store

import "context"

/**
 * NOTE: A slot as the member sees it on their own page.
 */
type MemberSlot struct {
	AssignmentID int64  `json:"assignmentId"`
	MinistryID   int64  `json:"ministryId"`
	MinistryName string `json:"ministryName"`
	ServiceID    int64  `json:"serviceId"`
	ServiceName  string `json:"serviceName"`
	Date         string `json:"date"`
	StartTime    string `json:"startTime"`
	RoleName     string `json:"roleName"`
	Status       string `json:"status"`
	CoverOpen    bool   `json:"coverOpen"`
	Communion    bool   `json:"communion"`
}

/**
 * NOTE: Every slot a person holds in published schedules in [from, to], including the ones they said they cannot
 * serve while a substitute is still being found.
 */
func (s *Store) MemberSlots(ctx context.Context, personID int64, from, to string) ([]MemberSlot, error) {
	rows, err := s.db.QueryContext(ctx, assignmentDetailQuery+`
		WHERE sc.status = 'published' AND a.person_id = ? AND a.service_date BETWEEN ? AND ?
		ORDER BY a.service_date, sv.start_time, m.name, r.position, a.slot`, personID, from, to)
	if err != nil {
		return nil, err
	}
	var details []AssignmentDetail
	for rows.Next() {
		d, err := scanAssignmentDetail(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		details = append(details, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slots := make([]MemberSlot, len(details))
	for i, d := range details {
		slots[i] = MemberSlot{
			AssignmentID: d.AssignmentID, MinistryID: d.MinistryID, MinistryName: d.MinistryName,
			ServiceID: d.ServiceID, ServiceName: d.ServiceName, Date: d.Date, StartTime: d.StartTime,
			RoleName: d.RoleName, Status: d.Status, Communion: d.Communion,
		}
		err := s.db.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM cover_requests WHERE assignment_id = ? AND status = 'open')", d.AssignmentID).Scan(&slots[i].CoverOpen)
		if err != nil {
			return nil, err
		}
	}
	return slots, nil
}

/**
 * NOTE: The slot when it belongs to the person in a published schedule; ErrNotFound otherwise, so nobody acts on
 * someone else's slot.
 */
func (s *Store) MemberAssignment(ctx context.Context, personID, assignmentID int64) (AssignmentDetail, error) {
	d, err := scanAssignmentDetail(s.db.QueryRowContext(ctx, assignmentDetailQuery+`
		WHERE a.id = ? AND a.person_id = ? AND sc.status = 'published'`, assignmentID, personID))
	return d, notFoundIfNoRows(err)
}

/**
 * NOTE: The ministries a person serves in, for telling their leaders about something the person did.
 */
func (s *Store) PersonMinistries(ctx context.Context, personID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id FROM ministry_members mm JOIN ministries m ON m.id = mm.ministry_id
		WHERE mm.person_id = ? AND m.archived_at IS NULL ORDER BY m.name`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
