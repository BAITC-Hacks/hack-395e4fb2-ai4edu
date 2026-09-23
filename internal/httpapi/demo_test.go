package httpapi

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"hack-395e4fb2-ai4edu/internal/optimizer"
	"hack-395e4fb2-ai4edu/internal/simulation"
)

// Read the executable examples themselves, so editing a curl request cannot
// silently detach the presentation from the scenarios verified by this test.
func demoCurl(t *testing.T, doc, name string) (path, body string) {
	t.Helper()
	marker := "<!-- demo:" + name + "-curl -->\n```sh\n"
	if strings.Count(doc, marker) != 1 {
		t.Fatalf("expected one curl block for %s", name)
	}
	_, rest, _ := strings.Cut(doc, marker)
	command, _, ok := strings.Cut(rest, "\n```")
	if !ok {
		t.Fatalf("unclosed curl block for %s", name)
	}
	url := regexp.MustCompile(`http://localhost:8080(/api/[a-z]+)`).FindStringSubmatch(command)
	data := regexp.MustCompile(`(?s)--data-raw '([^']+)'`).FindStringSubmatch(command)
	if len(url) != 2 || len(data) != 2 || !json.Valid([]byte(data[1])) {
		t.Fatalf("invalid documented curl for %s: %s", name, command)
	}
	return url[1], data[1]
}

func checkDemoTableRow(t *testing.T, doc, name, label string, result simulation.Result) {
	t.Helper()
	base := name == "base"
	weakest := result.DistrictBeforeAfter[0]
	score, delta, cost, critical := *result.FinalScore, *result.ScoreDelta, result.TotalCost, *result.CriticalAfter
	minimum := result.FinalBreakdown.MinimumDistrict
	var decisions []string
	for _, d := range result.Decisions {
		district := "город"
		if d.DistrictID != nil {
			district = *d.DistrictID
		}
		decisions = append(decisions, d.MeasureID+"→"+district)
	}
	decisionText := strings.Join(decisions, "; ")
	if base {
		score, delta, cost, critical = *result.BaseScore, 0, 0, *result.CriticalBefore
		minimum = result.BaseBreakdown.MinimumDistrict
		decisionText = "Нет; состояние до мер"
	}
	for _, district := range result.DistrictBeforeAfter {
		if (base && district.ScoreBefore < weakest.ScoreBefore) || (!base && district.ScoreAfter < weakest.ScoreAfter) {
			weakest = district
		}
	}
	weakestScore := weakest.ScoreAfter
	if base {
		weakestScore = weakest.ScoreBefore
	}
	if weakestScore != minimum {
		t.Fatal("weakest district disagrees with engine breakdown")
	}
	want := fmt.Sprintf("| %s (`%s`) | %s | %d | %.8f | %.8f | %d | %d | %s (%s): %.8f |",
		label, name, decisionText, cost, score, delta, *result.CriticalBefore, critical, weakest.Name, weakest.DistrictID, weakestScore)
	if strings.Count(doc, want) != 1 {
		t.Errorf("demo table is stale; expected exactly one row:\n%s", want)
	}
}

