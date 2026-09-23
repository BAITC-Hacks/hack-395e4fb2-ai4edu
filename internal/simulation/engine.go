package simulation

import (
	"math"
	"sort"
)

// Simulate is the only scenario execution entry point; invalid input never gets a score.
func Simulate(decisions []Decision) Result {
	return simulate(DefaultScenario(), decisions)
}

func simulate(s Scenario, input []Decision) Result {
	decisions := canonicalDecisions(input)
	cost, errors := validate(s, decisions)
	result := Result{
		Valid: len(errors) == 0, TotalCost: cost, RemainingBudget: s.Budget - cost,
		Decisions: decisions, ValidationErrors: errors, AppliedSynergies: []AppliedSynergy{},
	}
	if !result.Valid {
		return result
	}
	before := make(map[string]Indicators, len(s.Districts))
	after := make(map[string]Indicators, len(s.Districts))
	for _, d := range s.Districts {
		before[d.ID] = cloneIndicators(d.Indicators)
		after[d.ID] = cloneIndicators(d.Indicators)
	}
	measures := measureIndex(s)
	selected := make(map[string]Decision, len(decisions))
	for _, decision := range decisions {
		selected[decision.MeasureID] = decision
		m := measures[decision.MeasureID]
		factor := float64(s.Horizon-m.Lag) / float64(s.Horizon)
		effects := make(Indicators, len(m.Effects))
		for _, k := range s.IndicatorOrder {
			if delta, ok := m.Effects[k]; ok {
				effects[k] = delta * factor
			}
		}
		for _, district := range s.Districts {
			if m.Scope == DistrictScope && district.ID != *decision.DistrictID {
				continue
			}
			apply(after[district.ID], effects)
			result.AppliedEffects = append(result.AppliedEffects, AppliedEffect{
				MeasureID: m.ID, DistrictID: district.ID, LagFactor: factor, Effects: cloneIndicators(effects),
			})
		}
	}
	for _, synergy := range s.Synergies {
		_, hasA := selected[synergy.MeasureIDs[0]]
		_, hasB := selected[synergy.MeasureIDs[1]]
		if hasA && hasB {
			districtID := *selected[synergy.DistrictMeasureID].DistrictID
			apply(after[districtID], synergy.Effects) // Synergies have no lag multiplier.
			result.AppliedSynergies = append(result.AppliedSynergies, AppliedSynergy{
				MeasureIDs: synergy.MeasureIDs, DistrictID: districtID, Effects: cloneIndicators(synergy.Effects),
			})
		}
	}
	result.IndicatorDeltas = make(map[string]Indicators, len(s.Districts))
	for _, d := range s.Districts {
		clip(after[d.ID]) // Clip once, after all effects and bonuses have accumulated.
		deltas := make(Indicators, len(s.IndicatorOrder))
		for _, k := range s.IndicatorOrder {
			deltas[k] = after[d.ID][k] - before[d.ID][k]
		}
		result.IndicatorDeltas[d.ID] = deltas
		result.DistrictBeforeAfter = append(result.DistrictBeforeAfter, DistrictResult{
			DistrictID: d.ID, Name: d.Name, PopulationShare: d.PopulationShare,
			Before: before[d.ID], After: after[d.ID],
			ScoreBefore: districtScore(s, before[d.ID]), ScoreAfter: districtScore(s, after[d.ID]),
		})
	}
	base, criticalBefore, baseBreakdown := score(s, before)
	final, criticalAfter, finalBreakdown := score(s, after)
	delta := final - base
	result.BaseScore, result.FinalScore, result.ScoreDelta = &base, &final, &delta
	result.CriticalBefore, result.CriticalAfter = &criticalBefore, &criticalAfter
	result.BaseBreakdown, result.FinalBreakdown = &baseBreakdown, &finalBreakdown
	return result
}

func districtScore(s Scenario, indicators Indicators) float64 {
	var total float64
	// Never sum by map iteration: floating point accumulation order must be stable.
	for _, k := range s.IndicatorOrder {
		total += s.Weights[k] * indicators[k]
	}
	return total
}

func score(s Scenario, districts map[string]Indicators) (float64, int, ScoreBreakdown) {
	breakdown := ScoreBreakdown{MinimumDistrict: math.Inf(1)}
	critical := 0
	for _, d := range s.Districts {
		indicators := districts[d.ID]
		district := districtScore(s, indicators)
		breakdown.WeightedAverage += d.PopulationShare * district
		breakdown.MinimumDistrict = math.Min(breakdown.MinimumDistrict, district)
		for _, k := range s.IndicatorOrder {
			if indicators[k] < s.Scoring.CriticalThreshold {
				critical++
			}
		}
	}
	breakdown.CriticalPenalty = float64(critical) * s.Scoring.CriticalPenalty
	total := s.Scoring.AverageWeight*breakdown.WeightedAverage + s.Scoring.MinimumWeight*breakdown.MinimumDistrict - breakdown.CriticalPenalty
	return total, critical, breakdown
}

func apply(target, effects Indicators) {
	for k, delta := range effects {
		target[k] += delta
	}
}

func clip(indicators Indicators) {
	for k, value := range indicators {
		indicators[k] = math.Max(0, math.Min(100, value))
	}
}

func cloneIndicators(indicators Indicators) Indicators {
	copy := make(Indicators, len(indicators))
	for k, v := range indicators {
		copy[k] = v
	}
	return copy
}

func canonicalDecisions(input []Decision) []Decision {
	decisions := make([]Decision, len(input))
	for i, d := range input {
		decisions[i] = d
		if d.DistrictID != nil {
			id := *d.DistrictID
			decisions[i].DistrictID = &id
		}
	}
	sort.Slice(decisions, func(i, j int) bool {
		a, b := decisions[i], decisions[j]
		if a.MeasureID != b.MeasureID {
			return a.MeasureID < b.MeasureID
		}
		if a.DistrictID == nil || b.DistrictID == nil {
			if a.DistrictID != b.DistrictID {
				return a.DistrictID == nil
			}
			return !a.districtProvided && b.districtProvided
		}
		return *a.DistrictID < *b.DistrictID
	})
	return decisions
}
