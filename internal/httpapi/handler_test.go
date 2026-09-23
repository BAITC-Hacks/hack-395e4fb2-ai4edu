package httpapi

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hack-395e4fb2-ai4edu/internal/simulation"
)

const goldenJSON = `{"decisions":[{"measure_id":"M7","district_id":"nura"},{"measure_id":"M8","district_id":"nura"},{"measure_id":"M10","district_id":"nura"},{"measure_id":"M12"},{"measure_id":"M5","district_id":"saryarka"}]}`

func call(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestScenario(t *testing.T) {
	w := call(NewHandler(), "GET", "/api/scenario", "")
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("unexpected response: %d %v", w.Code, w.Header())
	}
	var s simulation.Scenario
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Districts) != 5 || len(s.Measures) != 14 || s.Horizon != 8 || s.Budget != 100 || s.RequiredDecisions != 5 || len(s.Synergies) != 3 || len(s.Incompatibilities) != 3 || len(s.Weights) != 10 {
		t.Fatalf("incomplete scenario: %+v", s)
	}
}

func TestSimulateGolden(t *testing.T) {
	w := call(NewHandler(), "POST", "/api/simulate", goldenJSON)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var r simulation.Result
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if !r.Valid || r.FinalScore == nil || math.Abs(*r.FinalScore-56.54307) > 1e-9 || r.TotalCost != 95 || r.RemainingBudget != 5 {
		t.Fatalf("unexpected result: %+v", r)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"valid", "total_cost", "remaining_budget", "base_score", "final_score", "score_delta", "critical_before", "critical_after", "district_before_after", "indicator_deltas", "applied_synergies", "applied_effects", "decisions", "base_breakdown", "final_breakdown"} {
		if _, ok := fields[field]; !ok {
			t.Errorf("missing field %s", field)
		}
	}
}

func TestInvalidRequestsHaveNoScore(t *testing.T) {
	tests := []struct {
		name, body, code string
		status           int
	}{
		{"empty", "", "invalid_json", 400},
		{"malformed", `{"decisions":`, "invalid_json", 400},
		{"null", "null", "invalid_json", 400},
		{"array", "[]", "invalid_json", 400},
		{"trailing object", goldenJSON + `{}`, "invalid_json", 400},
		{"trailing garbage", goldenJSON + `x`, "invalid_json", 400},
		{"unknown field", `{"decisions":[],"score":99}`, "invalid_json", 400},
		{"unknown decision field", `{"decisions":[{"measure_id":"M12","district":"nura"}]}`, "invalid_json", 400},
		{"wrong district type", strings.Replace(goldenJSON, `"nura"`, `12`, 1), "invalid_json", 400},
		{"missing decisions", `{}`, "decision_count", 422},
		{"null decisions", `{"decisions":null}`, "decision_count", 422},
		{"empty decisions", `{"decisions":[]}`, "decision_count", 422},
		{"null city district", strings.Replace(goldenJSON, `"measure_id":"M12"`, `"measure_id":"M12","district_id":null`, 1), "city_district_forbidden", 422},
		{"empty city district", strings.Replace(goldenJSON, `"measure_id":"M12"`, `"measure_id":"M12","district_id":""`, 1), "city_district_forbidden", 422},
		{"null district", strings.Replace(goldenJSON, `"nura"`, `null`, 1), "district_required", 422},
		{"empty district", strings.Replace(goldenJSON, `"nura"`, `""`, 1), "district_required", 422},
		{"oversize", strings.Repeat(" ", maxRequestBytes) + goldenJSON, "request_too_large", 413},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := call(NewHandler(), "POST", "/api/simulate", tt.body)
			if w.Code != tt.status {
				t.Fatalf("status %d, want %d: %s", w.Code, tt.status, w.Body.String())
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["valid"]) != "false" {
				t.Fatal("invalid request must return valid:false")
			}
			for _, field := range []string{"base_score", "final_score", "score_delta"} {
				if _, exists := fields[field]; exists {
					t.Errorf("invalid response includes %s", field)
				}
			}
			var errors []simulation.ValidationError
			if err := json.Unmarshal(fields["validation_errors"], &errors); err != nil {
				t.Fatal(err)
			}
			for _, err := range errors {
				if err.Code == tt.code && err.Message != "" {
					return
				}
			}
			t.Fatalf("missing error code %s: %+v", tt.code, errors)
		})
	}
}

func TestMethods(t *testing.T) {
	handler := NewHandler()
	for _, tc := range []struct{ method, path string }{{"POST", "/api/scenario"}, {"GET", "/api/simulate"}} {
		if w := call(handler, tc.method, tc.path, ""); w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s: status %d", tc.method, tc.path, w.Code)
		}
	}
}

func TestConcurrentRequests(t *testing.T) {
	handler := NewHandler()
	want := call(handler, "POST", "/api/simulate", goldenJSON).Body.String()
	for i := 0; i < 20; i++ {
		t.Run("isolated", func(t *testing.T) {
			t.Parallel()
			w := call(handler, "POST", "/api/simulate", goldenJSON)
			if w.Code != http.StatusOK || w.Body.String() != want {
				t.Fatal("concurrent simulation changed result")
			}
		})
	}
}
