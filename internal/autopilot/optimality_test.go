package autopilot

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"hack-395e4fb2-ai4edu/internal/optimizer"
	"hack-395e4fb2-ai4edu/internal/simulation"
)

func certificateScenario() simulation.Result {
	nura, saryarka := "nura", "saryarka"
	return simulation.Simulate([]simulation.Decision{
		{MeasureID: "M7", DistrictID: &nura}, {MeasureID: "M8", DistrictID: &nura},
		{MeasureID: "M10", DistrictID: &nura}, {MeasureID: "M12"}, {MeasureID: "M5", DistrictID: &saryarka},
	})
}

func TestOptimalityCertificateObjectivesAndExhaustiveness(t *testing.T) {
	for _, objective := range []string{"score", "weakest_district", "focus_district", "critical_first"} {
		t.Run(objective, func(t *testing.T) {
			goal := optimizer.Goal{Objective: objective, BudgetLimit: 100}
			if objective == "focus_district" {
				goal.FocusDistrict = "nura"
			}
			best, selected := certificateScenario(), certificateScenario()
			var want float64
			switch objective {
			case "score":
				want = *best.FinalScore
			case "weakest_district":
				want = best.FinalBreakdown.MinimumDistrict
			case "focus_district":
				for _, district := range best.DistrictBeforeAfter {
					if district.DistrictID == "nura" {
						want = district.ScoreAfter
					}
				}
			case "critical_first":
				want = float64(*best.CriticalAfter)
			}
			for _, exhaustive := range []bool{false, true} {
				got := certifyOptimality(goal, best, selected, exhaustive)
				if got == nil || got.Proven != exhaustive || got.Objective != objective || got.Gap != 0 || got.BestValue != want || got.SelectedValue != want {
					t.Fatalf("wrong certificate for exhaustive=%v: %+v", exhaustive, got)
				}
				if (got.ScoreCeiling != nil) != (objective == "score" && exhaustive) {
					t.Fatalf("incorrect Score ceiling: %+v", got)
				}
				if got.ScoreCeiling != nil && *got.ScoreCeiling != *best.FinalScore {
					t.Fatal("wrong Score ceiling")
				}
				if got.ScoreCeiling == nil {
					raw, err := json.Marshal(got)
					if err != nil || strings.Contains(string(raw), "score_ceiling") {
						t.Fatal("absent ceiling must not be serialized")
					}
				}
			}
		})
	}
}

func TestOptimalityCertificateInferiorAndContradictoryPlans(t *testing.T) {
	// Controlled engine-result copies isolate ranking dimensions without running
	// an exhaustive search. The certificate trusts the caller's verified winner.
	for _, objective := range []string{"score", "weakest_district", "focus_district", "critical_first"} {
		t.Run(objective, func(t *testing.T) {
			goal := optimizer.Goal{Objective: objective, BudgetLimit: 100}
			if objective == "focus_district" {
				goal.FocusDistrict = "nura"
			}
			best, selected := certificateScenario(), certificateScenario()
			switch objective {
			case "score":
				*selected.FinalScore -= 1
			case "weakest_district":
				selected.FinalBreakdown.MinimumDistrict -= 1
			case "focus_district":
				for i := range selected.DistrictBeforeAfter {
					if selected.DistrictBeforeAfter[i].DistrictID == "nura" {
						selected.DistrictBeforeAfter[i].ScoreAfter -= 1
					}
				}
			case "critical_first":
				*selected.CriticalAfter += 1
			}
			got := certifyOptimality(goal, best, selected, true)
			if got == nil || got.Proven || got.Gap != 1 {
				t.Fatalf("inferior result received false proof or wrong directional gap: %+v", got)
			}
			if got := certifyOptimality(goal, selected, best, true); got != nil {
				t.Fatalf("contradictory winner must not manufacture a proof/ceiling: %+v", got)
			}
		})
	}
}

func TestOptimalityCertificateOfficialScoreTieBreak(t *testing.T) {
	goal := optimizer.Goal{Objective: "weakest_district", BudgetLimit: 100}
	best, selected := certificateScenario(), certificateScenario()
	*selected.FinalScore = math.Nextafter(*best.FinalScore, math.Inf(-1))
	got := certifyOptimality(goal, best, selected, true)
	if got == nil || got.Proven || got.Gap != 0 || got.ScoreCeiling != nil {
		t.Fatalf("zero primary gap must not prove a worse official Score: %+v", got)
	}
	*selected.FinalScore = *best.FinalScore
	selected.Decisions[0].MeasureID = "other_tie_break_ID"
	got = certifyOptimality(goal, best, selected, true)
	if got == nil || !got.Proven {
		t.Fatal("numeric ties may differ in canonical decision ordering")
	}
}

func TestOptimalityCertificateInvalidData(t *testing.T) {
	goal := optimizer.Goal{Objective: "score", BudgetLimit: 100}
	for _, modify := range []func(*simulation.Result){
		func(r *simulation.Result) { r.Valid = false },
		func(r *simulation.Result) { r.FinalScore = nil },
		func(r *simulation.Result) { r.CriticalAfter = nil },
		func(r *simulation.Result) { r.FinalBreakdown = nil },
		func(r *simulation.Result) { r.TotalCost = 101 },
		func(r *simulation.Result) { *r.FinalScore = math.NaN() },
		func(r *simulation.Result) { *r.FinalScore = math.Inf(1) },
	} {
		best, bad := certificateScenario(), certificateScenario()
		modify(&bad)
		if certifyOptimality(goal, best, bad, true) != nil || certifyOptimality(goal, bad, best, true) != nil {
			t.Fatal("invalid/missing engine data produced a certificate")
		}
	}
	if certifyOptimality(optimizer.Goal{}, certificateScenario(), certificateScenario(), true) != nil {
		t.Fatal("invalid goal accepted")
	}
	for _, objective := range []string{"weakest_district", "focus_district", "critical_first"} {
		goal.Objective = objective
		best, bad := certificateScenario(), certificateScenario()
		switch objective {
		case "weakest_district":
			bad.FinalBreakdown.MinimumDistrict = math.NaN()
		case "focus_district":
			goal.FocusDistrict = "nura"
			bad.DistrictBeforeAfter = nil
		case "critical_first":
			goal.FocusDistrict = ""
			*bad.CriticalAfter = -1
		}
		if certifyOptimality(goal, best, bad, true) != nil {
			t.Fatal("invalid primary objective value accepted")
		}
	}
}

func TestOptimalityScoreCeilingDoesNotAliasInput(t *testing.T) {
	goal := optimizer.Goal{Objective: "score", BudgetLimit: 100}
	best, selected := certificateScenario(), certificateScenario()
	got := certifyOptimality(goal, best, selected, true)
	if got == nil || got.ScoreCeiling == nil {
		t.Fatal("missing exhaustive score ceiling")
	}
	want := *got.ScoreCeiling
	*best.FinalScore = -1
	if *got.ScoreCeiling != want {
		t.Fatal("certificate aliases mutable engine result")
	}
}
