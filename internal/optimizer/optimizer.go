// Package optimizer searches scenarios using simulation.Simulate as the only
// source of validation and scores. Search pruning uses the published catalog.
package optimizer

import (
	"sort"
	"sync"

	"hack-395e4fb2-ai4edu/internal/simulation"
)

// Candidate contains engine output, without generated explanations.
type Candidate struct {
	Decisions       []simulation.Decision `json:"decisions"`
	FinalScore      float64               `json:"final_score"`
	TotalCost       int                   `json:"total_cost"`
	RemainingBudget int                   `json:"remaining_budget"`
	CriticalAfter   int                   `json:"critical_after"`
}

// Improvement measures ScoreDelta against the submitted scenario, not base Score.
type Improvement struct {
	Candidate
	ScoreDelta float64 `json:"score_delta"`
}

var (
	bestOnce      sync.Once
	bestDecisions []simulation.Decision
)

// Best exhaustively searches once per process. The server calls this before
// listening. Subsequent calls simulate the cached decisions to return fresh data
// that callers can safely mutate without affecting the cache or other requests.
func Best() Candidate {
	bestOnce.Do(func() {
		bestDecisions = searchBest(simulation.DefaultScenario()).Decisions
	})
	return candidate(simulation.Simulate(bestDecisions))
}

func candidate(result simulation.Result) Candidate {
	return Candidate{
		Decisions: result.Decisions, FinalScore: *result.FinalScore,
		TotalCost: result.TotalCost, RemainingBudget: result.RemainingBudget,
		CriticalAfter: *result.CriticalAfter,
	}
}

// Improve returns the original engine result (including validation errors for
// invalid input) and up to three strictly better one-decision replacements.
// Replacing a measure allows any legal district for the new measure; replacing
// only a district keeps the measure. The other four decisions stay unchanged.
func Improve(decisions []simulation.Decision) (simulation.Result, []Improvement) {
	current := simulation.Simulate(decisions)
	if !current.Valid {
		return current, nil
	}
	s := simulation.DefaultScenario()
	options := decisionOptions(s)
	neighbors := make([]Improvement, 0)
	input := append([]simulation.Decision(nil), current.Decisions...)
	for i, original := range current.Decisions {
		for _, option := range options {
			if sameDecision(original, option) {
				continue
			}
			input[i] = option
			result := simulation.Simulate(input)
			if result.Valid && *result.FinalScore > *current.FinalScore {
				neighbors = append(neighbors, Improvement{
					Candidate: candidate(result), ScoreDelta: *result.FinalScore - *current.FinalScore,
				})
			}
		}
		input[i] = original
	}
	sort.Slice(neighbors, func(i, j int) bool {
		return better(neighbors[i].Candidate, neighbors[j].Candidate)
	})
	if len(neighbors) > 3 {
		neighbors = neighbors[:3]
	}
	return current, neighbors
}

func searchBest(s simulation.Scenario) Candidate {
	var best Candidate
	visitScenarios(s, func(decisions []simulation.Decision) {
		result := simulation.Simulate(decisions)
		if !result.Valid {
			return
		}
		if best.Decisions == nil || better(candidate(result), best) {
			best = candidate(result)
		}
	})
	return best
}

// visitScenarios chooses increasing measure indices (no duplicates or
// permutations), then every legal district. Budget, category limits and catalog
// incompatibilities prune partial scenarios before invoking the engine.
func visitScenarios(s simulation.Scenario, visit func([]simulation.Decision)) {
	decisions := make([]simulation.Decision, 0, s.RequiredDecisions)
	categories := make(map[simulation.Category]int)
	var walk func(int, int)
	walk = func(start, cost int) {
		if len(decisions) == s.RequiredDecisions {
			visit(decisions)
			return
		}
		needed := s.RequiredDecisions - len(decisions)
		for i := start; i <= len(s.Measures)-needed; i++ {
			measure := s.Measures[i]
			if cost+measure.Cost > s.Budget || categories[measure.Category] >= s.MaxMeasuresPerCategory {
				continue
			}
			for _, option := range measureOptions(s, measure) {
				if incompatible(s, decisions, option) {
					continue
				}
				decisions = append(decisions, option)
				categories[measure.Category]++
				walk(i+1, cost+measure.Cost)
				categories[measure.Category]--
				decisions = decisions[:len(decisions)-1]
			}
		}
	}
	walk(0, 0)
}

func incompatible(s simulation.Scenario, selected []simulation.Decision, next simulation.Decision) bool {
	for _, rule := range s.Incompatibilities {
		for _, previous := range selected {
			if !((previous.MeasureID == rule.MeasureIDs[0] && next.MeasureID == rule.MeasureIDs[1]) ||
				(previous.MeasureID == rule.MeasureIDs[1] && next.MeasureID == rule.MeasureIDs[0])) {
				continue
			}
			if !rule.SameDistrictOnly || (previous.DistrictID != nil && next.DistrictID != nil && *previous.DistrictID == *next.DistrictID) {
				return true
			}
		}
	}
	return false
}

func decisionOptions(s simulation.Scenario) []simulation.Decision {
	var options []simulation.Decision
	for _, measure := range s.Measures {
		options = append(options, measureOptions(s, measure)...)
	}
	return options
}

func measureOptions(s simulation.Scenario, measure simulation.Measure) []simulation.Decision {
	if measure.Scope == simulation.CityScope {
		return []simulation.Decision{{MeasureID: measure.ID}}
	}
	options := make([]simulation.Decision, 0, len(s.Districts))
	for _, district := range s.Districts {
		id := district.ID
		options = append(options, simulation.Decision{MeasureID: measure.ID, DistrictID: &id})
	}
	return options
}

func sameDecision(a, b simulation.Decision) bool {
	return a.MeasureID == b.MeasureID && districtID(a) == districtID(b)
}

func districtID(decision simulation.Decision) string {
	if decision.DistrictID == nil {
		return ""
	}
	return *decision.DistrictID
}

// Engine results already have canonical decisions. Exact float64 ties use the
// lexicographically smaller sequence of (measure_id, district_id), no epsilon.
func better(a, b Candidate) bool {
	if a.FinalScore != b.FinalScore {
		return a.FinalScore > b.FinalScore
	}
	for i, decision := range a.Decisions {
		other := b.Decisions[i]
		if decision.MeasureID != other.MeasureID {
			return decision.MeasureID < other.MeasureID
		}
		if districtID(decision) != districtID(other) {
			return districtID(decision) < districtID(other)
		}
	}
	return false
}
