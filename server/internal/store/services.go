package store

import (
	"context"
	"time"
)

type Service struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Weekday   int    `json:"weekday"`
	StartTime string `json:"startTime"`
	Active    bool   `json:"active"`
}

/**
 * NOTE: Weekday follows Go's time.Weekday (0 is Sunday) and StartTime is local church time as "HH:MM".
 * New services always start active; Active only matters on update.
 */
type ServiceInput struct {
	Name      string `json:"name"`
	Weekday   int    `json:"weekday"`
	StartTime string `json:"startTime"`
	Active    bool   `json:"active"`
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
	return ServiceInput{Name: name, Weekday: in.Weekday, StartTime: start.Format("15:04"), Active: in.Active}, nil
}

const serviceColumns = "id, name, weekday, start_time, active"

func scanService(row interface{ Scan(...any) error }) (Service, error) {
	var s Service
	err := row.Scan(&s.ID, &s.Name, &s.Weekday, &s.StartTime, &s.Active)
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
		"INSERT INTO services (name, weekday, start_time) VALUES (?, ?, ?) RETURNING "+serviceColumns,
		in.Name, in.Weekday, in.StartTime))
}

func (s *Store) UpdateService(ctx context.Context, id int64, in ServiceInput) (Service, error) {
	in, err := in.clean()
	if err != nil {
		return Service{}, err
	}
	sv, err := scanService(s.db.QueryRowContext(ctx,
		"UPDATE services SET name = ?, weekday = ?, start_time = ?, active = ? WHERE id = ? RETURNING "+serviceColumns,
		in.Name, in.Weekday, in.StartTime, boolInt(in.Active), id))
	return sv, notFoundIfNoRows(err)
}
