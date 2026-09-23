package optimizer

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"hack-395e4fb2-ai4edu/internal/simulation"
)

func TestValidateGoal(t *testing.T) {
	negative, tooMany, zero := -1, 51, 0
	for _, goal := range []Goal{
		{Objective: "score", BudgetLimit: 100},
		{Objective: "weakest_district", BudgetLimit: 1},
		{Objective: "focus_district", FocusDistrict: "nura", BudgetLimit: 90, ProtectDistricts: []string{"nura", "esil"}},
		{Objective: "critical_first", BudgetLimit: 100, MaxCritical: &zero},
	} {
		if err := ValidateGoal(goal); err != nil {
			t.Fatalf("valid goal %+v: %v", goal, err)
		}
	}
	for _, goal := range []Goal{
		{}, {Objective: "arbitrary", BudgetLimit: 100},
		{Objective: "score", BudgetLimit: 0}, {Objective: "score", BudgetLimit: 101},
		{Objective: "focus_district", BudgetLimit: 100},
		{Objective: "focus_district", FocusDistrict: "unknown", BudgetLimit: 100},
		{Objective: "score", FocusDistrict: "nura", BudgetLimit: 100},
		{Objective: "score", BudgetLimit: 100, ProtectDistricts: []string{"unknown"}},
		{Objective: "score", BudgetLimit: 100, ProtectDistricts: []string{"nura", "nura"}},
		{Objective: "score", BudgetLimit: 100, MaxCritical: &negative},
		{Objective: "score", BudgetLimit: 100, MaxCritical: &tooMany},
	} {
		if err := ValidateGoal(goal); err == nil {
			t.Fatalf("accepted invalid goal %+v", goal)
		}
		if SatisfiesGoal(goal, simulation.Simulate(golden())) {
			t.Fatal("invalid goal must never be satisfied")
		}
	}
}

func TestSatisfiesGoalHardConstraints(t *testing.T) {
	zero := 0
	goal := Goal{Objective: "score", BudgetLimit: 95, MaxCritical: &zero, ProtectDistricts: []string{"nura"}}
	r := simulation.Simulate(golden())
	if !SatisfiesGoal(goal, r) {
		t.Fatal("golden must satisfy budget, zero criticals and protected Nura")
	}
	goal.BudgetLimit = 94
	if SatisfiesGoal(goal, r) {
		t.Fatal("over budget result accepted")
	}
	goal.BudgetLimit = 95
	*r.CriticalAfter = 1
	if SatisfiesGoal(goal, r) {
		t.Fatal("critical cap ignored")
	}
	*r.CriticalAfter = 0
	for i := range r.DistrictBeforeAfter {
		if r.DistrictBeforeAfter[i].DistrictID == "nura" {
			r.DistrictBeforeAfter[i].ScoreAfter = r.DistrictBeforeAfter[i].ScoreBefore - 0.01
		}
	}
	if SatisfiesGoal(goal, r) {
		t.Fatal("protected district regression accepted")
	}
	if SatisfiesGoal(goal, simulation.Simulate(nil)) {
		t.Fatal("invalid simulation accepted")
	}
	// Safe crossings reduce T1 but improve the district's total Score. The
	// protection explicitly applies to that total, not to every indicator.
	decisions := golden()
	for i := range decisions {
		if decisions[i].MeasureID == "M10" {
			decisions[i].MeasureID = "M11"
		}
	}
	crossings := simulation.Simulate(decisions)
	if !SatisfiesGoal(goal, crossings) || crossings.IndicatorDeltas["nura"]["T1"] >= 0 {
		t.Fatal("district Score protection was incorrectly applied to each indicator")
	}
}

func TestEquallyRanked(t *testing.T) {
	for _, objective := range []string{"score", "weakest_district", "focus_district", "critical_first"} {
		t.Run(objective, func(t *testing.T) {
			goal := Goal{Objective: objective, BudgetLimit: 100}
			if objective == "focus_district" {
				goal.FocusDistrict = "nura"
			}
			a, b := simulation.Simulate(golden()), simulation.Simulate(golden())
			if !EquallyRanked(goal, a, b) {
				t.Fatal("identical engine results must tie")
			}
			// Alter only the ID tie-break in this ranking fixture. Numeric equality
			// must not depend on which of two plans sorts first by decision IDs.
			b.Decisions[0].MeasureID = "lexically_different"
			if !EquallyRanked(goal, a, b) {
				t.Fatal("canonical ID tie-break must not reject a numerical tie")
			}
			*b.FinalScore = math.Nextafter(*a.FinalScore, math.Inf(-1))
			if EquallyRanked(goal, a, b) || EquallyRanked(goal, b, a) {
				t.Fatal("official Score difference must break ties without rounding")
			}
			*b.FinalScore = *a.FinalScore
			switch objective {
			case "score":
				*b.FinalScore++
			case "weakest_district":
				b.FinalBreakdown.MinimumDistrict++
			case "critical_first":
				*b.CriticalAfter++
			case "focus_district":
				for i := range b.DistrictBeforeAfter {
					if b.DistrictBeforeAfter[i].DistrictID == goal.FocusDistrict {
						b.DistrictBeforeAfter[i].ScoreAfter++
					}
				}
			}
			if EquallyRanked(goal, a, b) || EquallyRanked(goal, b, a) {
				t.Fatal("different objective values cannot tie")
			}
		})
	}
	r := simulation.Simulate(golden())
	if EquallyRanked(Goal{}, r, r) {
		t.Fatal("invalid goal accepted")
	}
	if EquallyRanked(Goal{Objective: "score", BudgetLimit: 94}, r, r) {
		t.Fatal("over-budget plans cannot count as acceptable ties")
	}
	if EquallyRanked(Goal{Objective: "score", BudgetLimit: 100}, r, simulation.Simulate(nil)) {
		t.Fatal("invalid result accepted")
	}
}

