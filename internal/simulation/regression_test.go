package simulation

import (
	"os"
	"strings"
	"testing"
)

func TestMeasureCreatesCriticalIndicator(t *testing.T) {
	r := Simulate([]Decision{decision("M9", "esil"), decision("M11", "almaty"), decision("M10", "esil"), decision("M12", ""), decision("M4", "esil")})
	if !r.Valid {
		t.Fatal(r.ValidationErrors)
	}
	near(t, r.DistrictBeforeAfter[1].After["T1"], 38.25, 1e-12)
	if *r.CriticalBefore != 2 || *r.CriticalAfter != 3 {
		t.Fatalf("critical count: %d -> %d, want 2 -> 3", *r.CriticalBefore, *r.CriticalAfter)
	}
	near(t, r.FinalBreakdown.CriticalPenalty, 3, 0)
}

func TestSynergyRequiresBothMeasures(t *testing.T) {
	for _, tt := range []struct {
		id, district string
		indicator    Indicator
		delta        float64
	}{
		{"M1", "nura", "T1", 4.5}, {"M2", "", "T1", 3},
		{"M10", "nura", "B1", 13.125}, {"M12", "", "B1", 2.625},
		{"M5", "nura", "E2", 8.75}, {"M6", "", "E2", 1.5},
	} {
		t.Run(tt.id, func(t *testing.T) {
			r := Simulate([]Decision{decision("M9", "nura"), decision("M11", "esil"), decision("M14", ""), decision("M4", "esil"), decision(tt.id, tt.district)})
			if !r.Valid || len(r.AppliedSynergies) != 0 {
				t.Fatalf("single member must not activate synergy: %+v", r)
			}
			near(t, r.IndicatorDeltas["nura"][tt.indicator], tt.delta, 1e-12)
		})
	}
}

func TestGoldenFullIndicatorVectors(t *testing.T) {
	want := map[string]Indicators{
		"esil":     values(45, 62, 68, 72, 48, 55, 78, 60, 75, 74.375),
		"almaty":   values(40, 75, 50, 55, 60, 65, 62, 52, 50, 64.375),
		"saryarka": values(50, 70, 42, 48.75, 62, 68, 58, 55, 47.5, 59.375),
		"baikonur": values(52, 68, 55, 50, 58, 60, 52, 58, 55, 62.375),
		"nura":     values(55, 40, 45, 65, 48, 43.75, 67.5, 51.75, 60, 54.375),
	}
	r := Simulate(golden())
	if !r.Valid || len(r.DistrictBeforeAfter) != len(want) {
		t.Fatalf("invalid result: %+v", r)
	}
	for _, d := range r.DistrictBeforeAfter {
		for k, expected := range want[d.DistrictID] {
			t.Run(d.DistrictID+"/"+string(k), func(t *testing.T) { near(t, d.After[k], expected, 1e-12) })
		}
	}
	near(t, *r.FinalScore, 56.54307, 1e-9)
}

func TestNamesMatchSpecification(t *testing.T) {
	data, err := os.ReadFile("../../docs/scoring.md")
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) > 4 {
			names[strings.TrimSpace(cells[1])] = strings.TrimSpace(cells[3])
		}
	}
	s := DefaultScenario()
	for _, m := range s.Measures {
		if m.Name == "" || m.Name != names[m.ID] {
			t.Errorf("%s name %q differs from specification %q", m.ID, m.Name, names[m.ID])
		}
	}
	for _, k := range s.IndicatorOrder {
		if s.IndicatorNames[k] == "" || s.IndicatorNames[k] != names[string(k)] {
			t.Errorf("%s name differs from specification", k)
		}
	}
}
