package store

import (
	"context"
	"database/sql"
)

type RotationUnit string

const (
	RotationWeek    RotationUnit = "week"
	RotationService RotationUnit = "service"
)

type Ministry struct {
	ID           int64        `json:"id"`
	Name         string       `json:"name"`
	RotationUnit RotationUnit `json:"rotationUnit"`
	MemberCount  int          `json:"memberCount"`
}

type MinistryInput struct {
	Name         string       `json:"name"`
	RotationUnit RotationUnit `json:"rotationUnit"`
}

type Role struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Slots int    `json:"slots"`
}

type RoleInput struct {
	Name  string `json:"name"`
	Slots int    `json:"slots"`
}

type MemberRole struct {
	RoleID int64 `json:"roleId"`
	Fixed  bool  `json:"fixed"`
}

type Member struct {
	PersonID int64        `json:"personId"`
	Name     string       `json:"name"`
	Phone    string       `json:"phone"`
	IsLeader bool         `json:"isLeader"`
	Roles    []MemberRole `json:"roles"`
}

type MemberInput struct {
	IsLeader bool         `json:"isLeader"`
	Roles    []MemberRole `json:"roles"`
}

type MinistryDetail struct {
	Ministry
	Roles      []Role   `json:"roles"`
	Members    []Member `json:"members"`
	ServiceIDs []int64  `json:"serviceIds"`
}

func (in MinistryInput) clean() (MinistryInput, error) {
	name, err := cleanName(in.Name, "Nome do ministério")
	if err != nil {
		return MinistryInput{}, err
	}
	switch in.RotationUnit {
	case "":
		in.RotationUnit = RotationWeek
	case RotationWeek, RotationService:
	default:
		return MinistryInput{}, invalid("rodízio inválido: use por semana ou por culto")
	}
	return MinistryInput{Name: name, RotationUnit: in.RotationUnit}, nil
}

func (in RoleInput) clean() (RoleInput, error) {
	name, err := cleanName(in.Name, "Nome da função")
	if err != nil {
		return RoleInput{}, err
	}
	if in.Slots < 1 || in.Slots > 20 {
		return RoleInput{}, invalid("a função precisa de 1 a 20 vagas por escala")
	}
	return RoleInput{Name: name, Slots: in.Slots}, nil
}

func (s *Store) ListMinistries(ctx context.Context) ([]Ministry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.name, m.rotation_unit, count(mm.person_id)
		FROM ministries m
		LEFT JOIN ministry_members mm ON mm.ministry_id = m.id
		WHERE m.archived_at IS NULL
		GROUP BY m.id
		ORDER BY m.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ministries := []Ministry{}
	for rows.Next() {
		var m Ministry
		if err := rows.Scan(&m.ID, &m.Name, &m.RotationUnit, &m.MemberCount); err != nil {
			return nil, err
		}
		ministries = append(ministries, m)
	}
	return ministries, rows.Err()
}

func (s *Store) CreateMinistry(ctx context.Context, in MinistryInput) (Ministry, error) {
	in, err := in.clean()
	if err != nil {
		return Ministry{}, err
	}
	m := Ministry{Name: in.Name, RotationUnit: in.RotationUnit}
	err = s.db.QueryRowContext(ctx,
		"INSERT INTO ministries (name, rotation_unit) VALUES (?, ?) RETURNING id", in.Name, in.RotationUnit).Scan(&m.ID)
	if isUniqueViolation(err) {
		return Ministry{}, invalid("já existe um ministério com esse nome")
	}
	return m, err
}

func (s *Store) UpdateMinistry(ctx context.Context, id int64, in MinistryInput) error {
	in, err := in.clean()
	if err != nil {
		return err
	}
	err = requireAffected(s.db.ExecContext(ctx,
		"UPDATE ministries SET name = ?, rotation_unit = ? WHERE id = ? AND archived_at IS NULL", in.Name, in.RotationUnit, id))
	if isUniqueViolation(err) {
		return invalid("já existe um ministério com esse nome")
	}
	return err
}

// NOTE: The archived name gets a suffix so a new ministry can reuse it.
func (s *Store) ArchiveMinistry(ctx context.Context, id int64) error {
	return requireAffected(s.db.ExecContext(ctx, `
		UPDATE ministries
		SET archived_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), name = name || ' (arquivado ' || id || ')'
		WHERE id = ? AND archived_at IS NULL`, id))
}

func (s *Store) Ministry(ctx context.Context, id int64) (MinistryDetail, error) {
	var d MinistryDetail
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, rotation_unit FROM ministries WHERE id = ? AND archived_at IS NULL", id,
	).Scan(&d.ID, &d.Name, &d.RotationUnit)
	if err != nil {
		return MinistryDetail{}, notFoundIfNoRows(err)
	}
	if d.Roles, err = s.roles(ctx, id); err != nil {
		return MinistryDetail{}, err
	}
	if d.Members, err = s.members(ctx, id); err != nil {
		return MinistryDetail{}, err
	}
	d.MemberCount = len(d.Members)
	if d.ServiceIDs, err = s.ministryServiceIDs(ctx, id); err != nil {
		return MinistryDetail{}, err
	}
	return d, nil
}

func (s *Store) roles(ctx context.Context, ministryID int64) ([]Role, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, slots FROM roles WHERE ministry_id = ? ORDER BY position, id", ministryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := []Role{}
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Slots); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