func goalTestCatalog() simulation.Scenario {
	s := simulation.DefaultScenario()
	s.Measures = []simulation.Measure{s.Measures[0], s.Measures[1], s.Measures[3], s.Measures[6], s.Measures[7], s.Measures[8], s.Measures[10], s.Measures[13]}
	s.Districts = []simulation.District{s.Districts[0], s.Districts[4]}
	return s
}

// This oracle makes no pruning assumptions: enumerate all measure subsets and
// districts, then let the engine and separate constraint/ranking code decide.
func goalOracle(s simulation.Scenario, goal Goal) []simulation.Result {
	var found []simulation.Result
	var walk func(int, []simulation.Decision)
	walk = func(index int, selected []simulation.Decision) {
		if len(selected) == s.RequiredDecisions {
			r := simulation.Simulate(selected)
			if !r.Valid || r.TotalCost > goal.BudgetLimit || (goal.MaxCritical != nil && *r.CriticalAfter > *goal.MaxCritical) {
				return
			}
			for _, id := range goal.ProtectDistricts {
				for _, district := range r.DistrictBeforeAfter {
					if district.DistrictID == id && district.ScoreAfter < district.ScoreBefore {
						return
					}
				}
			}
			found = append(found, r)
			return
		}
		if index == len(s.Measures) {
			return
		}
		walk(index+1, selected)
		m := s.Measures[index]
		if m.Scope == simulation.CityScope {
			walk(index+1, append(selected, simulation.Decision{MeasureID: m.ID}))
		} else {
			for _, district := range s.Districts {
				id := district.ID
				walk(index+1, append(selected, simulation.Decision{MeasureID: m.ID, DistrictID: &id}))
			}
		}
	}
	walk(0, nil)
	primary := func(r simulation.Result) float64 {
		switch goal.Objective {
		case "critical_first":
			return -float64(*r.CriticalAfter)
		case "weakest_district":
			minimum := math.Inf(1)
			for _, district := range r.DistrictBeforeAfter {
				minimum = math.Min(minimum, district.ScoreAfter)
			}
			return minimum
		case "focus_district":
			for _, district := range r.DistrictBeforeAfter {
				if district.DistrictID == goal.FocusDistrict {
					return district.ScoreAfter
				}
			}
		}
		return *r.FinalScore
	}
	sort.Slice(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if primary(a) != primary(b) {
			return primary(a) > primary(b)
		}
		if *a.FinalScore != *b.FinalScore {
			return *a.FinalScore > *b.FinalScore
		}
		for k := range a.Decisions {
			ad, bd := a.Decisions[k], b.Decisions[k]
			if ad.MeasureID != bd.MeasureID {
				return ad.MeasureID < bd.MeasureID
			}
			aID, bID := "", ""
			if ad.DistrictID != nil {
				aID = *ad.DistrictID
			}
			if bd.DistrictID != nil {
				bID = *bd.DistrictID
			}
			if aID != bID {
				return aID < bID
			}
		}
		return false
	})
	return found
}

