package httpapi

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"hack-395e4fb2-ai4edu/internal/optimizer"
	"hack-395e4fb2-ai4edu/internal/simulation"
)

func improveJSON(body string) string {
	if body == `{}` {
		return `{"mode":"improve"}`
	}
	return `{"mode":"improve",` + strings.TrimPrefix(body, "{")
}

func TestRecommendBest(t *testing.T) {
	// All HTTP success tests share the one process-wide search via sync.Once.
	optimizer.Best()
	handler := NewHandler()
	want := call(handler, "POST", "/api/recommend", `{"mode":"best"}`)
	if want.Code != http.StatusOK {
		t.Fatalf("status %d: %s", want.Code, want.Body.String())
	}
	var response struct {
		Best optimizer.Candidate `json:"best"`
	}
	if err := json.Unmarshal(want.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	r := simulation.Simulate(response.Best.Decisions)
	if !r.Valid || *r.FinalScore != response.Best.FinalScore || response.Best.FinalScore < 56.54307 || response.Best.TotalCost != r.TotalCost {
		t.Fatalf("best differs from the engine: %s", want.Body.String())
	}
	bestRequest, err := json.Marshal(struct {
		Mode      string                `json:"mode"`
		Decisions []simulation.Decision `json:"decisions"`
	}{"improve", response.Best.Decisions})
	if err != nil {
		t.Fatal(err)
	}
	noImprovement := call(handler, "POST", "/api/recommend", string(bestRequest))
	if noImprovement.Code != http.StatusOK || !strings.Contains(noImprovement.Body.String(), `"improvements":[]`) {
		t.Fatalf("optimum must return an empty array: %s", noImprovement.Body.String())
	}
	for i := 0; i < 8; i++ {
		t.Run("deterministic best", func(t *testing.T) {
			t.Parallel()
			got := call(handler, "POST", "/api/recommend", `{"mode":"best"}`)
			if got.Code != want.Code || got.Body.String() != want.Body.String() {
				t.Fatal("concurrent best response changed")
			}
		})
	}
}

func TestRecommendImprove(t *testing.T) {
	handler := NewHandler()
	request := improveJSON(goldenJSON)
	want := call(handler, "POST", "/api/recommend", request)
	if want.Code != http.StatusOK {
		t.Fatalf("status %d: %s", want.Code, want.Body.String())
	}
	var response struct {
		CurrentScore float64                 `json:"current_score"`
		Improvements []optimizer.Improvement `json:"improvements"`
	}
	if err := json.Unmarshal(want.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	var input simulation.Request
	if err := json.Unmarshal([]byte(goldenJSON), &input); err != nil {
		t.Fatal(err)
	}
	current, improvements := optimizer.Improve(input.Decisions)
	expected, err := json.Marshal(improvements)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := json.Marshal(response.Improvements)
	if err != nil {
		t.Fatal(err)
	}
	if response.CurrentScore != *current.FinalScore || len(response.Improvements) != 3 || string(actual) != string(expected) {
		t.Fatalf("HTTP response differs from optimizer: %s", want.Body.String())
	}
	for i := 0; i < 8; i++ {
		t.Run("deterministic improve", func(t *testing.T) {
			t.Parallel()
			got := call(handler, "POST", "/api/recommend", request)
			if got.Code != want.Code || got.Body.String() != want.Body.String() {
				t.Fatal("concurrent improve response changed")
			}
		})
	}
}

func TestRecommendValidationMatchesSimulate(t *testing.T) {
	handler := NewHandler()
	for _, body := range []string{
		`{}`, `{"decisions":null}`, `{"decisions":[]}`,
		strings.Replace(goldenJSON, `"M7"`, `"M8"`, 1),
		strings.Replace(goldenJSON, `"M7"`, `"M99"`, 1),
		strings.Replace(goldenJSON, `"M12"`, `"M12","district_id":null`, 1),
		strings.Replace(goldenJSON, `"M12"`, `"M12","district_id":"nura"`, 1),
		strings.Replace(goldenJSON, `,"district_id":"nura"`, ``, 1),
		strings.Replace(goldenJSON, `"nura"`, `"unknown"`, 1),
		`{"decisions":[{"measure_id":"M3","district_id":"nura"},{"measure_id":"M5","district_id":"nura"},{"measure_id":"M7","district_id":"nura"},{"measure_id":"M8","district_id":"nura"},{"measure_id":"M13","district_id":"nura"}]}`,
		strings.Replace(goldenJSON, `"M10"`, `"M9"`, 1),
		strings.Replace(goldenJSON, `"M10"`, `"M4"`, 1),
	} {
		want := call(handler, "POST", "/api/simulate", body)
		got := call(handler, "POST", "/api/recommend", improveJSON(body))
		if want.Code != 422 || got.Code != want.Code || got.Body.String() != want.Body.String() {
			t.Fatalf("validation differs for %s: simulate %d %s; recommend %d %s", body, want.Code, want.Body.String(), got.Code, got.Body.String())
		}
	}
}

func TestRecommendBadRequests(t *testing.T) {
	handler := NewHandler()
	for _, body := range []string{
		"", "null", "[]", `{`, `{}`, `{"mode":null}`, `{"mode":"unknown"}`,
		`{"mode":"BEST"}`, `{"mode":"best","decisions":null}`,
		`{"mode":"best","decisions":[]}`, `{"mode":"best","extra":1}`,
		`{"mode":"best"}{}`, `{"mode":"improve","decisions":{}}`,
		improveJSON(strings.Replace(goldenJSON, `"nura"`, `12`, 1)),
		improveJSON(strings.Replace(goldenJSON, `"M12"`, `"M12","extra":1`, 1)),
	} {
		got := call(handler, "POST", "/api/recommend", body)
		if got.Code != 400 || !strings.Contains(got.Body.String(), `"code":"invalid_json"`) {
			t.Fatalf("body %s: status %d: %s", body, got.Code, got.Body.String())
		}
	}
	large := strings.Repeat(" ", maxRequestBytes) + `{"mode":"best"}`
	want := call(handler, "POST", "/api/simulate", large)
	got := call(handler, "POST", "/api/recommend", large)
	if got.Code != 413 || got.Body.String() != want.Body.String() {
		t.Fatal("body size error differs from simulate")
	}
	got = call(handler, "GET", "/api/recommend", "")
	if got.Code != 405 || got.Header().Get("Allow") != "POST" {
		t.Fatal("recommend must only allow POST")
	}
	got = call(handler, "OPTIONS", "/api/recommend", "")
	if got.Code != 204 || got.Body.Len() != 0 {
		t.Fatal("recommend preflight failed")
	}
}

func TestRecommendDocumentedExamples(t *testing.T) {
	optimizer.Best()
	doc := apiDocument(t)
	for _, name := range []string{"recommend-best", "recommend-improve"} {
		t.Run(name, func(t *testing.T) {
			got := call(NewHandler(), "POST", "/api/recommend", documentedJSON(t, doc, name+"-request"))
			want := jsonObject(t, documentedJSON(t, doc, name+"-response"))
			if got.Code != 200 || !reflect.DeepEqual(want, jsonObject(t, got.Body.String())) {
				t.Fatalf("docs/api.md %s example is stale: %d %s", name, got.Code, got.Body.String())
			}
		})
	}
}

func TestRecommendReadiness(t *testing.T) {
	// A closed channel publishes the controlled provider result. No sleeps,
	// polling or expensive search are needed to test the readiness transition.
	ready := make(chan struct{})
	var best optimizer.Candidate
	handler := NewHandlerWithOptions(Options{BestProvider: func() (optimizer.Candidate, bool) {
		select {
		case <-ready:
			return best, true
		default:
			return optimizer.Candidate{}, false
		}
	}})
	wantError := "{\"valid\":false,\"validation_errors\":[{\"code\":\"not_ready\",\"message\":\"Optimal scenario is still being computed\"}]}\n"
	if documented := documentedJSON(t, apiDocument(t), "recommend-not-ready-response"); !reflect.DeepEqual(jsonObject(t, documented), jsonObject(t, wantError)) {
		t.Fatal("documented not_ready response is stale")
	}
	check := func(status int, body string) {
		t.Helper()
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got := call(handler, "POST", "/api/recommend", `{"mode":"best"}`)
				if got.Code != status || got.Body.String() != body || got.Header().Get("Content-Type") != "application/json; charset=utf-8" {
					t.Errorf("readiness response: %d %s, want %d %s", got.Code, got.Body.String(), status, body)
				}
			}()
		}
		wg.Wait()
	}
	check(http.StatusServiceUnavailable, wantError)
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/scenario", ""},
		{"POST", "/api/simulate", goldenJSON},
		{"POST", "/api/explain", goldenJSON},
		{"POST", "/api/recommend", improveJSON(goldenJSON)},
	} {
		if got := call(handler, tc.method, tc.path, tc.body); got.Code != http.StatusOK {
			t.Errorf("%s must work before best is ready: %d %s", tc.path, got.Code, got.Body.String())
		}
	}
	// Only the provider is faked; the candidate numbers still come from the engine.
	var input simulation.Request
	if err := json.Unmarshal([]byte(goldenJSON), &input); err != nil {
		t.Fatal(err)
	}
	r := simulation.Simulate(input.Decisions)
	best = optimizer.Candidate{Decisions: r.Decisions, FinalScore: *r.FinalScore, TotalCost: r.TotalCost, RemainingBudget: r.RemainingBudget, CriticalAfter: *r.CriticalAfter}
	close(ready)
	want, err := json.Marshal(struct {
		Best optimizer.Candidate `json:"best"`
	}{best})
	if err != nil {
		t.Fatal(err)
	}
	check(http.StatusOK, string(want)+"\n")
}

