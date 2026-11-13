package schedule_test

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"louvadeira/internal/schedule"
)

const (
	voice  = 1
	guitar = 2
	bass   = 3
	drums  = 4

	thursday = 10
	sunday   = 20

	leader   = 100
	drummer  = 101
	singerA  = 200
	singerB  = 201
	singerC  = 202
	singerD  = 203
	bassistA = 300
	bassistB = 301
)

var services = []schedule.Service{
	{ID: thursday, Weekday: time.Thursday, StartTime: "19:30"},
	{ID: sunday, Weekday: time.Sunday, StartTime: "19:00"},
}

var roles = []schedule.Role{{ID: voice, Slots: 3}, {ID: guitar, Slots: 1}, {ID: bass, Slots: 1}, {ID: drums, Slots: 1}}

func worshipTeam() []schedule.Member {
	return []schedule.Member{
		{PersonID: leader, Roles: map[int64]bool{guitar: true, voice: true}},
		{PersonID: drummer, Roles: map[int64]bool{drums: true, guitar: false}},
		{PersonID: singerA, Roles: map[int64]bool{voice: false}},
		{PersonID: singerB, Roles: map[int64]bool{voice: false}},
		{PersonID: singerC, Roles: map[int64]bool{voice: false}},
		{PersonID: singerD, Roles: map[int64]bool{voice: false}},
		{PersonID: bassistA, Roles: map[int64]bool{bass: false}},
		{PersonID: bassistB, Roles: map[int64]bool{bass: false}},
	}
}

func december(t *testing.T) []schedule.Group {
	t.Helper()
	groups, err := schedule.Period("2026-12", services, schedule.UnitWeek)
	if err != nil {
		t.Fatal(err)
	}
	return groups
}

type key struct {
	date string
	role int64
}

func index(assignments []schedule.Assignment) map[key][]int64 {
	out := map[key][]int64{}
	for _, a := range assignments {
		k := key{a.Date, a.RoleID}
		out[k] = append(out[k], a.PersonID)
	}
	for k := range out {
		sort.Slice(out[k], func(i, j int) bool { return out[k][i] < out[k][j] })
	}
	return out
}

func rotatingSingers(people []int64) []int64 {
	var out []int64
	for _, p := range people {
		if p != leader {
			out = append(out, p)
		}
	}
	return out
}

func TestPeriodGroupsWeeksByTheMonthOfTheirFirstService(t *testing.T) {
	groups := december(t)
	var got []string
	for _, g := range groups {
		got = append(got, fmt.Sprintf("%s:%s+%s", g.Key, g.Occurrences[0].Date, g.Occurrences[1].Date))
	}
	want := []string{
		"2026-11-30:2026-12-03+2026-12-06",
		"2026-12-07:2026-12-10+2026-12-13",
		"2026-12-14:2026-12-17+2026-12-20",
		"2026-12-21:2026-12-24+2026-12-27",
		"2026-12-28:2026-12-31+2027-01-03",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groups =\n%v\nwant\n%v", got, want)
	}

	perService, _ := schedule.Period("2026-12", services, schedule.UnitService)
	if len(perService) != 10 || len(perService[0].Occurrences) != 1 {
		t.Fatalf("per-service groups = %d", len(perService))
	}
	if _, err := schedule.Period("dezembro", services, schedule.UnitWeek); err == nil {
		t.Fatal("expected an error for a malformed month")
	}
}

func TestFixedMembersServeEveryWeekAndTheWeekKeepsOneTeam(t *testing.T) {
	groups := december(t)
	got := index(schedule.Generate(schedule.Input{Unit: schedule.UnitWeek, Groups: groups, Roles: roles, Members: worshipTeam()}))

	for _, g := range groups {
		thu, sun := g.Occurrences[0].Date, g.Occurrences[1].Date
		for _, role := range []int64{voice, guitar, bass, drums} {
			if !reflect.DeepEqual(got[key{thu, role}], got[key{sun, role}]) {
				t.Errorf("week %s role %d: Thursday %v != Sunday %v", g.Key, role, got[key{thu, role}], got[key{sun, role}])
			}
		}
		if voices := got[key{thu, voice}]; len(voices) != 3 || !contains(voices, leader) {
			t.Errorf("week %s voices = %v, want leader + 2", g.Key, voices)
		}
		if g := got[key{thu, guitar}]; !reflect.DeepEqual(g, []int64{leader}) {
			t.Errorf("guitar = %v, want leader", g)
		}
		if d := got[key{thu, drums}]; !reflect.DeepEqual(d, []int64{drummer}) {
			t.Errorf("drums = %v, want drummer", d)
		}
	}
}

func TestPairsNeverRepeatOnConsecutiveWeeksAndLoadStaysBalanced(t *testing.T) {
	groups := december(t)
	got := index(schedule.Generate(schedule.Input{Unit: schedule.UnitWeek, Groups: groups, Roles: roles, Members: worshipTeam()}))

	count := map[int64]int{}
	var previous []int64
	for _, g := range groups {
		pair := rotatingSingers(got[key{g.Occurrences[0].Date, voice}])
		if reflect.DeepEqual(pair, previous) {
			t.Errorf("week %s repeats last week's pair %v", g.Key, pair)
		}
		previous = pair
		for _, p := range pair {
			count[p]++
		}
		basses := got[key{g.Occurrences[0].Date, bass}]
		count[basses[0]]++
	}
	for _, singer := range []int64{singerA, singerB, singerC, singerD} {
		if count[singer] < 2 || count[singer] > 3 {
			t.Errorf("singer %d served %d of 5 weeks, want 2 or 3", singer, count[singer])
		}
	}
	if diff := count[bassistA] - count[bassistB]; diff < -1 || diff > 1 {
		t.Errorf("bassists served %d and %d weeks", count[bassistA], count[bassistB])
	}
}

