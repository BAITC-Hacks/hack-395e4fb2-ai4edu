package explanation

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"hack-395e4fb2-ai4edu/internal/simulation"
)

func fallback(s simulation.Scenario, r simulation.Result) string {
	type change struct {
		text  string
		delta float64
	}
	var gains []change
	var critical, losses []string
	weakest := r.DistrictBeforeAfter[0]
	districtNames := make(map[string]string)
	for _, d := range r.DistrictBeforeAfter {
		districtNames[d.DistrictID] = d.Name
		if d.ScoreAfter < weakest.ScoreAfter {
			weakest = d
		}
		for _, k := range s.IndicatorOrder {
			delta := r.IndicatorDeltas[d.DistrictID][k]
			label := fmt.Sprintf("%s — %s", d.Name, s.IndicatorNames[k])
			if delta > 0 {
				gains = append(gains, change{fmt.Sprintf("%s: +%s", label, number(delta)), delta})
			} else if delta < 0 {
				losses = append(losses, fmt.Sprintf("%s: %s", label, number(delta)))
			}
			if d.After[k] < s.Scoring.CriticalThreshold {
				critical = append(critical, fmt.Sprintf("%s: %s", label, number(d.After[k])))
			}
		}
	}
	sort.SliceStable(gains, func(i, j int) bool { return gains[i].delta > gains[j].delta })
	var strongest []string
	for i := 0; i < len(gains) && i < 3; i++ {
		strongest = append(strongest, gains[i].text)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Сильные стороны. Наибольшие улучшения показателей: %s. ", strings.Join(strongest, "; "))
	measureNames := make(map[string]string)
	selected := make(map[string]bool)
	for _, d := range r.Decisions {
		selected[d.MeasureID] = true
	}
	used := make(map[simulation.Category]bool)
	for _, m := range s.Measures {
		measureNames[m.ID] = m.Name
		if selected[m.ID] {
			used[m.Category] = true
		}
	}
	if len(r.AppliedSynergies) == 0 {
		out.WriteString("Синергии не активированы. ")
	}
	for _, bonus := range r.AppliedSynergies {
		var effects []string
		for _, k := range s.IndicatorOrder {
			if delta, ok := bonus.Effects[k]; ok {
				effects = append(effects, fmt.Sprintf("%s +%s", s.IndicatorNames[k], number(delta)))
			}
		}
		fmt.Fprintf(&out, "Синергия «%s» + «%s» в районе %s: %s. ",
			measureNames[bonus.MeasureIDs[0]], measureNames[bonus.MeasureIDs[1]], districtNames[bonus.DistrictID], strings.Join(effects, ", "))
	}
	fmt.Fprintf(&out, "\n\nРиски. Самый слабый район по итоговому районному Score — %s (%s). ", weakest.Name, number(weakest.ScoreAfter))
	if len(critical) == 0 {
		out.WriteString("Критических значений не осталось. ")
	} else {
		fmt.Fprintf(&out, "Оставшиеся критические значения: %s. ", strings.Join(critical, "; "))
	}
	if len(losses) != 0 {
		fmt.Fprintf(&out, "Ухудшения показателей: %s. ", strings.Join(losses, "; "))
	}
	var covered, uncovered []string
	for _, category := range []simulation.Category{simulation.Transport, simulation.Ecology, simulation.Social, simulation.Safety, simulation.Services} {
		if used[category] {
			covered = append(covered, s.CategoryNames[category])
		} else {
			uncovered = append(uncovered, s.CategoryNames[category])
		}
	}
	fmt.Fprintf(&out, "\n\nКомпромиссы. Неиспользованный бюджет: %d; остаток не даёт бонуса к Score. Выбраны меры направлений: %s. ", r.RemainingBudget, strings.Join(covered, ", "))
	if len(uncovered) != 0 {
		fmt.Fprintf(&out, "Нет выбранных мер направлений: %s. Это не исключает побочных эффектов на их показатели. ", strings.Join(uncovered, ", "))
	}
	out.WriteString("Эффекты учтены с лагом; бонусы синергий применены без лага.")
	return out.String()
}

// number hides float artifacts such as 52.962500000000006 in display text only.
func number(v float64) string {
	return strconv.FormatFloat(math.Round(v*1e4)/1e4, 'f', -1, 64)
}
