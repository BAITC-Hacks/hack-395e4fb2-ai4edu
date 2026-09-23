package optimizer

import (
	"encoding/json"
	"math"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"hack-395e4fb2-ai4edu/internal/simulation"
)

func golden() []simulation.Decision {
	nura, saryarka := "nura", "saryarka"
	return []simulation.Decision{
		{MeasureID: "M7", DistrictID: &nura},
		{MeasureID: "M8", DistrictID: &nura},
		{MeasureID: "M10", DistrictID: &nura},
		{MeasureID: "M12"},
		{MeasureID: "M5", DistrictID: &saryarka},
	}
}

func encoded(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestBest(t *testing.T) {
	want := Best()
	actual := simulation.Simulate(want.Decisions)
	if !actual.Valid || !reflect.DeepEqual(want, candidate(actual)) || want.FinalScore < 56.54307 {
		t.Fatalf("invalid or sub-golden optimum: %+v", want)
	}
	nura := "nura"
	expected := []simulation.Decision{
		{MeasureID: "M14"}, {MeasureID: "M2"},
		{MeasureID: "M3", DistrictID: &nura},
		{MeasureID: "M8", DistrictID: &nura},
		{MeasureID: "M9", DistrictID: &nura},
	}
	if !reflect.DeepEqual(want.Decisions, expected) || math.Abs(want.FinalScore-57.236735) > 1e-9 || want.TotalCost != 98 || want.CriticalAfter != 0 {
		t.Fatalf("parallel search changed the expected optimum: %+v", want)
	}
	if _, improvements := Improve(want.Decisions); len(improvements) != 0 {
		t.Fatalf("global optimum has better neighbors: %+v", improvements)
	}
	jsonWant := encoded(t, want)
	// Neither slice entries nor their district pointers may alias cached data.
	want.Decisions[0].MeasureID = "corrupted"
	for _, d := range want.Decisions {
		if d.DistrictID != nil {
			*d.DistrictID = "corrupted"
		}
	}
	for i := 0; i < 8; i++ {
		t.Run("concurrent isolated cache", func(t *testing.T) {
			t.Parallel()
			if got := encoded(t, Best()); got != jsonWant {
				t.Fatalf("Best changed: %s", got)
			}
			ready, ok := ReadyBest()
			if !ok || encoded(t, ready) != jsonWant {
				t.Fatal("published optimum differs from blocking Best")
			}
		})
	}
	t.Logf("best: %s", jsonWant)
}

// Compare pruning with an independent unpruned enumeration on a smaller catalog
// of real measures/districts. It includes all three incompatibility pairs,
// expensive sets and three measures in a category; Simulate alone filters it.
func TestEnumerationMatchesUnprunedEngine(t *testing.T) {
	s := simulation.DefaultScenario()
	s.Measures = []simulation.Measure{s.Measures[0], s.Measures[1], s.Measures[2], s.Measures[3], s.Measures[4], s.Measures[6], s.Measures[7], s.Measures[12]}
	s.Districts = []simulation.District{s.Districts[2], s.Districts[4]}
	want := make(map[string]bool)
	var oracleBest Candidate
	var enumerate func(int, []simulation.Decision)
	enumerate = func(index int, input []simulation.Decision) {
		if len(input) == s.RequiredDecisions {
			r := simulation.Simulate(input)
			if r.Valid {
				key := encoded(t, r.Decisions)
				want[key] = true
				if oracleBest.Decisions == nil || *r.FinalScore > oracleBest.FinalScore ||
					(*r.FinalScore == oracleBest.FinalScore && key < encoded(t, oracleBest.Decisions)) {
					oracleBest = candidate(r)
				}
			}
			return
		}
		if index == len(s.Measures) {
			return
		}
		enumerate(index+1, input)
		m := s.Measures[index]
		if m.Scope == simulation.CityScope {
			enumerate(index+1, append(input, simulation.Decision{MeasureID: m.ID}))
		} else {
			for _, d := range s.Districts {
				id := d.ID
				enumerate(index+1, append(input, simulation.Decision{MeasureID: m.ID, DistrictID: &id}))
			}
		}
	}
	enumerate(0, nil)
	if len(want) == 0 {
		t.Fatal("empty enumeration oracle")
	}
	got := make(map[string]bool)
	visitScenarios(s, func(input []simulation.Decision) {
		r := simulation.Simulate(input)
		key := encoded(t, r.Decisions)
		if !r.Valid || !want[key] || got[key] {
			t.Fatalf("pruned enumeration emitted invalid, unexpected or duplicate scenario: %s", key)
		}
		got[key] = true
	})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("enumeration omitted scenarios: got %d, want %d", len(got), len(want))
	}
	if best := searchBest(s); !reflect.DeepEqual(best, oracleBest) {
		t.Fatalf("best differs from exhaustive oracle: got %+v, want %+v", best, oracleBest)
	}
	// Vary worker counts on this reduced catalog, without repeating the expensive
	// full-dataset search. Both serial and parallel reductions must match the oracle.
	for _, workers := range []int{1, 2, 3, runtime.NumCPU()} {
		if best := searchBestWithWorkers(s, workers); !reflect.DeepEqual(best, oracleBest) {
			t.Fatalf("%d workers changed the winner: %+v", workers, best)
		}
	}
	// Traversal order must not affect score ties or the winning scenario.
	for i, j := 0, len(s.Measures)-1; i < j; i, j = i+1, j-1 {
		s.Measures[i], s.Measures[j] = s.Measures[j], s.Measures[i]
	}
	if best := searchBest(s); !reflect.DeepEqual(best, oracleBest) {
		t.Fatal("reversing enumeration changed the winner")
	}
}

