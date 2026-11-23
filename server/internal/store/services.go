package store

import (
	"context"
	"database/sql"
	"time"
)

type Service struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Weekday       int    `json:"weekday"`
	StartTime     string `json:"startTime"`
	Active        bool   `json:"active"`
	CommunionWeek *int   `json:"communionWeek"`
}

/**
 * NOTE: Weekday follows Go's time.Weekday (0 is Sunday) and StartTime is local church time as "HH:MM".
 * New services always start active; Active only matters on update. CommunionWeek marks the Lord's Supper on the
 * nth occurrence of the weekday in each month (3 = third Sunday), or nil when there is none.
 */
type ServiceInput struct {
	Name          string `json:"name"`
	Weekday       int    `json:"weekday"`
	StartTime     string `json:"startTime"`
	Active        bool   `json:"active"`
	CommunionWeek *int   `json:"communionWeek"`
}

func (in ServiceInput) clean() (ServiceInput, error) {
	name, err := cleanName(in.Name, "Nome do culto")
	if err != nil {
		return ServiceInput{}, err
	}
	if in.Weekday < 0 || in.Weekday > 6 {
		return ServiceInput{}, invalid("dia da semana inválido")
	}
	start, err := time.Parse("15:04", in.StartTime)
	if err != nil {
		return ServiceInput{}, invalid("horário inválido: use HH:MM, como 19:30")
	}
	if in.CommunionWeek != nil && (*in.CommunionWeek < 1 || *in.CommunionWeek > 4) {
		return ServiceInput{}, invalid("Santa Ceia: escolha do 1º ao 4º")
	}
	return ServiceInput{Name: name, Weekday: in.Weekday, StartTime: start.Format("15:04"), Active: in.Active, CommunionWeek: in.CommunionWeek}, nil
}

const serviceColumns = "id, name, weekday, start_time, active, communion_week"

func scanService(row interface{ Scan(...any) error }) (Service, error) {
	var s Service
	err := row.Scan(&s.ID, &s.Name, &s.Weekday, &s.StartTime, &s.Active, &s.CommunionWeek)
	return s, err
}

func (s *Store) ListServices(ctx context.Context) ([]Service, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+serviceColumns+" FROM services ORDER BY active DESC, weekday, start_time")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	services := []Service{}
	for rows.Next() {
		sv, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, sv)
	}
	return services, rows.Err()
}

func (s *Store) CreateService(ctx context.Context, in ServiceInput) (Service, error) {
	in, err := in.clean()
	if err != nil {
		return Service{}, err
	}
	return scanService(s.db.QueryRowContext(ctx,
		"INSERT INTO services (name, weekday, start_time, communion_week) VALUES (?, ?, ?, ?) RETURNING "+serviceColumns,
		in.Name, in.Weekday, in.StartTime, in.CommunionWeek))
}

/**
 * CONSTRAINT: A service with people in a published schedule from today on keeps its weekday and stays active. Its
 * dates are already in those schedules, so another weekday would put them on the wrong day, and an inactive service
 * would still be asked about. A new start time is fine: the dates stay right.
 */
func (s *Store) UpdateService(ctx context.Context, id int64, in ServiceInput, today string) (Service, error) {
	in, err := in.clean()
	if err != nil {
		return Service{}, err
	}
	var sv Service
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		current, err := scanService(tx.QueryRowContext(ctx, "SELECT "+serviceColumns+" FROM services WHERE id = ?", id))
		if err != nil {
			return notFoundIfNoRows(err)
		}
		if current.Weekday != in.Weekday || (current.Active && !in.Active) {
			var scheduled bool
			if err := tx.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM assignments a JOIN schedules sc ON sc.id = a.schedule_id
					WHERE a.service_id = ? AND sc.status = 'published' AND a.service_date >= ? AND a.person_id IS NOT NULL)`, id, today).Scan(&scheduled); err != nil {
				return err
			}
			if scheduled {
				return invalid("esse culto tem gente escalada daqui para frente; tire as pessoas dele nas escalas publicadas antes de mudar o dia ou desativar")
			}
		}
		sv, err = scanService(tx.QueryRowContext(ctx,
			"UPDATE services SET name = ?, weekday = ?, start_time = ?, active = ?, communion_week = ? WHERE id = ? RETURNING "+serviceColumns,
			in.Name, in.Weekday, in.StartTime, boolInt(in.Active), in.CommunionWeek, id))
		return notFoundIfNoRows(err)
	})
	return sv, err
}

/**
 * NOTE: The Lord's Supper happens in one service a month, so choosing a service clears any other; a nil service means
 * none. A failed choice leaves the previous one in place.
 */
func (s *Store) SetCommunion(ctx context.Context, serviceID *int64, week *int) error {
	if serviceID != nil && (week == nil || *week < 1 || *week > 4) {
		return invalid("escolha a semana da Santa Ceia, de 1 a 4")
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE services SET communion_week = NULL WHERE communion_week IS NOT NULL"); err != nil {
			return err
		}
		if serviceID == nil {
			return nil
		}
		result, err := tx.ExecContext(ctx, "UPDATE services SET communion_week = ? WHERE id = ?", *week, *serviceID)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed == 0 {
			return ErrNotFound
		}
		return nil
	})
}
