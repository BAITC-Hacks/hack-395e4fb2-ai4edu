package autopilot

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"hack-395e4fb2-ai4edu/internal/optimizer"
	"hack-395e4fb2-ai4edu/internal/simulation"
)

type fakeAgents struct {
	mu    sync.Mutex
	roles []string
	fn    func(context.Context, string, json.RawMessage, any) (Usage, error)
}

func (f *fakeAgents) Estimate(string, json.RawMessage) (float64, error) { return .02, nil }
func (f *fakeAgents) Call(ctx context.Context, role string, input json.RawMessage, out any) (Usage, error) {
	f.mu.Lock()
	f.roles = append(f.roles, role)
	f.mu.Unlock()
	if f.fn != nil {
		return f.fn(ctx, role, input, out)
	}
	fillRole(role, out)
	return Usage{Model: "fake", InputTokens: 100, OutputTokens: 100, EstimatedCostUSD: .001}, nil
}
func (f *fakeAgents) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.roles...)
}
func fillRole(role string, out any) {
	switch role {
	case "master":
		*out.(*Brief) = Brief{Goal: optimizer.Goal{Objective: "score", BudgetLimit: 100, ProtectDistricts: []string{}}, Summary: "Повысить Score"}
	case "planner":
		*out.(*Proposal) = Proposal{CandidateIndex: 0, Explanation: "План рассчитан по данным симулятора."}
	case "reviewer":
		*out.(*Review) = Review{Approved: true, RevisionTarget: "none", Feedback: "План соответствует цели.", Tradeoffs: []string{"Эффекты ограничены моделью."}}
	}
}
func testResult() simulation.Result {
	nura, saryarka := "nura", "saryarka"
	return simulation.Simulate([]simulation.Decision{{MeasureID: "M7", DistrictID: &nura}, {MeasureID: "M8", DistrictID: &nura}, {MeasureID: "M10", DistrictID: &nura}, {MeasureID: "M12"}, {MeasureID: "M5", DistrictID: &saryarka}})
}
func testSearch(context.Context, optimizer.Goal, int) (optimizer.SearchResult, error) {
	return optimizer.SearchResult{Candidates: []simulation.Result{testResult()}, Evaluated: 1, Feasible: 1, Exhaustive: true}, nil
}
func testService(t *testing.T, a Agents, c Config) *Service {
	t.Helper()
	b, err := NewBudget(50, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Search == nil {
		c.Search = testSearch
	}
	s := New(a, b, c)
	t.Cleanup(s.Close)
	return s
}
func awaitRun(t *testing.T, s *Service, id string) Run {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r, ok := s.Get(id)
		if !ok {
			t.Fatal("run missing")
		}
		if r.Status != "queued" && r.Status != "running" {
			return r
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("run did not terminate")
	return Run{}
}
func startRun(t *testing.T, s *Service) Run {
	t.Helper()
	r, err := s.Start(Request{Goal: "Подготовь лучший план"})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestMasterRevisionLoopAndVerifiedNumbers(t *testing.T) {
	reviews, plans := 0, 0
	a := &fakeAgents{fn: func(ctx context.Context, role string, in json.RawMessage, out any) (Usage, error) {
		fillRole(role, out)
		if role == "planner" || role == "reviewer" {
			var context struct {
				Candidates [][]resolvedAction `json:"candidate_actions"`
				Selected   []resolvedAction   `json:"selected_actions"`
			}
			if err := json.Unmarshal(in, &context); err != nil {
				t.Error(err)
			}
			actions := context.Selected
			if role == "planner" && len(context.Candidates) == 1 {
				actions = context.Candidates[0]
			}
			byID := map[string]resolvedAction{}
			for _, action := range actions {
				byID[action.MeasureID] = action
			}
			if len(actions) != 5 || byID["M7"].Name != "Школа + детсад (модульное строительство)" || byID["M7"].Location != "Нура" || byID["M12"].Location != "Весь город" {
				t.Errorf("%s did not receive the actual plan names and locations: %+v", role, actions)
			}
		}
		if role == "planner" {
			plans++
			if plans == 2 {
				var feedback struct {
					Previous *Review `json:"previous_review"`
				}
				if json.Unmarshal(in, &feedback) != nil || feedback.Previous == nil || feedback.Previous.Approved {
					t.Error("review not delegated back")
				}
				out.(*Proposal).Explanation = "Исправленное объяснение проверенных компромиссов."
			}
		}
		if role == "reviewer" {
			reviews++
			if reviews == 1 {
				out.(*Review).Approved = false
				out.(*Review).RevisionTarget = "planner"
				out.(*Review).Feedback = "Укажи компромиссы."
			}
		}
		return Usage{Model: "fake", InputTokens: 1, OutputTokens: 1, EstimatedCostUSD: .001}, nil
	}}
	s := testService(t, a, Config{Search: func(ctx context.Context, g optimizer.Goal, n int) (optimizer.SearchResult, error) {
		r, _ := testSearch(ctx, g, n)
		*r.Candidates[0].FinalScore = 9999
		return r, nil
	}})
	r := awaitRun(t, s, startRun(t, s).ID)
	if r.Status != "completed" || r.Iterations != 2 || r.Result == nil || math.Abs(*r.Result.FinalScore-56.54307) > 1e-8 || r.Review == nil || !r.Review.Approved {
		t.Fatalf("unexpected completed result: %+v", r)
	}
	if !reflect.DeepEqual(a.calls(), []string{"master", "planner", "reviewer", "planner", "reviewer"}) {
		t.Fatal(a.calls())
	}
	if math.Abs(r.EstimatedCostUSD-.005) > 1e-9 {
		t.Fatal("usage not aggregated", r.EstimatedCostUSD)
	}
	*r.Result.FinalScore = 9999
	r.Events[0].Message = "mutated"
	again, _ := s.Get(r.ID)
	if *again.Result.FinalScore == 9999 || again.Events[0].Message == "mutated" {
		t.Fatal("snapshot aliases job state")
	}
}

func TestAutopilotStopsWithoutFalseSuccess(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		modify       func(string, any)
		search       SearchFunc
		iterations   int
	}{
		{"clarification", "needs_clarification", func(role string, out any) {
			if role == "master" {
				out.(*Brief).Clarification = "Что означает запрет ухудшения: Score или каждый показатель?"
			}
		}, nil, 0},
		{"invalid_goal", "needs_clarification", func(role string, out any) {
			if role == "master" {
				out.(*Brief).Goal.BudgetLimit = 101
			}
		}, nil, 0},
		{"infeasible", "infeasible", nil, func(context.Context, optimizer.Goal, int) (optimizer.SearchResult, error) {
			return optimizer.SearchResult{Exhaustive: true, Candidates: []simulation.Result{}}, nil
		}, 0},
		{"incomplete_search", "failed", nil, func(context.Context, optimizer.Goal, int) (optimizer.SearchResult, error) {
			return optimizer.SearchResult{}, nil
		}, 0},
		{"rejected", "needs_review", func(role string, out any) {
			if role == "reviewer" {
				out.(*Review).Approved = false
				out.(*Review).RevisionTarget = "planner"
			}
		}, nil, 1},
		{"out_of_range", "needs_review", func(role string, out any) {
			if role == "planner" {
				out.(*Proposal).CandidateIndex = 999
			}
		}, nil, 1},
		{"repeated_proposal", "needs_review", func(role string, out any) {
			if role == "reviewer" {
				out.(*Review).Approved = false
				out.(*Review).RevisionTarget = "planner"
			}
		}, nil, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &fakeAgents{fn: func(_ context.Context, role string, _ json.RawMessage, out any) (Usage, error) {
				fillRole(role, out)
				if tc.modify != nil {
					tc.modify(role, out)
				}
				return Usage{InputTokens: 1, EstimatedCostUSD: .001}, nil
			}}
			s := testService(t, a, Config{Search: tc.search, MaxIterations: tc.iterations})
			r := awaitRun(t, s, startRun(t, s).ID)
			if r.Status != tc.status {
				t.Fatalf("got %s want %s", r.Status, tc.status)
			}
			if tc.name == "clarification" && len(a.calls()) != 1 {
				t.Fatal("asked other agents before clarification")
			}
			if tc.name == "repeated_proposal" && len(a.calls()) != 4 {
				t.Fatal("stalled loop did not stop", a.calls())
			}
		})
	}
}

func TestAutopilotBudgetBeforePaidRequest(t *testing.T) {
	a := &fakeAgents{}
	s := testService(t, a, Config{RunBudgetUSD: .01})
	r := awaitRun(t, s, startRun(t, s).ID)
	if r.Status != "budget_exceeded" || len(a.calls()) != 0 {
		t.Fatal("made API request without budget", r.Status, a.calls())
	}
}

func TestMasterRejectsFeasibleButWorsePlan(t *testing.T) {
	a := &fakeAgents{fn: func(_ context.Context, role string, _ json.RawMessage, out any) (Usage, error) {
		fillRole(role, out)
		if role == "planner" {
			out.(*Proposal).CandidateIndex = 1
		}
		return Usage{InputTokens: 1, EstimatedCostUSD: .001}, nil
	}}
	s := testService(t, a, Config{MaxIterations: 1, Search: func(context.Context, optimizer.Goal, int) (optimizer.SearchResult, error) {
		nura := "nura"
		best := simulation.Simulate([]simulation.Decision{{MeasureID: "M14"}, {MeasureID: "M2"}, {MeasureID: "M3", DistrictID: &nura}, {MeasureID: "M8", DistrictID: &nura}, {MeasureID: "M9", DistrictID: &nura}})
		return optimizer.SearchResult{Candidates: []simulation.Result{best, testResult()}, Exhaustive: true, Evaluated: 2, Feasible: 2}, nil
	}})
	r := awaitRun(t, s, startRun(t, s).ID)
	if r.Status != "needs_review" || len(a.calls()) != 2 || r.Result == nil || math.Abs(*r.Result.FinalScore-57.236735) > 1e-8 {
		t.Fatal("inferior plan bypassed master", r.Status, a.calls())
	}
}

func TestAutopilotCancellationConcurrencyAndUnknownUsage(t *testing.T) {
	entered := make(chan struct{})
	finished := make(chan struct{})
	a := &fakeAgents{fn: func(ctx context.Context, _ string, _ json.RawMessage, _ any) (Usage, error) {
		close(entered)
		<-ctx.Done()
		close(finished)
		return Usage{Uncertain: true}, ctx.Err()
	}}
	s := testService(t, a, Config{Concurrency: 1})
	initial := startRun(t, s)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("no request")
	}
	if _, err := s.Start(Request{}); !errors.Is(err, ErrBusy) {
		t.Fatal("unbounded concurrency", err)
	}
	r, ok := s.Cancel(initial.ID)
	if !ok || r.Status != "cancelled" {
		t.Fatal("cancel failed")
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("request ignored cancellation")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		r, _ = s.Get(initial.ID)
		if len(r.Usage) > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if r.Status != "cancelled" || len(r.Usage) != 1 || !r.Usage[0].Uncertain || math.Abs(r.EstimatedCostUSD-.02) > 1e-9 {
		t.Fatalf("lost uncertain cost on cancel: %+v", r)
	}
}

func TestAutopilotProviderFailureKeepsCheckedPlan(t *testing.T) {
	a := &fakeAgents{fn: func(_ context.Context, role string, _ json.RawMessage, out any) (Usage, error) {
		if role == "planner" {
			return Usage{Uncertain: true}, errors.New("secret raw provider error")
		}
		fillRole(role, out)
		return Usage{InputTokens: 1, EstimatedCostUSD: .001}, nil
	}}
	s := testService(t, a, Config{})
	r := awaitRun(t, s, startRun(t, s).ID)
	if r.Status != "unavailable" || r.Result == nil || !r.Result.Valid || r.Review != nil {
		t.Fatal("lost checked result or pretended success")
	}
	if r.Message == "secret raw provider error" {
		t.Fatal("leaked provider error")
	}
}

func TestAutopilotDeadline(t *testing.T) {
	a := &fakeAgents{fn: func(ctx context.Context, _ string, _ json.RawMessage, _ any) (Usage, error) {
		<-ctx.Done()
		return Usage{Uncertain: true}, ctx.Err()
	}}
	s := testService(t, a, Config{Timeout: 10 * time.Millisecond})
	r := awaitRun(t, s, startRun(t, s).ID)
	if r.Status != "timeout" {
		t.Fatal(r.Status)
	}
}

func TestReviewerRoutesGoalCorrectionToMaster(t *testing.T) {
	masters, reviews, searches := 0, 0, 0
	userGoal := "Максимизируй Score при бюджете не больше девяноста пяти"
	nura := "nura"
	best := simulation.Simulate([]simulation.Decision{{MeasureID: "M14"}, {MeasureID: "M2"}, {MeasureID: "M3", DistrictID: &nura}, {MeasureID: "M8", DistrictID: &nura}, {MeasureID: "M9", DistrictID: &nura}})
	a := &fakeAgents{fn: func(_ context.Context, role string, input json.RawMessage, out any) (Usage, error) {
		fillRole(role, out)
		switch role {
		case "master":
			masters++
			if masters == 2 {
				var received struct {
					Goal   string  `json:"user_goal"`
					Brief  *Brief  `json:"previous_brief"`
					Review *Review `json:"previous_review"`
				}
				if json.Unmarshal(input, &received) != nil || received.Goal != userGoal || received.Brief == nil || received.Brief.Goal.BudgetLimit != 100 || received.Review == nil || received.Review.RevisionTarget != "master" {
					t.Errorf("master did not receive original goal and correction: %s", input)
				}
				out.(*Brief).Goal.BudgetLimit = 95
			}
		case "reviewer":
			reviews++
			if reviews == 1 {
				*out.(*Review) = Review{Feedback: "Пользователь ограничил бюджет числом 95. Исправь формальную цель.", RevisionTarget: "master", Tradeoffs: []string{}}
			} else {
				out.(*Review).RevisionTarget = "none"
			}
		}
		return Usage{InputTokens: 1, OutputTokens: 1, EstimatedCostUSD: .001}, nil
	}}
	s := testService(t, a, Config{Search: func(_ context.Context, goal optimizer.Goal, _ int) (optimizer.SearchResult, error) {
		searches++
		result := best
		if goal.BudgetLimit == 95 {
			result = testResult()
		}
		return optimizer.SearchResult{Candidates: []simulation.Result{result}, Evaluated: searches, Feasible: 1, Exhaustive: true}, nil
	}})
	started, err := s.Start(Request{Goal: userGoal})
	if err != nil {
		t.Fatal(err)
	}
	r := awaitRun(t, s, started.ID)
	if r.Status != "completed" || r.Iterations != 2 || masters != 2 || searches != 2 || r.Result == nil || r.Result.TotalCost != 95 || r.Brief.Goal.BudgetLimit != 95 || r.Evaluated != 2 {
		t.Fatalf("goal correction did not trigger a new checked search: %+v", r)
	}
	if r.Optimality == nil || !r.Optimality.Proven || r.Optimality.ScoreCeiling == nil || *r.Optimality.ScoreCeiling != *testResult().FinalScore {
		t.Fatal("optimality certificate was not replaced after goal correction", r.Optimality)
	}
	if !reflect.DeepEqual(a.calls(), []string{"master", "planner", "reviewer", "master", "planner", "reviewer"}) {
		t.Fatal(a.calls())
	}
}

func TestMasterCorrectionFailureClearsStalePlan(t *testing.T) {
	for _, mode := range []string{"clarification", "provider_failure", "incomplete_search", "infeasible"} {
		t.Run(mode, func(t *testing.T) {
			masters, searches := 0, 0
			a := &fakeAgents{fn: func(_ context.Context, role string, _ json.RawMessage, out any) (Usage, error) {
				fillRole(role, out)
				if role == "master" {
					masters++
					if masters == 2 {
						if mode == "clarification" {
							out.(*Brief).Clarification = "Уточните ограничение."
						}
						if mode == "provider_failure" {
							return Usage{Uncertain: true}, errors.New("provider down")
						}
					}
				}
				if role == "reviewer" {
					*out.(*Review) = Review{RevisionTarget: "master", Feedback: "Неверно понята цель.", Tradeoffs: []string{}}
				}
				return Usage{InputTokens: 1, EstimatedCostUSD: .001}, nil
			}}
			s := testService(t, a, Config{Search: func(ctx context.Context, goal optimizer.Goal, limit int) (optimizer.SearchResult, error) {
				searches++
				if searches == 2 && mode == "incomplete_search" {
					return optimizer.SearchResult{}, nil
				}
				if searches == 2 && mode == "infeasible" {
					return optimizer.SearchResult{Exhaustive: true}, nil
				}
				return testSearch(ctx, goal, limit)
			}})
			r := awaitRun(t, s, startRun(t, s).ID)
			want := map[string]string{"clarification": "needs_clarification", "provider_failure": "unavailable", "incomplete_search": "failed", "infeasible": "infeasible"}[mode]
			if r.Status != want || r.Result != nil || r.Optimality != nil || r.Explanation != "" || r.Review != nil {
				t.Fatalf("stale plan survived master correction: %+v", r)
			}
		})
	}
}

func TestMasterCorrectionSharesIterationLimit(t *testing.T) {
	a := &fakeAgents{fn: func(_ context.Context, role string, _ json.RawMessage, out any) (Usage, error) {
		fillRole(role, out)
		if role == "reviewer" {
			*out.(*Review) = Review{RevisionTarget: "master", Feedback: "Исправь цель.", Tradeoffs: []string{}}
		}
		return Usage{InputTokens: 1, EstimatedCostUSD: .001}, nil
	}}
	s := testService(t, a, Config{MaxIterations: 2})
	r := awaitRun(t, s, startRun(t, s).ID)
	if r.Status != "needs_review" || r.Iterations != 2 || len(a.calls()) != 6 {
		t.Fatalf("master bypassed shared iteration limit: %+v calls=%v", r, a.calls())
	}
}

func TestMasterRejectsContradictoryReviewRouting(t *testing.T) {
	for _, review := range []Review{
		{Approved: true, RevisionTarget: "master", Feedback: "Исправить цель"},
		{Approved: false, RevisionTarget: "none", Feedback: "Не одобрено"},
		{Approved: true, Feedback: "Нет адресата"},
		{Approved: true, RevisionTarget: "none", Feedback: " "},
	} {
		a := &fakeAgents{fn: func(_ context.Context, role string, _ json.RawMessage, out any) (Usage, error) {
			fillRole(role, out)
			if role == "reviewer" {
				*out.(*Review) = review
			}
			return Usage{InputTokens: 1, EstimatedCostUSD: .001}, nil
		}}
		s := testService(t, a, Config{})
		r := awaitRun(t, s, startRun(t, s).ID)
		if r.Status != "unavailable" || r.Review != nil {
			t.Fatalf("invalid protocol was accepted: %+v", r)
		}
	}
}

func TestExplicitBudgetGuardRepairsBeforeSearch(t *testing.T) {
	masters, searches := 0, 0
	a := &fakeAgents{fn: func(_ context.Context, role string, input json.RawMessage, out any) (Usage, error) {
		fillRole(role, out)
		if role == "master" {
			masters++
			if masters == 2 {
				var correction struct {
					Previous *Review `json:"previous_review"`
				}
				if json.Unmarshal(input, &correction) != nil || correction.Previous == nil || correction.Previous.RevisionTarget != "master" {
					t.Error("numeric correction not delivered to master")
				}
				out.(*Brief).Goal.BudgetLimit = 95
			}
		}
		return Usage{InputTokens: 1, EstimatedCostUSD: .001}, nil
	}}
	s := testService(t, a, Config{Search: func(ctx context.Context, g optimizer.Goal, n int) (optimizer.SearchResult, error) {
		searches++
		if g.BudgetLimit != 95 {
			t.Errorf("wrong goal reached search: %+v", g)
		}
		return testSearch(ctx, g, n)
	}})
	initial, err := s.Start(Request{Goal: "Максимизируй Score при бюджете не больше 95"})
	if err != nil {
		t.Fatal(err)
	}
	r := awaitRun(t, s, initial.ID)
	if r.Status != "completed" || r.Iterations != 2 || masters != 2 || searches != 1 || !reflect.DeepEqual(a.calls(), []string{"master", "master", "planner", "reviewer"}) {
		t.Fatalf("numeric guard did not recover: %+v, calls=%v", r, a.calls())
	}
}

func TestExplicitBudgetGuardStopsPersistentMisreading(t *testing.T) {
	a := &fakeAgents{}
	s := testService(t, a, Config{MaxIterations: 2, Search: func(context.Context, optimizer.Goal, int) (optimizer.SearchResult, error) {
		t.Error("incorrect budget reached search")
		return optimizer.SearchResult{}, nil
	}})
	initial, err := s.Start(Request{Goal: "Бюджет: 95"})
	if err != nil {
		t.Fatal(err)
	}
	r := awaitRun(t, s, initial.ID)
	if r.Status != "needs_review" || r.Result != nil || r.Optimality != nil || !reflect.DeepEqual(a.calls(), []string{"master", "master"}) {
		t.Fatalf("invalid budget accepted: %+v calls=%v", r, a.calls())
	}
}

func TestInvalidExplicitBudgetNeedsClarificationWithoutPaidCalls(t *testing.T) {
	a := &fakeAgents{}
	s := testService(t, a, Config{})
	initial, err := s.Start(Request{Goal: "Бюджет 95.5"})
	if err != nil {
		t.Fatal(err)
	}
	r := awaitRun(t, s, initial.ID)
	if r.Status != "needs_clarification" || len(a.calls()) != 0 {
		t.Fatalf("unsupported numeric budget was silently altered: %+v", r)
	}
}
