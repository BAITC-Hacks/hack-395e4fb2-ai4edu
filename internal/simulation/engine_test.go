package simulation

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func decision(id, district string) Decision {
	d := Decision{MeasureID: id}
	if district != "" {
		d.DistrictID = &district
	}
	return d
}

func golden() []Decision {
	return []Decision{decision("M7", "nura"), decision("M8", "nura"), decision("M10", "nura"), decision("M12", ""), decision("M5", "saryarka")}
}

func near(t *testing.T, got, want, tolerance float64) {
	t.Helper()
	if math.IsNaN(got) || math.Abs(got-want) > tolerance {
		t.Fatalf("got %.12f, want %.12f (tolerance %g)", got, want, tolerance)
	}
}

func TestBaseScore(t *testing.T) {
	s := DefaultScenario()
	districts := make(map[string]Indicators)
	for _, d := range s.Districts {
		districts[d.ID] = d.Indicators
	}
	got, critical, breakdown := score(s, districts)
	near(t, got, 52.55768, 1e-9)
	near(t, breakdown.WeightedAverage, 56.8624, 1e-9)
	near(t, breakdown.MinimumDistrict, 49.18, 1e-9)
	if critical != 2 {
		t.Fatalf("critical = %d, want 2", critical)
	}
}

func TestGoldenScenario(t *testing.T) {
	r := Simulate(golden())
	if !r.Valid {
		t.Fatalf("invalid golden scenario: %+v", r.ValidationErrors)
	}
	near(t, *r.BaseScore, 52.55768, 1e-9)
	near(t, *r.FinalScore, 56.543, .001)
	near(t, *r.ScoreDelta, *r.FinalScore-*r.BaseScore, 1e-12)
	if r.TotalCost != 95 || r.RemainingBudget != 5 || *r.CriticalBefore != 2 || *r.CriticalAfter != 0 {
		t.Fatalf("unexpected totals: %+v", r)
	}
	near(t, r.IndicatorDeltas["nura"]["S1"], 10, 1e-12)
	near(t, r.IndicatorDeltas["nura"]["S2"], 8.75, 1e-12)
	near(t, r.IndicatorDeltas["nura"]["B1"], 12.5, 1e-12)
	near(t, r.IndicatorDeltas["saryarka"]["E2"], 8.75, 1e-12)
	for _, d := range r.DistrictBeforeAfter {
		near(t, r.IndicatorDeltas[d.DistrictID]["C2"], 4.375, 1e-12)
		for k, before := range d.Before {
			near(t, d.After[k]-before, r.IndicatorDeltas[d.DistrictID][k], 1e-12)
		}
	}
	t.Logf("base=%.8f final=%.8f delta=%.8f", *r.BaseScore, *r.FinalScore, *r.ScoreDelta)
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name      string
		decisions []Decision
		code      string
	}{
		{"four decisions", golden()[:4], "decision_count"},
		{"six decisions", append(golden(), decision("M11", "esil")), "decision_count"},
		{"duplicate", []Decision{decision("M10", "nura"), decision("M10", "esil"), decision("M12", ""), decision("M4", "nura"), decision("M9", "esil")}, "duplicate_measure"},
		{"budget", []Decision{decision("M3", "nura"), decision("M7", "nura"), decision("M8", "nura"), decision("M5", "esil"), decision("M12", "")}, "budget_exceeded"},
		{"global conflict different districts", []Decision{decision("M1", "esil"), decision("M3", "nura"), decision("M9", "nura"), decision("M10", "nura"), decision("M12", "")}, "incompatible_measures"},
		{"M4 M7 same district", []Decision{decision("M4", "nura"), decision("M7", "nura"), decision("M10", "nura"), decision("M12", ""), decision("M14", "")}, "incompatible_measures"},
		{"M4 M7 different districts", []Decision{decision("M4", "esil"), decision("M7", "nura"), decision("M10", "nura"), decision("M12", ""), decision("M14", "")}, ""},
		{"M5 M13 same district", []Decision{decision("M5", "nura"), decision("M13", "nura"), decision("M9", "esil"), decision("M10", "esil"), decision("M12", "")}, "incompatible_measures"},
		{"M5 M13 different districts", []Decision{decision("M5", "nura"), decision("M13", "esil"), decision("M9", "esil"), decision("M10", "esil"), decision("M12", "")}, ""},
		{"category limit", []Decision{decision("M7", "nura"), decision("M8", "nura"), decision("M9", "esil"), decision("M10", "esil"), decision("M12", "")}, "category_limit"},
		{"city with district", []Decision{decision("M7", "nura"), decision("M8", "nura"), decision("M10", "nura"), decision("M12", "esil"), decision("M5", "saryarka")}, "city_district_forbidden"},
		{"district missing", []Decision{decision("M7", ""), decision("M8", "nura"), decision("M10", "nura"), decision("M12", ""), decision("M5", "saryarka")}, "district_required"},
		{"unknown district", []Decision{decision("M7", "unknown"), decision("M8", "nura"), decision("M10", "nura"), decision("M12", ""), decision("M5", "saryarka")}, "unknown_district"},
		{"unknown measure", []Decision{decision("M99", "nura"), decision("M8", "nura"), decision("M10", "nura"), decision("M12", ""), decision("M5", "saryarka")}, "unknown_measure"},
		{"exact budget", []Decision{decision("M3", "esil"), decision("M7", "nura"), decision("M6", ""), decision("M10", "nura"), decision("M12", "")}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Simulate(tt.decisions)
			if tt.code == "" {
				if !r.Valid || r.FinalScore == nil {
					t.Fatalf("expected valid: %+v", r.ValidationErrors)
				}
				return
			}
			if r.Valid || r.FinalScore != nil || r.BaseScore != nil || r.ScoreDelta != nil || len(r.DistrictBeforeAfter) != 0 {
				t.Fatalf("invalid scenario must not be scored: %+v", r)
			}
			for _, err := range r.ValidationErrors {
				if err.Code == tt.code && err.Message != "" {
					return
				}
			}
			t.Fatalf("missing error %s: %+v", tt.code, r.ValidationErrors)
		})
	}
}