func TestDemoScenarios(t *testing.T) {
	data, err := os.ReadFile("../../docs/demo.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	// Reuse the same sync.Once as the existing best/doc HTTP tests. Do not repeat
	// the full search per example, and do not replace it with a hardcoded winner.
	optimizer.Best()
	handler := NewHandler()
	results := make(map[string]simulation.Result)
	for _, tc := range []struct{ name, label string }{
		{"base", "База"}, {"golden", "Golden"}, {"optimal", "Оптимум"},
		{"skew", "Перекос"}, {"trap", "Ловушка"}, {"invalid", "Невалидный"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, body := demoCurl(t, doc, tc.name)
			if path != "/api/simulate" {
				t.Fatalf("%s must use /api/simulate", tc.name)
			}
			if (tc.name == "golden" || tc.name == "base") && !reflect.DeepEqual(jsonObject(t, body), jsonObject(t, goldenJSON)) {
				t.Fatal("base/golden example changed the golden decisions")
			}
			response := call(handler, "POST", path, body)
			status := 200
			if tc.name == "invalid" {
				status = 422
			}
			if response.Code != status {
				t.Fatalf("status %d, want %d: %s", response.Code, status, response.Body.String())
			}
			var result simulation.Result
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			actual := jsonObject(t, response.Body.String())
			documented := jsonObject(t, documentedJSON(t, doc, "demo-"+tc.name+"-response"))
			if tc.name == "invalid" {
				if result.Valid || len(result.ValidationErrors) != 1 || result.ValidationErrors[0].Code != "incompatible_measures" ||
					!reflect.DeepEqual(result.ValidationErrors[0].MeasureIDs, []string{"M1", "M3"}) {
					t.Fatal("invalid example must fail only because of M1+M3")
				}
				for _, field := range []string{"base_score", "final_score", "score_delta", "critical_after"} {
					if _, ok := actual[field]; ok {
						t.Errorf("invalid response contains %s", field)
					}
				}
				if !reflect.DeepEqual(documented, actual) {
					t.Fatal("documented 422 body differs from the handler")
				}
				return
			}
			if !result.Valid || result.FinalScore == nil || result.CriticalAfter == nil || result.BaseScore == nil || result.ScoreDelta == nil ||
				result.CriticalBefore == nil || result.BaseBreakdown == nil || result.FinalBreakdown == nil || len(result.DistrictBeforeAfter) == 0 {
				t.Fatal("incomplete valid engine response")
			}
			fields := []string{"total_cost", "final_score", "score_delta", "critical_before", "critical_after", "final_breakdown"}
			if tc.name == "base" {
				fields = []string{"base_score", "critical_before", "base_breakdown"}
			}
			projection := make(map[string]any)
			for _, field := range fields {
				projection[field] = actual[field]
			}
			if !reflect.DeepEqual(documented, projection) {
				t.Error("documented JSON excerpt differs from the handler")
			}
			checkDemoTableRow(t, doc, tc.name, tc.label, result)
			results[tc.name] = result
		})
	}
	if len(results) != 5 {
		t.Fatal("cannot check demo comparisons without all five engine results")
	}
	t.Run("best matches ordinary simulation", func(t *testing.T) {
		path, body := demoCurl(t, doc, "best")
		if path != "/api/recommend" || !reflect.DeepEqual(jsonObject(t, body), map[string]any{"mode": "best"}) {
			t.Fatal("optimum must come from /api/recommend best")
		}
		response := call(handler, "POST", path, body)
		if response.Code != 200 {
			t.Fatalf("best not ready after warmup: %d %s", response.Code, response.Body.String())
		}
		if !reflect.DeepEqual(jsonObject(t, response.Body.String()), jsonObject(t, documentedJSON(t, doc, "demo-best-response"))) {
			t.Fatal("documented best response differs from the handler")
		}
		var responseBody struct {
			Best optimizer.Candidate `json:"best"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &responseBody); err != nil {
			t.Fatal(err)
		}
		best, simulated := responseBody.Best, results["optimal"]
		if !reflect.DeepEqual(best.Decisions, simulated.Decisions) || best.FinalScore != *simulated.FinalScore || best.TotalCost != simulated.TotalCost || best.CriticalAfter != *simulated.CriticalAfter {
			t.Fatal("optimal demo scenario differs from the exhaustive optimizer")
		}
	})
	t.Run("skew leaves other districts behind", func(t *testing.T) {
		r, golden := results["skew"], results["golden"]
		for _, d := range r.Decisions {
			if d.DistrictID == nil || *d.DistrictID != "esil" {
				t.Fatal("skew must invest only in esil, without city measures")
			}
		}
		for _, d := range r.DistrictBeforeAfter {
			if d.DistrictID != "esil" && !reflect.DeepEqual(d.Before, d.After) {
				t.Fatal("skew changed a district other than esil")
			}
		}
		if *r.CriticalAfter != *r.CriticalBefore || r.FinalBreakdown.MinimumDistrict != r.BaseBreakdown.MinimumDistrict ||
			r.FinalBreakdown.WeightedAverage <= golden.FinalBreakdown.WeightedAverage || *r.FinalScore >= *golden.FinalScore {
			t.Fatal("skew no longer supports the documented comparison with golden")
		}
	})
	t.Run("M11 creates a new critical pair", func(t *testing.T) {
		r := results["trap"]
		newCritical := make([]string, 0)
		var scenario simulation.Scenario
		if err := json.Unmarshal(call(handler, "GET", "/api/scenario", "").Body.Bytes(), &scenario); err != nil {
			t.Fatal(err)
		}
		for _, d := range r.DistrictBeforeAfter {
			for id, after := range d.After {
				if d.Before[id] >= scenario.Scoring.CriticalThreshold && after < scenario.Scoring.CriticalThreshold {
					newCritical = append(newCritical, d.DistrictID+"/"+string(id))
				}
			}
			if d.DistrictID == "almaty" {
				for _, id := range []simulation.Indicator{"T1", "B2"} {
					row := fmt.Sprintf("| %s | %.8f | %.8f | %.8f |", id, d.Before[id], d.After[id], r.IndicatorDeltas[d.DistrictID][id])
					if strings.Count(doc, row) != 1 {
						t.Errorf("trap indicator table is stale; want %s", row)
					}
				}
			}
		}
		if !reflect.DeepEqual(newCritical, []string{"almaty/T1"}) || *r.CriticalAfter != 1 || r.FinalBreakdown.CriticalPenalty != 1 {
			t.Fatal("trap must create exactly one new critical pair: almaty/T1")
		}
	})
}
