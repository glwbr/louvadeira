package schedule

import (
	"sort"
	"time"
)

type Role struct {
	ID    int64
	Slots int
}

/**
 * NOTE: Roles maps each role the member can do to whether they are fixed in it (serve every time).
 */
type Member struct {
	PersonID int64
	Roles    map[int64]bool
}

type Unavailability struct {
	PersonID int64
	Start    string
	End      string
}

type PastAssignment struct {
	PersonID int64
	RoleID   int64
	Date     string
}

/**
 * NOTE: Roles are in priority order: when people are short, earlier roles are filled first. History holds the
 * published assignments of the weeks before the period and seeds the fairness score.
 */
type Input struct {
	Unit        Unit
	Groups      []Group
	Roles       []Role
	Members     []Member
	Unavailable []Unavailability
	History     []PastAssignment
}

/**
 * NOTE: PersonID 0 marks a slot nobody could fill.
 */
type Assignment struct {
	ServiceID int64
	Date      string
	RoleID    int64
	Slot      int
	PersonID  int64
}

/**
 * TRADEOFF: Weights of the cost function. Repeating a pair costs a bit more than serving two weeks in a row, so pairs
 * keep changing over the months at the price of someone occasionally serving consecutive weeks. Repeating last week's
 * pair is close to forbidden.
 */
const (
	weightLoad          = 10
	weightServedLast    = 6
	weightPairTogether  = 8
	weightSamePairAgain = 40
	maxCombinations     = 5000
)

type pair struct{ a, b int64 }

func newPair(a, b int64) pair {
	if a > b {
		a, b = b, a
	}
	return pair{a, b}
}

type state struct {
	load        map[int64]int
	lastServed  map[int64]string
	servedLast  map[int64]bool
	lastPicks   map[int64]map[pair]bool
	together    map[int64]map[pair]int
	unavailable map[int64][]Unavailability
}

func Generate(in Input) []Assignment {
	s := newState(in)
	members := append([]Member(nil), in.Members...)
	sort.Slice(members, func(i, j int) bool { return members[i].PersonID < members[j].PersonID })

	var out []Assignment
	for _, group := range in.Groups {
		out = append(out, s.fillGroup(group, in.Roles, members)...)
	}
	return out
}

func newState(in Input) *state {
	s := &state{
		load:        map[int64]int{},
		lastServed:  map[int64]string{},
		servedLast:  map[int64]bool{},
		lastPicks:   map[int64]map[pair]bool{},
		together:    map[int64]map[pair]int{},
		unavailable: map[int64][]Unavailability{},
	}
	for _, u := range in.Unavailable {
		s.unavailable[u.PersonID] = append(s.unavailable[u.PersonID], u)
	}

	fixed := map[int64]map[int64]bool{}
	for _, m := range in.Members {
		fixed[m.PersonID] = m.Roles
	}
	groupKey := func(date string) string {
		if in.Unit == UnitService {
			return date
		}
		return WeekStart(date)
	}

	byGroup := map[string]map[int64][]int64{}
	var keys []string
	for _, past := range in.History {
		s.load[past.PersonID]++
		if past.Date > s.lastServed[past.PersonID] {
			s.lastServed[past.PersonID] = past.Date
		}
		if fixed[past.PersonID][past.RoleID] {
			continue
		}
		key := groupKey(past.Date)
		if byGroup[key] == nil {
			byGroup[key] = map[int64][]int64{}
			keys = append(keys, key)
		}
		byGroup[key][past.RoleID] = appendUnique(byGroup[key][past.RoleID], past.PersonID)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for roleID, people := range byGroup[key] {
			s.recordPairs(roleID, people)
		}
	}

	if len(keys) > 0 && len(in.Groups) > 0 && s.isPreviousGroup(keys[len(keys)-1], in) {
		last := keys[len(keys)-1]
		for _, past := range in.History {
			if groupKey(past.Date) == last {
				s.servedLast[past.PersonID] = true
			}
		}
		for roleID, people := range byGroup[last] {
			s.lastPicks[roleID] = pairsOf(people)
		}
	}
	return s
}

func (s *state) isPreviousGroup(key string, in Input) bool {
	if in.Unit == UnitService {
		return true
	}
	first, err := time.Parse(dateLayout, in.Groups[0].Key)
	if err != nil {
		return false
	}
	return first.AddDate(0, 0, -7).Format(dateLayout) == key
}

type taken map[int64][]bool