func TestSynergies(t *testing.T) {
	tests := []struct {
		name                    string
		decisions               []Decision
		pair                    [2]string
		indicator               Indicator
		targetDelta, otherDelta float64
	}{
		{"transport", []Decision{decision("M1", "nura"), decision("M2", ""), decision("M4", "esil"), decision("M9", "esil"), decision("M14", "")}, [2]string{"M1", "M2"}, "T1", 9.5, 3},
		{"safety", []Decision{decision("M10", "nura"), decision("M12", ""), decision("M1", "esil"), decision("M5", "esil"), decision("M8", "esil")}, [2]string{"M10", "M12"}, "B1", 12.5, 0},
		{"ecology", []Decision{decision("M5", "nura"), decision("M6", ""), decision("M1", "esil"), decision("M9", "esil"), decision("M14", "")}, [2]string{"M5", "M6"}, "E2", 12.25, 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Simulate(tt.decisions)
			if !r.Valid || len(r.AppliedSynergies) != 1 {
				t.Fatalf("expected one synergy: %+v", r)
			}
			bonus := r.AppliedSynergies[0]
			if bonus.MeasureIDs != tt.pair || bonus.DistrictID != "nura" {
				t.Fatalf("unexpected synergy: %+v", bonus)
			}
			near(t, bonus.Effects[tt.indicator], 2, 1e-12)
			near(t, r.IndicatorDeltas["nura"][tt.indicator], tt.targetDelta, 1e-12)
			near(t, r.IndicatorDeltas["almaty"][tt.indicator], tt.otherDelta, 1e-12)
		})
	}
}

