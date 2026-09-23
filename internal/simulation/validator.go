package simulation

import "fmt"

func validate(s Scenario, decisions []Decision) (int, []ValidationError) {
	var errors []ValidationError
	add := func(code, message string, ids ...string) {
		errors = append(errors, ValidationError{Code: code, Message: message, MeasureIDs: ids})
	}
	if len(decisions) != s.RequiredDecisions {
		add("decision_count", fmt.Sprintf("Exactly %d decisions are required; got %d", s.RequiredDecisions, len(decisions)))
	}
	measures := measureIndex(s)
	districts := make(map[string]bool, len(s.Districts))
	for _, d := range s.Districts {
		districts[d.ID] = true
	}
	selected := make(map[string]Decision)
	categories := make(map[Category]int)
	totalCost := 0
	for _, d := range decisions {
		if _, exists := selected[d.MeasureID]; exists {
			add("duplicate_measure", "Measure may only be selected once", d.MeasureID)
		}
		selected[d.MeasureID] = d
		m, exists := measures[d.MeasureID]
		if !exists {
			add("unknown_measure", "Unknown measure ID", d.MeasureID)
			continue
		}
		totalCost += m.Cost
		categories[m.Category]++
		if m.Scope == CityScope && (d.DistrictID != nil || d.districtProvided) {
			add("city_district_forbidden", "District must be omitted for a city measure", d.MeasureID)
		}
		if m.Scope == DistrictScope {
			if d.DistrictID == nil || *d.DistrictID == "" {
				add("district_required", "District is required for a district measure", d.MeasureID)
			} else if !districts[*d.DistrictID] {
				add("unknown_district", fmt.Sprintf("Unknown district ID %q", *d.DistrictID), d.MeasureID)
			}
		}
	}
	if totalCost > s.Budget {
		add("budget_exceeded", fmt.Sprintf("Total cost %d exceeds budget %d", totalCost, s.Budget))
	}
	// Fixed category order keeps validation errors deterministic too.
	for _, category := range []Category{Transport, Ecology, Social, Safety, Services} {
		if categories[category] > s.MaxMeasuresPerCategory {
			add("category_limit", fmt.Sprintf("Category %s exceeds the limit of %d measures", category, s.MaxMeasuresPerCategory))
		}
	}
	for _, rule := range s.Incompatibilities {
		a, hasA := selected[rule.MeasureIDs[0]]
		b, hasB := selected[rule.MeasureIDs[1]]
		if !hasA || !hasB {
			continue
		}
		if rule.SameDistrictOnly {
			if a.DistrictID == nil || b.DistrictID == nil || *a.DistrictID != *b.DistrictID {
				continue
			}
		}
		message := "Measures are globally incompatible"
		if rule.SameDistrictOnly {
			message = "Measures are incompatible in the same district"
		}
		add("incompatible_measures", message, rule.MeasureIDs[:]...)
	}
	return totalCost, errors
}

func measureIndex(s Scenario) map[string]Measure {
	index := make(map[string]Measure, len(s.Measures))
	for _, m := range s.Measures {
		index[m.ID] = m
	}
	return index
}
