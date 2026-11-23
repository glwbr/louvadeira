package notify

import (
	"testing"
	"time"
)

func TestParseCommand(t *testing.T) {
	friday := time.Date(2026, 11, 13, 21, 0, 0, 0, time.UTC)
	cases := []struct {
		text   string
		kind   commandKind
		ranges string
	}{
		{"Escala", commandSchedule, ""},
		{"minha escala!", commandSchedule, ""},
		{"Ajuda", commandHelp, ""},
		{"não posso", commandCannot, ""},
		{"Não posso ir", commandCannot, ""},
		{"Não posso dia 19", commandCannot, "2026-11-19"},
		{"nao posso dia 5", commandCannot, "2026-12-05"},
		{"não posso 19/11", commandCannot, "2026-11-19"},
		{"não posso 05/01", commandCannot, "2027-01-05"},
		{"Não posso domingo", commandCannot, "2026-11-15"},
		{"não posso na quinta-feira", commandCannot, "2026-11-19"},
		{"não vou conseguir hoje", commandCannot, "2026-11-13"},
		{"Não consigo amanhã", commandCannot, "2026-11-14"},
		{"Não posso de 10/12 a 20/12", commandCannot, "2026-12-10..2026-12-20"},
		{"não vou poder do dia 10/12 até o dia 20/12", commandCannot, "2026-12-10..2026-12-20"},
		{"não posso até 20/12", commandCannot, "2026-11-13..2026-12-20"},
		{"não posso dia 19 e 22/11", commandCannot, "2026-11-19 2026-11-22"},
		{"não posso dia 19, 15 e 22", commandCannot, "2026-11-19 2026-11-15 2026-11-22"},
		{"não posso dia 31/11", commandCannot, ""},
	}
	for _, c := range cases {
		cmd, ok := parseCommand(c.text, friday)
		if !ok || cmd.kind != c.kind || rangesText(cmd) != c.ranges {
			t.Errorf("%q = %v %q (ok %v), want %v %q", c.text, cmd.kind, rangesText(cmd), ok, c.kind, c.ranges)
		}
	}

	for _, text := range []string{"Oi, tudo bem?", "escala de dezembro saiu?", "não posso ir na reunião de jovens", "posso sim"} {
		if cmd, ok := parseCommand(text, friday); ok {
			t.Errorf("%q should be left alone, got %v", text, cmd.kind)
		}
	}
}

func rangesText(cmd command) string {
	var out string
	for i, r := range cmd.ranges {
		if i > 0 {
			out += " "
		}
		if r.From == r.To {
			out += r.From
		} else {
			out += r.From + ".." + r.To
		}
	}
	return out
}

func TestParseCommandUntilWithDayWord(t *testing.T) {
	friday := time.Date(2026, 11, 13, 21, 0, 0, 0, time.UTC)
	cmd, _ := parseCommand("não posso até o dia 20/12", friday)
	if got := rangesText(cmd); got != "2026-11-13..2026-12-20" {
		t.Fatalf("até o dia 20/12 = %q", got)
	}
}

func TestDaysWithoutAMonthAndListsSharingOne(t *testing.T) {
	cases := []struct {
		text   string
		today  time.Time
		ranges string
	}{
		{"não posso dia 15", time.Date(2027, 1, 31, 10, 0, 0, 0, time.UTC), "2027-02-15"},
		{"não posso dia 10", time.Date(2027, 3, 31, 10, 0, 0, 0, time.UTC), "2027-04-10"},
		{"não posso dia 31", time.Date(2027, 4, 10, 10, 0, 0, 0, time.UTC), "2027-05-31"},
		{"não posso dia 30", time.Date(2027, 2, 10, 10, 0, 0, 0, time.UTC), "2027-03-30"},
		{"não posso dia 19 e 22/11", time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC), "2026-11-19 2026-11-22"},
		{"não posso 19 e 22/11", time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC), "2026-11-19 2026-11-22"},
		{"não posso 19, 20 e 22/11", time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC), "2026-11-19 2026-11-20 2026-11-22"},
		{"não posso dia 3 e 6/1/27", time.Date(2026, 12, 20, 10, 0, 0, 0, time.UTC), "2027-01-03 2027-01-06"},
		{"não posso de 10/12 a 20/12", time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC), "2026-12-10..2026-12-20"},
	}
	for _, c := range cases {
		cmd, ok := parseCommand(c.text, c.today)
		if !ok || rangesText(cmd) != c.ranges {
			t.Errorf("%q on %s = %q (ok %v), want %q", c.text, c.today.Format("02/01"), rangesText(cmd), ok, c.ranges)
		}
	}
}
