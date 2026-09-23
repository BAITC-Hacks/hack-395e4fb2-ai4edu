package advisor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"hack-395e4fb2-ai4edu/internal/simulation"
)

type plannerFunc func(context.Context, []json.RawMessage, bool, bool) (Turn, error)

func (f plannerFunc) Next(c context.Context, h []json.RawMessage, a, r bool) (Turn, error) {
	return f(c, h, a, r)
}

type reviewerFunc func(context.Context, json.RawMessage) (string, error)

func (f reviewerFunc) Generate(c context.Context, j json.RawMessage) (string, error) { return f(c, j) }

const baseArgs = `{"decisions":[{"measure_id":"M1","district_id":"esil"},{"measure_id":"M4","district_id":"saryarka"},{"measure_id":"M7","district_id":"nura"},{"measure_id":"M10","district_id":"nura"},{"measure_id":"M12","district_id":null}]}`
const betterArgs = `{"decisions":[{"measure_id":"M7","district_id":"nura"},{"measure_id":"M8","district_id":"nura"},{"measure_id":"M10","district_id":"nura"},{"measure_id":"M12","district_id":null},{"measure_id":"M5","district_id":"saryarka"}]}`

func base(t *testing.T) simulation.Result {
	t.Helper()
	r := evaluate(ToolCall{Name: "simulate_scenario", Arguments: baseArgs})
	if r.Result == nil || !r.Result.Valid {
		t.Fatal(r)
	}
	return *r.Result
}
func toolTurn(id, args string) Turn {
	call := ToolCall{ID: id, Name: "simulate_scenario", Arguments: args}
	return Turn{Calls: []ToolCall{call}, Output: []json.RawMessage{marshal(map[string]string{"type": "function_call", "call_id": id, "name": call.Name, "arguments": args})}}
}

func TestAdviceUsesVerifiedNumbersAndIndependentReview(t *testing.T) {
	original := base(t)
	snapshot := marshal(original)
	calls := 0
	reviews := 0
	planner := plannerFunc(func(ctx context.Context, history []json.RawMessage, allow, require bool) (Turn, error) {
		calls++
		if calls == 1 {
			if !allow || !require || !bytes.Contains(history[0], []byte("indicator_weights")) {
				t.Fatal("missing catalogue/tools")
			}
			return toolTurn("one", betterArgs), nil
		}
		if require || !allow {
			t.Fatal("incorrect second turn flags")
		}
		if len(history) != 3 || !bytes.Contains(history[2], []byte("best_verified_result")) {
			t.Fatal("verified tool result was not returned to planner")
		}
		var toolOutput struct {
			Output string `json:"output"`
		}
		var feedback struct {
			Best simulation.Result `json:"best_verified_result"`
		}
		if json.Unmarshal(history[2], &toolOutput) != nil || json.Unmarshal([]byte(toolOutput.Output), &feedback) != nil || feedback.Best.FinalScore == nil || math.Abs(*feedback.Best.FinalScore-56.54307) > 1e-8 {
			t.Fatal("wrong calculated feedback")
		}
		return Turn{Text: "Объяснение проверенного сценария."}, nil
	})
	reviewer := reviewerFunc(func(_ context.Context, input json.RawMessage) (string, error) {
		reviews++
		if !bytes.Contains(input, []byte("original_result")) || !bytes.Contains(input, []byte("recommended_result")) {
			t.Fatal("missing comparison")
		}
		return "Остались транспортные проблемы.", nil
	})
	got := New(planner, reviewer).Advise(context.Background(), original)
	if got.Status != "complete" || !got.Improved || got.Improvement <= 0 || got.Recommended.TotalCost != 95 || got.Analysis.Provider != "openai" || got.Review.Provider != "nvidia" || calls != 2 || reviews != 1 {
		t.Fatalf("unexpected advice: %+v", got)
	}
	if !bytes.Equal(snapshot, marshal(original)) || !bytes.Equal(snapshot, marshal(got.Result)) {
		t.Fatal("original result mutated")
	}
}