func TestOnlyValidBestConsultsProvider(t *testing.T) {
	handler := NewHandlerWithOptions(Options{BestProvider: func() (optimizer.Candidate, bool) {
		t.Error("this request must not consult or wait for the optimum")
		return optimizer.Candidate{}, false
	}})
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/api/scenario", "", 200},
		{"POST", "/api/simulate", goldenJSON, 200},
		{"POST", "/api/explain", goldenJSON, 200},
		{"POST", "/api/recommend", improveJSON(goldenJSON), 200},
		{"POST", "/api/recommend", `{"mode":"improve","decisions":[]}`, 422},
		{"POST", "/api/recommend", `{"mode":"best","decisions":null}`, 400},
		{"POST", "/api/recommend", `{"mode":"unknown"}`, 400},
		{"POST", "/api/recommend", `{"mode":"best"}{}`, 400},
		{"POST", "/api/recommend", strings.Repeat(" ", maxRequestBytes) + `{"mode":"best"}`, 413},
		{"GET", "/api/recommend", "", 405},
		{"OPTIONS", "/api/recommend", "", 204},
	} {
		if got := call(handler, tc.method, tc.path, tc.body); got.Code != tc.status {
			t.Errorf("%s %s: %d, want %d", tc.method, tc.path, got.Code, tc.status)
		}
	}
}