func TestUnavailablePeopleAreSkippedAndTheWeekPrefersWhoCanDoBothServices(t *testing.T) {
	groups := december(t)
	second := groups[1]
	in := schedule.Input{
		Unit:    schedule.UnitWeek,
		Groups:  groups[1:2],
		Roles:   roles,
		Members: worshipTeam(),
		Unavailable: []schedule.Unavailability{
			{PersonID: singerA, Start: second.Occurrences[1].Date, End: second.Occurrences[1].Date},
			{PersonID: singerB, Start: "2026-12-01", End: "2026-12-31"},
			{PersonID: bassistA, Start: second.Occurrences[0].Date, End: second.Occurrences[0].Date},
		},
	}
	got := index(schedule.Generate(in))
	thu, sun := second.Occurrences[0].Date, second.Occurrences[1].Date

	if voices := rotatingSingers(got[key{thu, voice}]); !reflect.DeepEqual(voices, []int64{singerC, singerD}) {
		t.Errorf("Thursday voices = %v, want the two who can do the whole week", voices)
	}
	if !reflect.DeepEqual(got[key{thu, voice}], got[key{sun, voice}]) {
		t.Errorf("voices differ between Thursday and Sunday: %v vs %v", got[key{thu, voice}], got[key{sun, voice}])
	}
	if b := got[key{thu, bass}]; !reflect.DeepEqual(b, []int64{bassistB}) {
		t.Errorf("bass = %v, want the bassist free on Thursday", b)
	}
}

func TestWithoutTheLeaderTheDrummerMovesToGuitarAndDrumsStayEmpty(t *testing.T) {
	groups := december(t)[:1]
	week := groups[0]
	in := schedule.Input{
		Unit:        schedule.UnitWeek,
		Groups:      groups,
		Roles:       []schedule.Role{{ID: voice, Slots: 3}, {ID: guitar, Slots: 1}, {ID: bass, Slots: 1}, {ID: drums, Slots: 1}},
		Members:     worshipTeam(),
		Unavailable: []schedule.Unavailability{{PersonID: leader, Start: week.Occurrences[0].Date, End: week.Occurrences[1].Date}},
	}
	got := index(schedule.Generate(in))
	thu := week.Occurrences[0].Date

	if g := got[key{thu, guitar}]; !reflect.DeepEqual(g, []int64{drummer}) {
		t.Errorf("guitar = %v, want drummer", g)
	}
	if d := got[key{thu, drums}]; !reflect.DeepEqual(d, []int64{0}) {
		t.Errorf("drums = %v, want an empty slot", d)
	}
	if voices := got[key{thu, voice}]; contains(voices, leader) || contains(voices, 0) {
		t.Errorf("voices = %v, want three singers without the leader", voices)
	}
}

func TestNobodyAvailableLeavesTheSlotEmpty(t *testing.T) {
	groups := december(t)[:1]
	week := groups[0]
	got := index(schedule.Generate(schedule.Input{
		Unit:    schedule.UnitWeek,
		Groups:  groups,
		Roles:   roles,
		Members: worshipTeam(),
		Unavailable: []schedule.Unavailability{
			{PersonID: bassistA, Start: week.Occurrences[0].Date, End: week.Occurrences[1].Date},
			{PersonID: bassistB, Start: week.Occurrences[0].Date, End: week.Occurrences[1].Date},
		},
	}))
	if b := got[key{week.Occurrences[1].Date, bass}]; !reflect.DeepEqual(b, []int64{0}) {
		t.Errorf("bass = %v, want empty", b)
	}
}

func TestHistoryAvoidsLastWeeksPairAndWhoServedMore(t *testing.T) {
	groups := december(t)[:1]
	history := []schedule.PastAssignment{
		{PersonID: singerA, RoleID: voice, Date: "2026-11-26"},
		{PersonID: singerB, RoleID: voice, Date: "2026-11-26"},
		{PersonID: singerA, RoleID: voice, Date: "2026-11-29"},
		{PersonID: singerB, RoleID: voice, Date: "2026-11-29"},
		{PersonID: bassistA, RoleID: bass, Date: "2026-11-26"},
		{PersonID: bassistA, RoleID: bass, Date: "2026-11-29"},
	}
	got := index(schedule.Generate(schedule.Input{Unit: schedule.UnitWeek, Groups: groups, Roles: roles, Members: worshipTeam(), History: history}))
	thu := groups[0].Occurrences[0].Date
	if voices := rotatingSingers(got[key{thu, voice}]); !reflect.DeepEqual(voices, []int64{singerC, singerD}) {
		t.Errorf("voices = %v, want the two who did not serve last week", voices)
	}
	if b := got[key{thu, bass}]; !reflect.DeepEqual(b, []int64{bassistB}) {
		t.Errorf("bass = %v, want the bassist who did not serve last week", b)
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	in := schedule.Input{Unit: schedule.UnitWeek, Groups: december(t), Roles: roles, Members: worshipTeam()}
	if !reflect.DeepEqual(schedule.Generate(in), schedule.Generate(in)) {
		t.Fatal("two runs with the same input differ")
	}
}

func contains(list []int64, value int64) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