func TestFullCatalogEnumerationCount(t *testing.T) {
	count := 0
	visitScenarios(simulation.DefaultScenario(), func([]simulation.Decision) { count++ })
	if count != 694395 {
		t.Fatalf("enumerated %d scenarios, want 694395 for the fixed dataset", count)
	}
}

func TestImproveTopThreeAndGoldenInvariants(t *testing.T) {
	input := golden()
	before := encoded(t, input)
	current, got := Improve(input)
	if !current.Valid || math.Abs(*current.BaseScore-52.55768) > 1e-9 ||
		math.Abs(*current.FinalScore-56.54307) > 1e-9 || current.TotalCost != 95 || *current.CriticalAfter != 0 {
		t.Fatalf("golden changed: %+v", current)
	}
	if len(got) != 3 {
		t.Fatalf("got %d improvements, want 3", len(got))
	}
	if encoded(t, input) != before {
		t.Fatal("Improve mutated caller's decisions")
	}
	// Independent neighborhood oracle: remove each decision, append every catalog
	// option, let the engine validate, deduplicate and rank by score then JSON IDs.
	oracle := make(map[string]simulation.Result)
	s := simulation.DefaultScenario()
	for removed := range input {
		remaining := append([]simulation.Decision(nil), input[:removed]...)
		remaining = append(remaining, input[removed+1:]...)
		for _, m := range s.Measures {
			targets := []string{""}
			if m.Scope == simulation.DistrictScope {
				targets = nil
				for _, d := range s.Districts {
					targets = append(targets, d.ID)
				}
			}
			for _, id := range targets {
				d := simulation.Decision{MeasureID: m.ID}
				if id != "" {
					id := id
					d.DistrictID = &id
				}
				r := simulation.Simulate(append(remaining, d))
				if r.Valid && *r.FinalScore > *current.FinalScore {
					oracle[encoded(t, r.Decisions)] = r
				}
			}
		}
	}
	var ranked []simulation.Result
	for _, r := range oracle {
		ranked = append(ranked, r)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if *ranked[i].FinalScore != *ranked[j].FinalScore {
			return *ranked[i].FinalScore > *ranked[j].FinalScore
		}
		return encoded(t, ranked[i].Decisions) < encoded(t, ranked[j].Decisions)
	})
	for i, improvement := range got {
		r := simulation.Simulate(improvement.Decisions)
		if !r.Valid || *r.FinalScore <= *current.FinalScore ||
			!reflect.DeepEqual(improvement.Candidate, candidate(ranked[i])) ||
			improvement.ScoreDelta != *r.FinalScore-*current.FinalScore {
			t.Fatalf("improvement %d disagrees with the engine/top-three oracle: %+v", i, improvement)
		}
	}
	wantJSON := encoded(t, got)
	for i, j := 0, len(input)-1; i < j; i, j = i+1, j-1 {
		input[i], input[j] = input[j], input[i]
	}
	for i := 0; i < 8; i++ {
		t.Run("deterministic improvement", func(t *testing.T) {
			t.Parallel()
			_, repeated := Improve(input)
			if encoded(t, repeated) != wantJSON {
				t.Fatal("permuted/concurrent input changed improvements")
			}
		})
	}
}

func TestImproveInvalid(t *testing.T) {
	for _, input := range [][]simulation.Decision{nil, golden()[:4], append(golden()[:4], golden()[0])} {
		current, improvements := Improve(input)
		if current.Valid || len(improvements) != 0 || !reflect.DeepEqual(current, simulation.Simulate(input)) {
			t.Fatal("invalid scenario must return the unmodified engine validation result")
		}
	}
}

func TestTieBreak(t *testing.T) {
	nura, esil := "nura", "esil"
	a := Candidate{FinalScore: 55, Decisions: []simulation.Decision{{MeasureID: "M7", DistrictID: &esil}}}
	b := Candidate{FinalScore: 55, Decisions: []simulation.Decision{{MeasureID: "M7", DistrictID: &nura}}}
	if !better(a, b) || better(b, a) || better(a, a) {
		t.Fatal("equal scores must use lexicographic districts")
	}
	a.Decisions[0].MeasureID = "M10"
	if !better(a, b) {
		t.Fatal("equal scores must use lexicographic measure IDs")
	}
	b.FinalScore = math.Nextafter(a.FinalScore, math.Inf(1))
	if !better(b, a) {
		t.Fatal("strictly greater engine scores must win without rounding")
	}
}