func TestClippingAfterAllEffects(t *testing.T) {
	s := DefaultScenario()
	// Fixture near boundaries checks both negative effects and positive effects/bonuses.
	s.Districts[4].Indicators["T1"] = 1
	s.Districts[4].Indicators["B1"] = 99
	s.Districts[4].Indicators["B2"] = 99
	r := simulate(s, []Decision{decision("M11", "nura"), decision("M10", "nura"), decision("M12", ""), decision("M7", "nura"), decision("M8", "nura")})
	if !r.Valid {
		t.Fatal(r.ValidationErrors)
	}
	nura := r.DistrictBeforeAfter[4]
	near(t, nura.After["T1"], 0, 0)
	near(t, nura.After["B1"], 100, 0)
	near(t, nura.After["B2"], 100, 0)
	near(t, r.IndicatorDeltas["nura"]["B1"], 1, 0)
	// At the upper bound a positive and a negative T1 effect must be netted before clipping.
	s.Districts[4].Indicators["T1"] = 99
	r = simulate(s, []Decision{decision("M1", "nura"), decision("M11", "nura"), decision("M12", ""), decision("M9", "esil"), decision("M14", "")})
	if !r.Valid {
		t.Fatal(r.ValidationErrors)
	}
	near(t, r.DistrictBeforeAfter[4].After["T1"], 100, 0)
	for _, district := range r.DistrictBeforeAfter {
		for _, v := range district.After {
			if v < 0 || v > 100 {
				t.Fatalf("indicator out of bounds: %v", v)
			}
		}
	}
}

func TestCriticalThresholdStrict(t *testing.T) {
	s := DefaultScenario()
	districts := make(map[string]Indicators)
	for _, d := range s.Districts {
		districts[d.ID] = d.Indicators
	}
	districts["nura"]["S1"] = 40
	districts["nura"]["S2"] = 40
	_, critical, _ := score(s, districts)
	if critical != 0 {
		t.Fatalf("40 is not critical: %d", critical)
	}
	districts["nura"]["S1"] = 39.999
	_, critical, _ = score(s, districts)
	if critical != 1 {
		t.Fatalf("strictly below 40 is critical: %d", critical)
	}
}

func TestLagScopeAndNegativeEffect(t *testing.T) {
	r := Simulate([]Decision{decision("M3", "nura"), decision("M6", ""), decision("M11", "nura"), decision("M9", "esil"), decision("M14", "")})
	if !r.Valid || len(r.AppliedSynergies) != 0 {
		t.Fatalf("unexpected result: %+v", r)
	}
	near(t, r.IndicatorDeltas["nura"]["T1"], 6.25, 1e-12)
	near(t, r.IndicatorDeltas["nura"]["T2"], 10, 1e-12)
	near(t, r.IndicatorDeltas["nura"]["E2"], 3.5, 1e-12)
	near(t, r.IndicatorDeltas["nura"]["B2"], 10.5, 1e-12)
	near(t, r.IndicatorDeltas["esil"]["S1"], 2.625, 1e-12)
	near(t, r.IndicatorDeltas["almaty"]["T1"], 0, 0)
	for _, d := range r.DistrictBeforeAfter {
		near(t, r.IndicatorDeltas[d.DistrictID]["E1"], 2.5, 1e-12)
		near(t, r.IndicatorDeltas[d.DistrictID]["C1"], 4.375, 1e-12)
		near(t, r.IndicatorDeltas[d.DistrictID]["C2"], 1.75, 1e-12)
	}
}

func TestDecisionOrderAndIsolation(t *testing.T) {
	input := golden()
	before := golden()
	want, err := json.Marshal(Simulate(input))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, before) {
		t.Fatal("simulation mutated input")
	}
	var permute func(int)
	permute = func(i int) {
		if i == len(input) {
			got, err := json.Marshal(Simulate(input))
			if err != nil || string(got) != string(want) {
				t.Fatalf("result changed with decision order: %v", err)
			}
			return
		}
		for j := i; j < len(input); j++ {
			input[i], input[j] = input[j], input[i]
			permute(i + 1)
			input[i], input[j] = input[j], input[i]
		}
	}
	permute(0)
	s := DefaultScenario()
	s.Districts[0].Indicators["T1"] = 0
	near(t, DefaultScenario().Districts[0].Indicators["T1"], 45, 0)
}
