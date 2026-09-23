package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hack-395e4fb2-ai4edu/internal/autopilot"
	"hack-395e4fb2-ai4edu/internal/optimizer"
	"hack-395e4fb2-ai4edu/internal/simulation"
)

type autopilotAgents struct{ entered chan struct{} }

func (a autopilotAgents) Estimate(string, json.RawMessage) (float64, error) { return .001, nil }
func (a autopilotAgents) Call(ctx context.Context, role string, _ json.RawMessage, out any) (autopilot.Usage, error) {
	if a.entered != nil {
		a.entered <- struct{}{}
		<-ctx.Done()
		return autopilot.Usage{Uncertain: true}, ctx.Err()
	}
	switch role {
	case "master", "auditor":
		*out.(*autopilot.Brief) = autopilot.Brief{Goal: optimizer.Goal{Objective: "score", BudgetLimit: 100, ProtectDistricts: []string{}}, Summary: "Лучший план"}
	case "planner":
		*out.(*autopilot.Proposal) = autopilot.Proposal{CandidateIndex: 0, Explanation: "Объяснение проверенного плана."}
	case "reviewer":
		*out.(*autopilot.Review) = autopilot.Review{Approved: true, RevisionTarget: "none", Feedback: "Проверка пройдена.", Tradeoffs: []string{}}
	}
	return autopilot.Usage{InputTokens: 1, OutputTokens: 1, EstimatedCostUSD: .0001}, nil
}

func TestAutopilotHTTPLifecycleWithoutDecisions(t *testing.T) {
	budget, _ := autopilot.NewBudget(1, "")
	service := autopilot.New(autopilotAgents{}, budget, autopilot.Config{Search: func(context.Context, optimizer.Goal, int) (optimizer.SearchResult, error) {
		var request simulation.Request
		_ = json.Unmarshal([]byte(goldenJSON), &request)
		return optimizer.SearchResult{Candidates: []simulation.Result{simulation.Simulate(request.Decisions)}, Exhaustive: true, Evaluated: 1, Feasible: 1}, nil
	}})
	defer service.Close()
	h := NewHandlerWithOptions(Options{Autopilot: service})
	w := call(h, "POST", "/api/autopilot", `{"goal":"Подготовь план"}`)
	var run autopilot.Run
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &run) != nil || run.ID == "" || w.Header().Get("Location") != "/api/autopilot/"+run.ID {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		w = call(h, "GET", "/api/autopilot/"+run.ID, "")
		if json.Unmarshal(w.Body.Bytes(), &run) != nil {
			t.Fatal("invalid job JSON")
		}
		if run.Status == "completed" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if w.Code != 200 || run.Status != "completed" || run.Result == nil || len(run.Result.Decisions) != 5 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = call(h, "POST", "/api/autopilot/"+run.ID+"/cancel", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"completed"`) {
		t.Fatal("cancel changed completed job")
	}
}

func TestAutopilotHTTPValidationAndDisabled(t *testing.T) {
	h := NewHandler()
	for _, tc := range []struct {
		body string
		code int
	}{{`{}`, 503}, {`null`, 400}, {`{"goal":5}`, 400}, {`{"decisions":[]}`, 400}, {`{} {}`, 400}, {strings.Repeat(" ", maxRequestBytes) + `{}`, 413}} {
		w := call(h, "POST", "/api/autopilot", tc.body)
		if w.Code != tc.code {
			t.Fatalf("got %d want %d", w.Code, tc.code)
		}
	}
	for _, path := range []string{"/api/autopilot/missing", "/api/autopilot/missing/cancel"} {
		method := "GET"
		if strings.HasSuffix(path, "cancel") {
			method = "POST"
		}
		if w := call(h, method, path, ""); w.Code != 404 {
			t.Fatal(w.Code)
		}
	}
}

func TestAutopilotHTTPBusyAndCancel(t *testing.T) {
	entered := make(chan struct{}, 1)
	budget, _ := autopilot.NewBudget(1, "")
	s := autopilot.New(autopilotAgents{entered}, budget, autopilot.Config{Concurrency: 1})
	defer s.Close()
	h := NewHandlerWithOptions(Options{Autopilot: s})
	w := call(h, "POST", "/api/autopilot", `{}`)
	var run autopilot.Run
	_ = json.Unmarshal(w.Body.Bytes(), &run)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("job not started")
	}
	w = call(h, "POST", "/api/autopilot", `{}`)
	if w.Code != 429 || w.Header().Get("Retry-After") != "5" {
		t.Fatal(w.Code)
	}
	w = call(h, "POST", "/api/autopilot/"+run.ID+"/cancel", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"cancelled"`) {
		t.Fatal(w.Body.String())
	}
	// Request-disconnect does not own the background run; explicit cancel does.
	r := httptest.NewRequest("GET", "/api/autopilot/"+run.ID, nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}
