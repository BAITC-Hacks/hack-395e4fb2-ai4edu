package autopilot

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"hack-395e4fb2-ai4edu/internal/optimizer"
)

func TestAuditorBlindCheckRepairsMissingConstraint(t *testing.T) {
	masters, searches := 0, 0
	zero := 0
	a := &fakeAgents{fn: func(_ context.Context, role string, input json.RawMessage, out any) (Usage, error) {
		fillRole(role, out)
		if role == "master" {
			masters++
			if masters > 1 {
				out.(*Brief).Goal.MaxCritical = &zero
			}
		}
		if role == "auditor" {
			var received map[string]json.RawMessage
			if json.Unmarshal(input, &received) != nil || len(received) != 2 || received["user_goal"] == nil || received["scenario"] == nil {
				t.Errorf("auditor was exposed to another agent's interpretation: %s", input)
			}
			out.(*Brief).Goal.MaxCritical = &zero
		}
		return Usage{InputTokens: 1, EstimatedCostUSD: .001}, nil
	}}
	s := testService(t, a, Config{Search: func(ctx context.Context, g optimizer.Goal, limit int) (optimizer.SearchResult, error) {
		searches++
		if g.MaxCritical == nil || *g.MaxCritical != 0 {
			t.Error("lost constraint reached search")
		}
		return testSearch(ctx, g, limit)
	}})
	r := awaitRun(t, s, startRun(t, s).ID)
	if r.Status != "completed" || r.Iterations != 2 || searches != 1 || r.GoalAudit == nil || !sameGoal(r.Brief.Goal, r.GoalAudit.Goal) {
		t.Fatalf("auditor did not trigger a bounded correction: %+v", r)
	}
	if !reflect.DeepEqual(a.calls(), []string{"master", "auditor", "master", "auditor", "planner", "reviewer"}) {
		t.Fatal(a.calls())
	}
}

func TestAuditorDisagreementAndClarificationNeverStartSearch(t *testing.T) {
	for _, clarification := range []bool{false, true} {
		a := &fakeAgents{fn: func(_ context.Context, role string, _ json.RawMessage, out any) (Usage, error) {
			fillRole(role, out)
			if role == "auditor" {
				out.(*Brief).Goal.Objective = "weakest_district"
				if clarification {
					out.(*Brief).Clarification = "Уточните приоритет."
				}
			}
			return Usage{InputTokens: 1, EstimatedCostUSD: .001}, nil
		}}
		s := testService(t, a, Config{MaxIterations: 2, Search: func(context.Context, optimizer.Goal, int) (optimizer.SearchResult, error) {
			t.Error("unagreed goal reached search")
			return optimizer.SearchResult{}, nil
		}})
		r := awaitRun(t, s, startRun(t, s).ID)
		want, calls := "needs_review", 4
		if clarification {
			want, calls = "needs_clarification", 2
		}
		if r.Status != want || r.Result != nil || r.Optimality != nil || len(a.calls()) != calls {
			t.Fatalf("auditor could be bypassed: %+v calls=%v", r, a.calls())
		}
	}
}

func TestGoalAgreementComparesConstraintsNotTextOrSetOrder(t *testing.T) {
	a := optimizer.Goal{Objective: "score", BudgetLimit: 100, ProtectDistricts: []string{"esil", "nura"}}
	b := a
	b.ProtectDistricts = []string{"nura", "esil"}
	if !sameGoal(a, b) {
		t.Fatal("district order changed set meaning")
	}
	zero := 0
	b.MaxCritical = &zero
	if sameGoal(a, b) {
		t.Fatal("missing critical constraint agreed with zero critical constraint")
	}
	if !reflect.DeepEqual(a.ProtectDistricts, []string{"esil", "nura"}) {
		t.Fatal("agreement mutated goal")
	}
}
