package optimizer

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"sync"

	"hack-395e4fb2-ai4edu/internal/simulation"
)

// Goal separates the ranking objective from hard constraints. ProtectDistricts
// forbids a district score below its initial score, before any decisions.
// BudgetLimit is a cap on spending, not a replacement for the official budget.
type Goal struct {
	Objective        string   `json:"objective"`
	FocusDistrict    string   `json:"focus_district"`
	BudgetLimit      int      `json:"budget_limit"`
	MaxCritical      *int     `json:"max_critical"`
	ProtectDistricts []string `json:"protect_districts"`
}

// SearchResult counts evaluated scenarios after catalog/budget pruning, and
// feasible scenarios after all hard constraints. Exhaustive means the entire
// feasible catalog was searched, including when no feasible solution exists.
type SearchResult struct {
	Candidates []simulation.Result `json:"candidates"`
	Evaluated  int                 `json:"evaluated"`
	Feasible   int                 `json:"feasible"`
	Exhaustive bool                `json:"exhaustive"`
}

const maxGoalCandidates = 5
const maxCachedGoals = 8

// Only completed searches are cached, with at most five results per goal.
// Cached results never escape: every response is reconstructed by the engine.
var goalCache = struct {
	sync.Mutex
	entries map[string]SearchResult
	order   []string
}{entries: make(map[string]SearchResult)}

// A bounded queue prevents concurrent requests from starting competing full
// CPU searches. Waiting and traversal both honor the request context.
var goalSearchSlot = make(chan struct{}, 1)

func ValidateGoal(goal Goal) error {
	s := simulation.DefaultScenario()
	switch goal.Objective {
	case "score", "weakest_district", "focus_district", "critical_first":
	default:
		return fmt.Errorf("objective must be score, weakest_district, focus_district, or critical_first")
	}
	if goal.BudgetLimit < 1 || goal.BudgetLimit > s.Budget {
		return fmt.Errorf("budget_limit must be between 1 and %d", s.Budget)
	}
	known := make(map[string]bool, len(s.Districts))
	for _, district := range s.Districts {
		known[district.ID] = true
	}
	if goal.Objective == "focus_district" {
		if !known[goal.FocusDistrict] {
			return fmt.Errorf("focus_district must name a known district for this objective")
		}
	} else if goal.FocusDistrict != "" {
		return fmt.Errorf("focus_district must be empty unless objective is focus_district")
	}
	if goal.MaxCritical != nil && (*goal.MaxCritical < 0 || *goal.MaxCritical > len(s.Districts)*len(s.IndicatorOrder)) {
		return fmt.Errorf("max_critical must be between 0 and %d", len(s.Districts)*len(s.IndicatorOrder))
	}
	seen := make(map[string]bool, len(goal.ProtectDistricts))
	for _, id := range goal.ProtectDistricts {
		if !known[id] {
			return fmt.Errorf("protect_districts contains unknown district %q", id)
		}
		if seen[id] {
			return fmt.Errorf("protect_districts contains duplicate district %q", id)
		}
		seen[id] = true
	}
	return nil
}

// SatisfiesGoal checks a result produced by simulation.Simulate. It does not
// recalculate, modify the score, or assume that user-supplied numbers are trusted.
func SatisfiesGoal(goal Goal, result simulation.Result) bool {
	return ValidateGoal(goal) == nil && satisfiesGoal(goal, result)
}

// EquallyRanked compares engine-produced results on the requested objective and
// the official Score tie-break, while allowing different decision IDs. Both
// results must satisfy the hard constraints. It does not authenticate results
// from callers: recalculate untrusted decisions with simulation.Simulate first.
func EquallyRanked(goal Goal, a, b simulation.Result) bool {
	if ValidateGoal(goal) != nil || !satisfiesGoal(goal, a) || !satisfiesGoal(goal, b) {
		return false
	}
	return goalObjectiveValue(a, goal) == goalObjectiveValue(b, goal) && *a.FinalScore == *b.FinalScore
}

