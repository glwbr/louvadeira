package notify

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"louvadeira/internal/store"
)

type commandKind string

const (
	commandHelp     commandKind = "ajuda"
	commandSchedule commandKind = "escala"
	commandCannot   commandKind = "não posso"
)

/**
 * NOTE: A message a team member sends in private that the bot understands. For "não posso", ranges holds the days
 * mentioned; empty means the person did not say when.
 */
type command struct {
	kind   commandKind
	ranges []store.DateRange
}

var (
	helpWords     = map[string]bool{"ajuda": true, "menu": true, "comandos": true}
	scheduleWords = map[string]bool{"escala": true, "escalas": true, "minha escala": true, "minhas escalas": true, "ver escala": true, "ver minha escala": true}
	cannotPrefix  = []string{"nao posso", "nao vou poder", "nao vou conseguir", "nao consigo"}
	accents       = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "ç", "c")
	punctuation   = regexp.MustCompile(`[^a-z0-9/ ]+`)
	spaces        = regexp.MustCompile(`\s+`)
	dateWord      = regexp.MustCompile(`(?:dia )?(\d{1,2})/(\d{1,2})(?:/(\d{2,4}))?|dia (\d{1,2})|domingo|segunda|terca|quarta|quinta|sexta|sabado|hoje|amanha`)
	dayList       = regexp.MustCompile(`dia \d{1,2}((?: (?:e )?\d{1,2}\b)+)`)
	rangeLink     = regexp.MustCompile(`^\s*(a|ate|ao|e)\s*(o\s+)?(dia\s+)?$`)
	weekdayNames  = map[string]time.Weekday{"domingo": time.Sunday, "segunda": time.Monday, "terca": time.Tuesday, "quarta": time.Wednesday, "quinta": time.Thursday, "sexta": time.Friday, "sabado": time.Saturday}
)

func normalizeText(text string) string {
	text = accents.Replace(strings.ToLower(text))
	text = punctuation.ReplaceAllString(text, " ")
	return strings.TrimSpace(spaces.ReplaceAllString(text, " "))
}

/**
 * CONSTRAINT: The church number also carries ordinary conversations, so a message only counts as a command when it
 * is one of the known words or starts with "não posso"; anything else is left alone.
 */
func parseCommand(text string, today time.Time) (command, bool) {
	t := normalizeText(text)
	switch {
	case helpWords[t]:
		return command{kind: commandHelp}, true
	case scheduleWords[t]:
		return command{kind: commandSchedule}, true
	}
	for _, prefix := range cannotPrefix {
		if t != prefix && !strings.HasPrefix(t, prefix+" ") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(t, prefix))
		ranges := parseRanges(rest, today)
		// NOTE: Without a date, only a short "não posso (ir)" is about the schedule; "não posso ir na reunião" is not.
		if len(ranges) == 0 && len(strings.Fields(rest)) > 2 {
			return command{}, false
		}
		return command{kind: commandCannot, ranges: ranges}, true
	}
	return command{}, false
}

/**
 * NOTE: Reads "dia 19", "19/11", "domingo", "amanhã", "de 10/12 a 20/12", "até 20/12" and lists like "19 e 22/11".
 * Dates without a year, and days without a month, mean the next time that day comes.
 */
func parseRanges(text string, today time.Time) []store.DateRange {
	// NOTE: "dia 22, 19 e 15" lists more days after the first one; spell each out as "dia N".
	text = dayList.ReplaceAllStringFunc(text, func(list string) string {
		var days []string
		for _, field := range strings.Fields(list)[1:] {
			if field != "e" {
				days = append(days, "dia "+field)
			}
		}
		return strings.Join(days, " e ")
	})
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	matches := dateWord.FindAllStringSubmatchIndex(text, -1)
	var dates []time.Time
	var starts, ends []int
	for _, m := range matches {
		d, ok := resolveDate(text, m, day)
		if !ok {
			continue
		}
		dates = append(dates, d)
		starts, ends = append(starts, m[0]), append(ends, m[1])
	}
	if len(dates) == 0 {
		return nil
	}
	format := func(t time.Time) string { return t.Format("2006-01-02") }
	if before := strings.TrimSpace(text[:starts[0]]); len(dates) == 1 && (strings.HasSuffix(before, "ate") || strings.HasSuffix(before, "ate o")) {
		return []store.DateRange{{From: format(day), To: format(dates[0])}}
	}
	var ranges []store.DateRange
	for i := 0; i < len(dates); i++ {
		if i+1 < len(dates) && dates[i+1].After(dates[i]) {
			link := text[ends[i]:starts[i+1]]
			if m := rangeLink.FindStringSubmatch(link); m != nil && m[1] != "e" {
				ranges = append(ranges, store.DateRange{From: format(dates[i]), To: format(dates[i+1])})
				i++
				continue
			}
		}
		ranges = append(ranges, store.DateRange{From: format(dates[i]), To: format(dates[i])})
	}
	return ranges
}

func resolveDate(text string, m []int, today time.Time) (time.Time, bool) {
	group := func(i int) string {
		if m[2*i] < 0 {
			return ""
		}
		return text[m[2*i]:m[2*i+1]]
	}
	word := text[m[0]:m[1]]
	switch {
	case group(1) != "":
		d, _ := strconv.Atoi(group(1))
		mo, _ := strconv.Atoi(group(2))
		year := today.Year()
		if y := group(3); y != "" {
			year, _ = strconv.Atoi(y)
			if year < 100 {
				year += 2000
			}
		}
		date, ok := validDate(year, time.Month(mo), d)
		if ok && group(3) == "" && date.Before(today) {
			date, ok = validDate(year+1, time.Month(mo), d)
		}
		return date, ok
	case group(4) != "":
		d, _ := strconv.Atoi(group(4))
		date, ok := validDate(today.Year(), today.Month(), d)
		if ok && date.Before(today) {
			next := today.AddDate(0, 1, 0)
			date, ok = validDate(next.Year(), next.Month(), d)
		}
		return date, ok
	case word == "hoje":
		return today, true
	case word == "amanha":
		return today.AddDate(0, 0, 1), true
	}
	weekday := weekdayNames[word]
	return today.AddDate(0, 0, (int(weekday)-int(today.Weekday())+7)%7), true
}

func validDate(year int, month time.Month, day int) (time.Time, bool) {
	date := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return date, date.Year() == year && date.Month() == month && date.Day() == day
}