func TestGoalSearchMatchesIndependentOracle(t *testing.T) {
	zero := 0
	goals := []Goal{
		{Objective: "score", BudgetLimit: 100},
		{Objective: "weakest_district", BudgetLimit: 95, MaxCritical: &zero},
		{Objective: "focus_district", FocusDistrict: "esil", BudgetLimit: 85, ProtectDistricts: []string{"nura", "esil"}},
		{Objective: "critical_first", BudgetLimit: 90},
		{Objective: "score", BudgetLimit: 1},
	}
	for _, goal := range goals {
		t.Run(goal.Objective+"/"+strconv.Itoa(goal.BudgetLimit), func(t *testing.T) {
			s := goalTestCatalog()
			want := goalOracle(s, goal)
			for _, workers := range []int{1, 3, 100} {
				got, err := searchGoal(context.Background(), s, goal, workers)
				if err != nil || !got.Exhaustive || got.Feasible != len(want) || got.Evaluated < got.Feasible {
					t.Fatalf("invalid search statistics: %+v, err=%v, oracle feasible=%d", got, err, len(want))
				}
				count := len(want)
				if count > maxGoalCandidates {
					count = maxGoalCandidates
				}
				if len(got.Candidates) != count {
					t.Fatalf("candidate count %d, want %d", len(got.Candidates), count)
				}
				for i := range got.Candidates {
					if !reflect.DeepEqual(got.Candidates[i], want[i]) {
						t.Fatalf("%d workers candidate %d differs from independent oracle", workers, i)
					}
				}
			}
			// Catalog order changes traversal and jobs, never the exact winner/ties.
			for i, j := 0, len(s.Measures)-1; i < j; i, j = i+1, j-1 {
				s.Measures[i], s.Measures[j] = s.Measures[j], s.Measures[i]
			}
			reversed, err := searchGoal(context.Background(), s, goal, 2)
			if err != nil || reversed.Feasible != len(want) {
				t.Fatal("reversed search changed feasibility")
			}
			for i := range reversed.Candidates {
				if !reflect.DeepEqual(reversed.Candidates[i], want[i]) {
					t.Fatal("traversal changed stable ranking")
				}
			}
		})
	}
}

func TestGoalSearchCancellationAndInput(t *testing.T) {
	goal := Goal{Objective: "score", BudgetLimit: 100}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := Search(ctx, goal, 3)
	if !errors.Is(err, context.Canceled) || result.Exhaustive || result.Evaluated != 0 {
		t.Fatalf("pre-canceled search: %+v %v", result, err)
	}
	for _, limit := range []int{0, -1, 6} {
		if _, err := Search(context.Background(), goal, limit); err == nil {
			t.Fatal("invalid limit accepted")
		}
	}
	if _, err := Search(context.Background(), Goal{}, 1); err == nil {
		t.Fatal("invalid goal accepted")
	}
	// Cancel after the first leaf. The error must unwind traversal immediately,
	// rather than merely skipping simulations while enumerating all other leaves.
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	count := 0
	err = visitGoalScenarios(ctx, simulation.DefaultScenario(), 0, 100, func([]simulation.Decision) {
		count++
		cancel()
	})
	if !errors.Is(err, context.Canceled) || count != 1 {
		t.Fatalf("traversal failed to stop: count=%d err=%v", count, err)
	}
	// Exercise the real worker join/partial-result path, not the completed cache.
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	result, err = searchGoal(ctx, simulation.DefaultScenario(), goal, 3)
	if !errors.Is(err, context.DeadlineExceeded) || result.Exhaustive {
		t.Fatalf("canceled workers claimed completion: %+v %v", result, err)
	}
}

func TestGoalSearchDefaultOptimumAndCacheIsolation(t *testing.T) {
	goal := Goal{Objective: "score", BudgetLimit: 100}
	result, err := Search(context.Background(), goal, 5)
	if err != nil || !result.Exhaustive || len(result.Candidates) != 5 || result.Evaluated != 694395 || result.Feasible != result.Evaluated {
		t.Fatalf("default exact search: evaluated=%d feasible=%d exhaustive=%v candidates=%d err=%v", result.Evaluated, result.Feasible, result.Exhaustive, len(result.Candidates), err)
	}
	best := result.Candidates[0]
	if math.Abs(*best.FinalScore-57.236735) > 1e-9 || best.TotalCost != 98 || *best.CriticalAfter != 0 {
		t.Fatalf("default optimum changed: %+v", best)
	}
	want := encoded(t, result)
	// Corrupt every kind of nested caller-owned value. Later calls must be clean.
	result.Candidates[0].Decisions[0].MeasureID = "corrupt"
	for _, decision := range result.Candidates[0].Decisions {
		if decision.DistrictID != nil {
			*decision.DistrictID = "corrupt"
		}
	}
	*result.Candidates[0].FinalScore = -100
	result.Candidates[0].DistrictBeforeAfter[0].After["T1"] = -100
	for i := 0; i < 4; i++ {
		t.Run("cached concurrent isolation", func(t *testing.T) {
			t.Parallel()
			got, err := Search(context.Background(), goal, 5)
			if err != nil || encoded(t, got) != want {
				t.Fatal("cached search was mutated")
			}
			one, err := Search(context.Background(), goal, 1)
			if err != nil || len(one.Candidates) != 1 || !reflect.DeepEqual(one.Candidates[0], got.Candidates[0]) || one.Evaluated != got.Evaluated {
				t.Fatal("cached limit changed winner/statistics")
			}
		})
	}
}