func (s *Store) members(ctx context.Context, ministryID int64) ([]Member, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.name, coalesce(p.phone, ''), mm.is_leader, mr.role_id, coalesce(mr.fixed, 0)
		FROM ministry_members mm
		JOIN people p ON p.id = mm.person_id
		LEFT JOIN member_roles mr ON mr.ministry_id = mm.ministry_id AND mr.person_id = mm.person_id
		LEFT JOIN roles r ON r.id = mr.role_id
		WHERE mm.ministry_id = ?
		ORDER BY p.name COLLATE NOCASE, p.id, r.position, r.id`, ministryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []Member{}
	for rows.Next() {
		var m Member
		var roleID sql.NullInt64
		var fixed bool
		if err := rows.Scan(&m.PersonID, &m.Name, &m.Phone, &m.IsLeader, &roleID, &fixed); err != nil {
			return nil, err
		}
		if n := len(members); n == 0 || members[n-1].PersonID != m.PersonID {
			m.Roles = []MemberRole{}
			members = append(members, m)
		}
		if roleID.Valid {
			last := &members[len(members)-1]
			last.Roles = append(last.Roles, MemberRole{RoleID: roleID.Int64, Fixed: fixed})
		}
	}
	return members, rows.Err()
}

func (s *Store) ministryServiceIDs(ctx context.Context, ministryID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ms.service_id FROM ministry_services ms
		JOIN services sv ON sv.id = ms.service_id
		WHERE ms.ministry_id = ?
		ORDER BY sv.weekday, sv.start_time`, ministryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) CreateRole(ctx context.Context, ministryID int64, in RoleInput) (Role, error) {
	in, err := in.clean()
	if err != nil {
		return Role{}, err
	}
	r := Role{Name: in.Name, Slots: in.Slots}
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO roles (ministry_id, name, slots, position)
		SELECT m.id, ?, ?, coalesce((SELECT max(position) + 1 FROM roles WHERE ministry_id = m.id), 0)
		FROM ministries m WHERE m.id = ? AND m.archived_at IS NULL
		RETURNING id`, in.Name, in.Slots, ministryID).Scan(&r.ID)
	if isUniqueViolation(err) {
		return Role{}, invalid("esse ministério já tem uma função com esse nome")
	}
	return r, notFoundIfNoRows(err)
}

func (s *Store) UpdateRole(ctx context.Context, ministryID, roleID int64, in RoleInput) error {
	in, err := in.clean()
	if err != nil {
		return err
	}
	err = requireAffected(s.db.ExecContext(ctx,
		"UPDATE roles SET name = ?, slots = ? WHERE id = ? AND ministry_id = ?", in.Name, in.Slots, roleID, ministryID))
	if isUniqueViolation(err) {
		return invalid("esse ministério já tem uma função com esse nome")
	}
	return err
}

func (s *Store) DeleteRole(ctx context.Context, ministryID, roleID int64) error {
	return requireAffected(s.db.ExecContext(ctx, "DELETE FROM roles WHERE id = ? AND ministry_id = ?", roleID, ministryID))
}

/**
 * NOTE: Adds the person to the ministry or updates them, replacing their roles with exactly the ones given.
 */
func (s *Store) PutMember(ctx context.Context, ministryID, personID int64, in MemberInput) error {
	seen := map[int64]bool{}
	for _, role := range in.Roles {
		if seen[role.RoleID] {
			return invalid("função repetida")
		}
		seen[role.RoleID] = true
	}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var exists bool
		err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM ministries WHERE id = ? AND archived_at IS NULL)
			   AND EXISTS (SELECT 1 FROM people WHERE id = ? AND archived_at IS NULL)`, ministryID, personID).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ministry_members (ministry_id, person_id, is_leader) VALUES (?, ?, ?)
			ON CONFLICT (ministry_id, person_id) DO UPDATE SET is_leader = excluded.is_leader`,
			ministryID, personID, boolInt(in.IsLeader))
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM member_roles WHERE ministry_id = ? AND person_id = ?", ministryID, personID); err != nil {
			return err
		}
		for _, role := range in.Roles {
			_, err := tx.ExecContext(ctx,
				"INSERT INTO member_roles (ministry_id, person_id, role_id, fixed) VALUES (?, ?, ?, ?)",
				ministryID, personID, role.RoleID, boolInt(role.Fixed))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if isForeignKeyViolation(err) {
		return invalid("função não pertence a este ministério")
	}
	return err
}

func (s *Store) RemoveMember(ctx context.Context, ministryID, personID int64) error {
	return requireAffected(s.db.ExecContext(ctx,
		"DELETE FROM ministry_members WHERE ministry_id = ? AND person_id = ?", ministryID, personID))
}

func (s *Store) SetMinistryServices(ctx context.Context, ministryID int64, serviceIDs []int64) error {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM ministries WHERE id = ? AND archived_at IS NULL)", ministryID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM ministry_services WHERE ministry_id = ?", ministryID); err != nil {
			return err
		}
		for _, id := range serviceIDs {
			_, err := tx.ExecContext(ctx,
				"INSERT INTO ministry_services (ministry_id, service_id) VALUES (?, ?) ON CONFLICT DO NOTHING", ministryID, id)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if isForeignKeyViolation(err) {
		return invalid("culto inexistente")
	}
	return err
}
