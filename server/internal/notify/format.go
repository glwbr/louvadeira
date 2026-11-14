package notify

import (
	"fmt"
	"strings"
	"time"

	"louvadeira/internal/store"
)

var weekdays = []string{"domingo", "segunda", "terça", "quarta", "quinta", "sexta", "sábado"}

var months = []string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}

func parseDate(date string) time.Time {
	t, _ := time.Parse("2006-01-02", date)
	return t
}

func weekday(date string) string {
	return weekdays[parseDate(date).Weekday()]
}

func dayMonth(date string) string {
	return parseDate(date).Format("02/01")
}

func fullDate(date string) string {
	return parseDate(date).Format("02/01/2006")
}

func hour(startTime string) string {
	h, m, _ := strings.Cut(startTime, ":")
	h = strings.TrimPrefix(h, "0")
	if m == "00" {
		return h + "h"
	}
	return h + "h" + m
}

func monthName(month string) string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return month
	}
	return months[t.Month()-1]
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func firstName(name string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(name), " ")
	return first
}

func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " e " + names[len(names)-1]
}

func personNames(people []store.Person) []string {
	names := make([]string, len(people))
	for i, p := range people {
		names[i] = p.Name
	}
	return names
}

/**
 * NOTE: "domingo, 22/11, às 19h" — how every message names a service.
 */
func when(d store.AssignmentDetail) string {
	return fmt.Sprintf("%s, %s, às %s", weekday(d.Date), dayMonth(d.Date), hour(d.StartTime))
}

/**
 * NOTE: "no domingo", "na quinta": weekdays from Monday to Friday are feminine in Portuguese.
 */
func onWeekday(date string) string {
	day := parseDate(date).Weekday()
	if day == time.Saturday || day == time.Sunday {
		return "no " + weekdays[day]
	}
	return "na " + weekdays[day]
}

func communionMark(communion bool) string {
	if communion {
		return " 🍷"
	}
	return ""
}

func slotText(d store.AssignmentDetail) string {
	return fmt.Sprintf("%s (%s)", when(d), d.RoleName)
}

var weekdaysPlural = []string{"domingos", "segundas", "terças", "quartas", "quintas", "sextas", "sábados"}

/**
 * NOTE: How an absence reads after "não vai poder servir no Louvor": "de 16/11 a 13/12", "em 19/11", "às quintas
 * até 31/03" or "no 1º domingo do mês".
 */
func absenceText(u store.Unavailability) string {
	var text string
	switch {
	case u.Repeat == store.RepeatWeekly && u.Weekday != nil:
		text = "às " + weekdaysPlural[*u.Weekday]
		if *u.Weekday == 0 || *u.Weekday == 6 {
			text = "aos " + weekdaysPlural[*u.Weekday]
		}
	case u.Repeat == store.RepeatMonthly && u.Weekday != nil && u.WeekOfMonth != nil:
		text = fmt.Sprintf("na %dª %s do mês", *u.WeekOfMonth, weekdays[*u.Weekday])
		if *u.Weekday == 0 || *u.Weekday == 6 {
			text = fmt.Sprintf("no %dº %s do mês", *u.WeekOfMonth, weekdays[*u.Weekday])
		}
	case u.StartDate == u.EndDate:
		return "em " + dayMonth(u.StartDate)
	default:
		return fmt.Sprintf("de %s a %s", dayMonth(u.StartDate), dayMonth(u.EndDate))
	}
	if u.EndDate != "" {
		text += " até " + dayMonth(u.EndDate)
	}
	return text
}

var reasonLabels = map[store.Reason]string{
	store.ReasonTravel:  "viagem",
	store.ReasonWork:    "trabalho",
	store.ReasonHealth:  "saúde",
	store.ReasonFamily:  "família",
	store.ReasonStudies: "estudos",
	store.ReasonOther:   "outro motivo",
}

/**
 * NOTE: "outro motivo" says nothing to the leaders, so it is left out of their messages.
 */
func reasonLabel(r store.Reason) string {
	if r == store.ReasonOther {
		return ""
	}
	return reasonLabels[r]
}