func (s *state) fillGroup(group Group, roles []Role, members []Member) []Assignment {
	occurrences := group.Occurrences
	takenBy := make([]taken, len(occurrences))
	for i := range takenBy {
		takenBy[i] = taken{}
	}
	served := map[int64]bool{}
	picks := map[int64][]int64{}
	var out []Assignment

	for _, role := range roles {
		assigned := make([][]int64, len(occurrences))
		assign := func(i int, person int64, isFixed bool) {
			assigned[i] = append(assigned[i], person)
			takenBy[i][person] = append(takenBy[i][person], isFixed)
		}

		for _, m := range members {
			if isFixed, has := m.Roles[role.ID]; !has || !isFixed {
				continue
			}
			for i, occurrence := range occurrences {
				if len(assigned[i]) < role.Slots && s.available(m.PersonID, occurrence.Date) && canTake(takenBy[i][m.PersonID], true) {
					assign(i, m.PersonID, true)
				}
			}
		}

		var rotating []int64
		for _, m := range members {
			if isFixed, has := m.Roles[role.ID]; has && !isFixed {
				rotating = append(rotating, m.PersonID)
			}
		}
		eligible := func(person int64, i int) bool {
			return s.available(person, occurrences[i].Date) &&
				canTake(takenBy[i][person], false) &&
				!contains(assigned[i], person)
		}

		need := role.Slots
		for i := range occurrences {
			need = min(need, role.Slots-len(assigned[i]))
		}
		var whole []int64
		for _, person := range rotating {
			ok := true
			for i := range occurrences {
				ok = ok && eligible(person, i)
			}
			if ok {
				whole = append(whole, person)
			}
		}
		for _, person := range s.best(whole, need, role.ID, nil) {
			for i := range occurrences {
				assign(i, person, false)
			}
			picks[role.ID] = appendUnique(picks[role.ID], person)
		}

		for i, occurrence := range occurrences {
			var candidates []int64
			for _, person := range rotating {
				if eligible(person, i) {
					candidates = append(candidates, person)
				}
			}
			for _, person := range s.best(candidates, role.Slots-len(assigned[i]), role.ID, picks[role.ID]) {
				assign(i, person, false)
				picks[role.ID] = appendUnique(picks[role.ID], person)
			}
			for slot := 0; slot < role.Slots; slot++ {
				var person int64
				if slot < len(assigned[i]) {
					person = assigned[i][slot]
					served[person] = true
					s.load[person]++
					if occurrence.Date > s.lastServed[person] {
						s.lastServed[person] = occurrence.Date
					}
				}
				out = append(out, Assignment{ServiceID: occurrence.ServiceID, Date: occurrence.Date, RoleID: role.ID, Slot: slot, PersonID: person})
			}
		}
	}

	s.servedLast = served
	s.lastPicks = map[int64]map[pair]bool{}
	for roleID, people := range picks {
		s.recordPairs(roleID, people)
		s.lastPicks[roleID] = pairsOf(people)
	}
	return out
}

/**
 * INVARIANT: One person, one role per service, except someone fixed in several roles (the leader on guitar and voice)
 * who does all of their fixed roles together.
 */
func canTake(rolesTaken []bool, fixedInThisRole bool) bool {
	if len(rolesTaken) == 0 {
		return true
	}
	if !fixedInThisRole {
		return false
	}
	for _, wasFixed := range rolesTaken {
		if !wasFixed {
			return false
		}
	}
	return true
}

func (s *state) available(person int64, date string) bool {
	for _, u := range s.unavailable[person] {
		if u.Start <= date && date <= u.End {
			return false
		}
	}
	return true
}

func (s *state) best(candidates []int64, k int, roleID int64, alreadyPicked []int64) []int64 {
	if k <= 0 || len(candidates) == 0 {
		return nil
	}
	k = min(k, len(candidates))
	ordered := append([]int64(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if s.load[a] != s.load[b] {
			return s.load[a] < s.load[b]
		}
		if s.lastServed[a] != s.lastServed[b] {
			return s.lastServed[a] < s.lastServed[b]
		}
		return a < b
	})

	if binomial(len(ordered), k) > maxCombinations {
		return s.greedy(ordered, k, roleID, alreadyPicked)
	}
	var bestSet []int64
	bestCost := -1
	combination := make([]int64, 0, k)
	var walk func(start int)
	walk = func(start int) {
		if len(combination) == k {
			if cost := s.cost(combination, roleID, alreadyPicked); bestCost < 0 || cost < bestCost {
				bestCost = cost
				bestSet = append([]int64(nil), combination...)
			}
			return
		}
		for i := start; i <= len(ordered)-(k-len(combination)); i++ {
			combination = append(combination, ordered[i])
			walk(i + 1)
			combination = combination[:len(combination)-1]
		}
	}
	walk(0)
	return bestSet
}

func (s *state) greedy(ordered []int64, k int, roleID int64, alreadyPicked []int64) []int64 {
	chosen := []int64{}
	for len(chosen) < k {
		var next int64
		nextCost := -1
		for _, person := range ordered {
			if contains(chosen, person) {
				continue
			}
			if cost := s.cost(append(append([]int64(nil), chosen...), person), roleID, alreadyPicked); nextCost < 0 || cost < nextCost {
				next, nextCost = person, cost
			}
		}
		chosen = append(chosen, next)
	}
	return chosen
}

func (s *state) cost(set []int64, roleID int64, alreadyPicked []int64) int {
	total := 0
	for _, person := range set {
		total += weightLoad * s.load[person]
		if s.servedLast[person] {
			total += weightServedLast
		}
	}
	group := append(append([]int64(nil), alreadyPicked...), set...)
	for p := range pairsOf(group) {
		if !contains(set, p.a) && !contains(set, p.b) {
			continue
		}
		total += weightPairTogether * s.together[roleID][p]
		if s.lastPicks[roleID][p] {
			total += weightSamePairAgain
		}
	}
	return total
}

func (s *state) recordPairs(roleID int64, people []int64) {
	if s.together[roleID] == nil {
		s.together[roleID] = map[pair]int{}
	}
	for p := range pairsOf(people) {
		s.together[roleID][p]++
	}
}

func pairsOf(people []int64) map[pair]bool {
	pairs := map[pair]bool{}
	for i := range people {
		for j := i + 1; j < len(people); j++ {
			pairs[newPair(people[i], people[j])] = true
		}
	}
	return pairs
}

func binomial(n, k int) int {
	result := 1
	for i := 1; i <= k; i++ {
		result = result * (n - k + i) / i
		if result > maxCombinations {
			return result
		}
	}
	return result
}

func contains(list []int64, value int64) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func appendUnique(list []int64, value int64) []int64 {
	if contains(list, value) {
		return list
	}
	return append(list, value)
}