func TestInvalidProposalCanBeCorrected(t *testing.T) {
	calls := 0
	planner := plannerFunc(func(_ context.Context, h []json.RawMessage, a, r bool) (Turn, error) {
		calls++
		switch calls {
		case 1:
			return toolTurn("one", `{"decisions":[]}`), nil
		case 2:
			if !bytes.Contains(h[len(h)-1], []byte("decision_count")) {
				t.Fatal("missing validation feedback")
			}
			return toolTurn("two", betterArgs), nil
		default:
			if a || r {
				t.Fatal("tool limit not enforced")
			}
			return Turn{Text: "Исправлено после проверки."}, nil
		}
	})
	got := New(planner, nil).Advise(context.Background(), base(t))
	if !got.Improved || len(got.Evaluations) != 2 || got.Evaluations[0].Status != "invalid_scenario" || got.Status != "complete" {
		t.Fatalf("unexpected advice: %+v", got)
	}
}

func TestToolLimitAndNoRegression(t *testing.T) {
	calls := 0
	planner := plannerFunc(func(_ context.Context, _ []json.RawMessage, a, r bool) (Turn, error) {
		calls++
		return toolTurn(string(rune('a'+calls)), baseArgs), nil
	})
	original := *evaluate(ToolCall{Name: "simulate_scenario", Arguments: betterArgs}).Result
	got := New(planner, nil).Advise(context.Background(), original)
	if calls != 3 || len(got.Evaluations) != 2 || got.Improved || got.Improvement != 0 || got.Status != "partial" || *got.Recommended.FinalScore != *original.FinalScore {
		t.Fatalf("unbounded or regressed result: %+v", got)
	}
}

func TestProviderFailurePreservesCheckedImprovement(t *testing.T) {
	calls := 0
	planner := plannerFunc(func(context.Context, []json.RawMessage, bool, bool) (Turn, error) {
		calls++
		if calls == 1 {
			return toolTurn("one", betterArgs), nil
		}
		return Turn{}, errors.New("provider failed")
	})
	got := New(planner, reviewerFunc(func(context.Context, json.RawMessage) (string, error) { return "", errors.New("failed") })).Advise(context.Background(), base(t))
	if !got.Improved || got.Analysis.Source != "fallback" || got.Review.Source != "unavailable" || got.Status != "partial" {
		t.Fatal(got)
	}
}

func TestOfflineCancellationAndRefusal(t *testing.T) {
	var disabled *Service
	got := disabled.Advise(context.Background(), base(t))
	if got.Status != "disabled" || got.Improved || got.Analysis.Source != "fallback" {
		t.Fatal(got)
	}
	planner := plannerFunc(func(context.Context, []json.RawMessage, bool, bool) (Turn, error) {
		t.Fatal("cancelled input reached provider")
		return Turn{}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	New(planner, nil).Advise(ctx, base(t))
	refused := plannerFunc(func(context.Context, []json.RawMessage, bool, bool) (Turn, error) { return Turn{}, errRefused })
	review := reviewerFunc(func(context.Context, json.RawMessage) (string, error) {
		t.Fatal("refusal must not reroute to reviewer")
		return "", nil
	})
	if got := New(refused, review).Advise(context.Background(), base(t)); got.Status != "refused" {
		t.Fatal(got)
	}
}

func TestToolArguments(t *testing.T) {
	for _, args := range []string{`{`, baseArgs + `{}`, `{"decisions":[],"score":100}`, strings.Repeat("x", (64<<10)+1)} {
		if got := evaluate(ToolCall{Name: "simulate_scenario", Arguments: args}); got.Status != "invalid_arguments" || got.Result != nil {
			t.Fatal(got)
		}
	}
	if got := evaluate(ToolCall{Name: "run_shell", Arguments: baseArgs}); got.Status != "unknown_tool" {
		t.Fatal(got)
	}
	for _, args := range []string{strings.Replace(baseArgs, "M1", "M99", 1), strings.Replace(baseArgs, "esil", "missing", 1)} {
		if got := evaluate(ToolCall{Name: "simulate_scenario", Arguments: args}); got.Status != "invalid_scenario" {
			t.Fatal(got)
		}
	}
}
