package autopilot

import (
	"math"

	"hack-395e4fb2-ai4edu/internal/optimizer"
	"hack-395e4fb2-ai4edu/internal/simulation"
)

// Optimality compares a selected plan with the exact search winner for the same
// goal and fixed catalog. It is not a real-world forecast or a percentage of city
// quality. For critical_first, the objective value is a count and lower is better.
// A zero primary Gap alone is insufficient: official Score also breaks ties.
type Optimality struct {
	Proven        bool     `json:"proven"`
	Objective     string   `json:"objective"`
	BestValue     float64  `json:"best_value"`
	SelectedValue float64  `json:"selected_value"`
	Gap           float64  `json:"gap"`
	ScoreCeiling  *float64 `json:"score_ceiling,omitempty"`
}

// certifyOptimality accepts already recalculated engine results. The caller is
// responsible for supplying the search winner and its actual exhaustive flag.
// Inconsistent winners, missing values, and invalid plans produce no certificate.
func certifyOptimality(goal optimizer.Goal, best, selected simulation.Result, exhaustive bool) *Optimality {
	if !optimizer.SatisfiesGoal(goal, best) || !optimizer.SatisfiesGoal(goal, selected) {
		return nil
	}
	bestValue, bestOK := certificateValue(goal, best)
	selectedValue, selectedOK := certificateValue(goal, selected)
	if !bestOK || !selectedOK || !finiteCertificateValue(*best.FinalScore) || !finiteCertificateValue(*selected.FinalScore) {
		return nil
	}
	gap := bestValue - selectedValue
	if goal.Objective == "critical_first" {
		gap = selectedValue - bestValue
	}
	// Never turn a contradictory "best" value into a zero-gap proof/ceiling.
	if gap < 0 || !finiteCertificateValue(gap) {
		return nil
	}
	result := &Optimality{
		Proven:    exhaustive && optimizer.EquallyRanked(goal, best, selected),
		Objective: goal.Objective, BestValue: bestValue,
		SelectedValue: selectedValue, Gap: gap,
	}
	if goal.Objective == "score" && exhaustive {
		ceiling := *best.FinalScore
		result.ScoreCeiling = &ceiling
	}
	return result
}

func certificateValue(goal optimizer.Goal, result simulation.Result) (float64, bool) {
	var value float64
	switch goal.Objective {
	case "score":
		if result.FinalScore == nil {
			return 0, false
		}
		value = *result.FinalScore
	case "weakest_district":
		if result.FinalBreakdown == nil {
			return 0, false
		}
		value = result.FinalBreakdown.MinimumDistrict
	case "focus_district":
		found := false
		for _, district := range result.DistrictBeforeAfter {
			if district.DistrictID == goal.FocusDistrict {
				value, found = district.ScoreAfter, true
				break
			}
		}
		if !found {
			return 0, false
		}
	case "critical_first":
		if result.CriticalAfter == nil || *result.CriticalAfter < 0 {
			return 0, false
		}
		value = float64(*result.CriticalAfter)
	default:
		return 0, false
	}
	return value, finiteCertificateValue(value)
}

func finiteCertificateValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
