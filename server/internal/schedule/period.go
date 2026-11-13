package schedule

import (
	"fmt"
	"sort"
	"time"
)

const dateLayout = "2006-01-02"

type Unit string

const (
	UnitWeek    Unit = "week"
	UnitService Unit = "service"
)

type Service struct {
	ID        int64
	Weekday   time.Weekday
	StartTime string
}

type Occurrence struct {
	ServiceID int64
	Date      string
	StartTime string
}

/**
 * NOTE: A Group is the set of services that share one team: a whole Monday-to-Sunday week when the ministry
 * rotates weekly, or a single service when it rotates per service.
 */
type Group struct {
	Key         string
	Occurrences []Occurrence
}

func ParseMonth(month string) (time.Time, error) {
	start, err := time.Parse("2006-01", month)
	if err != nil {
		return time.Time{}, fmt.Errorf("mês inválido %q: use AAAA-MM", month)
	}
	return start, nil
}

/**
 * NOTE: A week belongs to the month of its first service, so Thursday the 31st and the Sunday after it stay in the
 * same month's schedule instead of being split across two.
 */
func Period(month string, services []Service, unit Unit) ([]Group, error) {
	start, err := ParseMonth(month)
	if err != nil {
		return nil, err
	}
	end := start.AddDate(0, 1, 0)
	monday := start.AddDate(0, 0, -daysSinceMonday(start.Weekday()))

	var groups []Group
	for ; monday.Before(end); monday = monday.AddDate(0, 0, 7) {
		week := occurrencesInWeek(monday, services)
		if len(week) == 0 || !sameMonth(week[0].Date, start) {
			continue
		}
		if unit == UnitService {
			for _, occurrence := range week {
				groups = append(groups, Group{Key: occurrence.Date + "#" + fmt.Sprint(occurrence.ServiceID), Occurrences: []Occurrence{occurrence}})
			}
			continue
		}
		groups = append(groups, Group{Key: monday.Format(dateLayout), Occurrences: week})
	}
	return groups, nil
}

func occurrencesInWeek(monday time.Time, services []Service) []Occurrence {
	var week []Occurrence
	for _, service := range services {
		date := monday.AddDate(0, 0, daysSinceMonday(service.Weekday))
		week = append(week, Occurrence{ServiceID: service.ID, Date: date.Format(dateLayout), StartTime: service.StartTime})
	}
	sort.Slice(week, func(i, j int) bool {
		if week[i].Date != week[j].Date {
			return week[i].Date < week[j].Date
		}
		return week[i].StartTime < week[j].StartTime
	})
	return week
}

func daysSinceMonday(day time.Weekday) int {
	return (int(day) + 6) % 7
}

func sameMonth(date string, month time.Time) bool {
	return len(date) >= 7 && date[:7] == month.Format("2006-01")
}

func WeekStart(date string) string {
	t, err := time.Parse(dateLayout, date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, -daysSinceMonday(t.Weekday())).Format(dateLayout)
}
