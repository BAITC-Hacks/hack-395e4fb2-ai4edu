package httpapi

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
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
