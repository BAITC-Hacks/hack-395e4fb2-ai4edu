package autopilot

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"hack-395e4fb2-ai4edu/internal/optimizer"
)

// This opt-in, paid evaluation injects one wrong interpretation, then uses real
// master/planner/reviewer calls to check recovery after the numeric guard. It never runs in normal CI.
// The local $0.10 ceiling is separate from the server's persistent usage ledger.
func TestLiveMasterCorrection(t *testing.T) {
	if os.Getenv("RUN_PAID_AUTOPILOT_EVAL") != "1" {
		t.Skip("set RUN_PAID_AUTOPILOT_EVAL=1 with an exported OPENAI_API_KEY to run this paid evaluation")
	}
	a, err := NewAgentsFromEnv()
	if err != nil || a == nil {
		t.Fatal("live evaluation requires valid OpenAI configuration")
	}
	budget, err := NewBudget(.10, "")
	if err != nil {
		t.Fatal(err)
	}
	s := New(&incorrectFirstMaster{Agents: a}, budget, Config{RunBudgetUSD: .10, MaxIterations: 3, Concurrency: 1})
	defer s.Close()
	started, err := s.Start(Request{Goal: "Максимизируй городской Score при бюджете не больше 95. Устрани все критические показатели."})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(190 * time.Second)
	for time.Now().Before(deadline) {
		run, _ := s.Get(started.ID)
		if run.Status == "queued" || run.Status == "running" {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		routed := false
		for _, event := range run.Events {
			if event.Stage == "explicit_budget_correction" {
				routed = true
			}
		}
		if run.Status != "completed" || !routed || run.Iterations < 2 || run.Brief == nil || run.Brief.Goal.BudgetLimit != 95 || run.Result == nil || run.Result.TotalCost > 95 || run.Result.CriticalAfter == nil || *run.Result.CriticalAfter != 0 || run.Optimality == nil || !run.Optimality.Proven {
			t.Fatalf("live agents did not recover: status=%s routed=%v iteration=%d brief=%+v review=%+v events=%+v", run.Status, routed, run.Iterations, run.Brief, run.Review, run.Events)
		}
		t.Logf("fault injection recovered: iterations=%d provider_calls=%d score=%.8f cost=%d critical=%d estimated_usd=%.6f", run.Iterations, len(run.Usage)-1, *run.Result.FinalScore, run.Result.TotalCost, *run.Result.CriticalAfter, run.EstimatedCostUSD)
		return
	}
	t.Fatal("live evaluation timed out")
}

type incorrectFirstMaster struct {
	Agents
	injected bool
}

func (a *incorrectFirstMaster) Call(ctx context.Context, role string, input json.RawMessage, output any) (Usage, error) {
	if role == "master" && !a.injected {
		a.injected = true
		zero := 0
		*output.(*Brief) = Brief{Goal: optimizer.Goal{Objective: "score", BudgetLimit: 100, MaxCritical: &zero, ProtectDistricts: []string{}}, Summary: "Максимизировать Score при бюджете 100 и устранить критические показатели."}
		return Usage{Model: "test-fault-injection-not-an-api-call"}, nil
	}
	return a.Agents.Call(ctx, role, input, output)
}