func satisfiesGoal(goal Goal, result simulation.Result) bool {
	if !result.Valid || result.FinalScore == nil || result.CriticalAfter == nil || result.FinalBreakdown == nil || result.TotalCost > goal.BudgetLimit {
		return false
	}
	if goal.MaxCritical != nil && *result.CriticalAfter > *goal.MaxCritical {
		return false
	}
	if goal.Objective == "focus_district" {
		if _, ok := goalDistrictScore(result, goal.FocusDistrict); !ok {
			return false
		}
	}
	for _, id := range goal.ProtectDistricts {
		found := false
		for _, district := range result.DistrictBeforeAfter {
			if district.DistrictID == id {
				if district.ScoreAfter < district.ScoreBefore {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Search returns the best 1..5 scenarios for the goal, using the unchanged
// simulation engine. A canceled search returns partial results with an error and
// Exhaustive=false; these must never be presented as proven optimal/infeasible.
func Search(ctx context.Context, goal Goal, limit int) (SearchResult, error) {
	empty := SearchResult{Candidates: []simulation.Result{}}
	if err := ValidateGoal(goal); err != nil {
		return empty, err
	}
	if limit < 1 || limit > maxGoalCandidates {
		return empty, fmt.Errorf("limit must be between 1 and %d", maxGoalCandidates)
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	// Snapshot pointer/slice fields, and normalize set order for cache identity.
	goal.ProtectDistricts = append([]string(nil), goal.ProtectDistricts...)
	sort.Strings(goal.ProtectDistricts)
	if goal.MaxCritical != nil {
		value := *goal.MaxCritical
		goal.MaxCritical = &value
	}
	encoded, _ := json.Marshal(goal)
	key := string(encoded)
	if cached, ok := cachedGoal(key, limit); ok {
		return cached, nil
	}
	select {
	case goalSearchSlot <- struct{}{}:
		defer func() { <-goalSearchSlot }()
	case <-ctx.Done():
		return empty, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	// Another request may have completed the identical goal while this waited.
	if cached, ok := cachedGoal(key, limit); ok {
		return cached, nil
	}
	result, err := searchGoal(ctx, simulation.DefaultScenario(), goal, runtime.GOMAXPROCS(0))
	if err != nil {
		return copyGoalResult(result, limit), err
	}
	goalCache.Lock()
	if len(goalCache.order) == maxCachedGoals {
		delete(goalCache.entries, goalCache.order[0])
		goalCache.order = goalCache.order[1:]
	}
	goalCache.entries[key] = result
	goalCache.order = append(goalCache.order, key)
	goalCache.Unlock()
	return copyGoalResult(result, limit), nil
}

func cachedGoal(key string, limit int) (SearchResult, bool) {
	goalCache.Lock()
	result, ok := goalCache.entries[key]
	goalCache.Unlock()
	if !ok {
		return SearchResult{}, false
	}
	return copyGoalResult(result, limit), true
}

func copyGoalResult(result SearchResult, limit int) SearchResult {
	out := SearchResult{Candidates: []simulation.Result{}, Evaluated: result.Evaluated, Feasible: result.Feasible, Exhaustive: result.Exhaustive}
	for i, candidate := range result.Candidates {
		if i == limit {
			break
		}
		out.Candidates = append(out.Candidates, simulation.Simulate(candidate.Decisions))
	}
	return out
}

// searchGoal takes a catalog subset for independent small-oracle tests. Every
// score and final validation still comes from the official simulation engine.
func searchGoal(ctx context.Context, s simulation.Scenario, goal Goal, workers int) (SearchResult, error) {
	result := SearchResult{Candidates: []simulation.Result{}}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	jobCount := len(s.Measures) - s.RequiredDecisions + 1
	if jobCount <= 0 {
		result.Exhaustive = true
		return result, nil
	}
	if workers > 8 {
		workers = 8
	}
	if workers > jobCount {
		workers = jobCount
	}
	if workers < 1 {
		workers = 1
	}
	jobs := make(chan int, jobCount)
	for first := 0; first < jobCount; first++ {
		jobs <- first
	}
	close(jobs)
	results := make(chan SearchResult, workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			local := SearchResult{Candidates: []simulation.Result{}}
			for first := range jobs {
				if ctx.Err() != nil {
					break
				}
				if err := visitGoalScenarios(ctx, s, first, goal.BudgetLimit, func(decisions []simulation.Decision) {
					local.Evaluated++
					next := simulation.Simulate(decisions)
					if satisfiesGoal(goal, next) {
						local.Feasible++
						local.Candidates = insertGoalCandidate(local.Candidates, next, goal)
					}
				}); err != nil {
					break
				}
			}
			results <- local
		}()
	}
	for worker := 0; worker < workers; worker++ {
		local := <-results
		result.Evaluated += local.Evaluated
		result.Feasible += local.Feasible
		for _, next := range local.Candidates {
			result.Candidates = insertGoalCandidate(result.Candidates, next, goal)
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	result.Exhaustive = true
	return result, nil
}

func insertGoalCandidate(best []simulation.Result, next simulation.Result, goal Goal) []simulation.Result {
	index := sort.Search(len(best), func(i int) bool { return betterForGoal(next, best[i], goal) })
	if index >= maxGoalCandidates {
		return best
	}
	best = append(best, simulation.Result{})
	copy(best[index+1:], best[index:])
	best[index] = next
	if len(best) > maxGoalCandidates {
		best = best[:maxGoalCandidates]
	}
	return best
}

func betterForGoal(a, b simulation.Result, goal Goal) bool {
	aPrimary, bPrimary := goalObjectiveValue(a, goal), goalObjectiveValue(b, goal)
	if aPrimary != bPrimary {
		return aPrimary > bPrimary
	}
	return better(candidate(a), candidate(b))
}

func goalObjectiveValue(result simulation.Result, goal Goal) float64 {
	switch goal.Objective {
	case "weakest_district":
		return result.FinalBreakdown.MinimumDistrict
	case "focus_district":
		value, _ := goalDistrictScore(result, goal.FocusDistrict)
		return value
	case "critical_first":
		return -float64(*result.CriticalAfter)
	default:
		return *result.FinalScore
	}
}

func goalDistrictScore(result simulation.Result, id string) (float64, bool) {
	for _, district := range result.DistrictBeforeAfter {
		if district.DistrictID == id {
			return district.ScoreAfter, true
		}
	}
	return 0, false
}

func visitGoalScenarios(ctx context.Context, s simulation.Scenario, first, budget int, visit func([]simulation.Decision)) error {
	decisions := make([]simulation.Decision, 0, s.RequiredDecisions)
	categories := make(map[simulation.Category]int)
	var walk func(int, int) error
	walk = func(start, cost int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(decisions) == s.RequiredDecisions {
			visit(decisions)
			return ctx.Err()
		}
		last := len(s.Measures) - (s.RequiredDecisions - len(decisions))
		if len(decisions) == 0 {
			last = first
		}
		for i := start; i <= last; i++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			measure := s.Measures[i]
			if cost+measure.Cost > budget || categories[measure.Category] >= s.MaxMeasuresPerCategory {
				continue
			}
			for _, option := range measureOptions(s, measure) {
				if incompatible(s, decisions, option) {
					continue
				}
				decisions = append(decisions, option)
				categories[measure.Category]++
				err := walk(i+1, cost+measure.Cost)
				categories[measure.Category]--
				decisions = decisions[:len(decisions)-1]
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(first, 0)
}
