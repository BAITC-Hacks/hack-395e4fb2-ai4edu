// Package advisor runs bounded AI proposals through the deterministic simulator.
package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"hack-395e4fb2-ai4edu/internal/explanation"
	"hack-395e4fb2-ai4edu/internal/simulation"
)

const Timeout = 45 * time.Second
const MaxToolCalls = 2

type Narrative struct {
	Source   string `json:"source"`
	Provider string `json:"provider,omitempty"`
	Text     string `json:"text"`
}

type Evaluation struct {
	// Only public actions/results; no private reasoning or raw provider messages.
	Status string             `json:"status"`
	Result *simulation.Result `json:"result,omitempty"`
}

type Advice struct {
	Result      simulation.Result `json:"result"`
	Recommended simulation.Result `json:"recommended"`
	Improvement float64           `json:"improvement"`
	Improved    bool              `json:"improved"`
	Status      string            `json:"status"`
	Evaluations []Evaluation      `json:"evaluations"`
	Analysis    Narrative         `json:"analysis"`
	Review      Narrative         `json:"review"`
}

type Service struct {
	planner  Planner
	reviewer explanation.Client
}

func New(planner Planner, reviewer explanation.Client) *Service {
	return &Service{planner: planner, reviewer: reviewer}
}

// Advise accepts a validated server result. Candidates cannot mutate that result
// and are never automatically applied to the user's decisions.
func (s *Service) Advise(ctx context.Context, original simulation.Result) Advice {
	out := Advice{Result: original, Recommended: original, Status: "disabled", Evaluations: []Evaluation{}, Review: Narrative{Source: "disabled", Text: "Независимая AI-проверка не настроена."}}
	if !original.Valid || original.FinalScore == nil {
		out.Status = "invalid"
		out.Analysis = Narrative{Source: "fallback", Text: "Исходный сценарий не прошёл проверку."}
		return out
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	var template *explanation.Service
	baseText := template.Explain(ctx, original)
	out.Analysis = Narrative{Source: "fallback", Text: baseText.Text}
	if s != nil && s.planner != nil {
		out.Status = "partial"
		initial := map[string]any{"scenario": simulation.DefaultScenario(), "original_result": original}
		history := []json.RawMessage{message("user", string(marshal(initial)))}
		callIDs := map[string]bool{}
		for round := 0; round <= MaxToolCalls; round++ {
			if ctx.Err() != nil {
				break
			}
			stepCtx, stepCancel := context.WithTimeout(ctx, 10*time.Second)
			turn, err := s.planner.Next(stepCtx, history, round < MaxToolCalls, round == 0)
			stepExpired := stepCtx.Err() != nil
			stepCancel()
			if err != nil || stepExpired {
				if errors.Is(err, errRefused) {
					out.Status = "refused"
				} else if len(out.Evaluations) == 0 {
					out.Status = "unavailable"
				}
				break
			}
			if len(turn.Calls) == 0 {
				// The first turn must use the simulator; don't treat unsupported
				// tool calling as a successful optimization.
				if round > 0 && strings.TrimSpace(turn.Text) != "" {
					out.Analysis = Narrative{Source: "llm", Provider: "openai", Text: turn.Text}
					out.Status = "complete"
				}
				break
			}
			if round == MaxToolCalls || len(turn.Calls) != 1 {
				break
			}
			call := turn.Calls[0]
			if call.ID == "" || callIDs[call.ID] {
				break
			}
			callIDs[call.ID] = true
			history = append(history, turn.Output...)
			eval := evaluate(call)
			out.Evaluations = append(out.Evaluations, eval)
			if eval.Result != nil && eval.Result.Valid && *eval.Result.FinalScore > *out.Recommended.FinalScore {
				out.Recommended = *eval.Result
			}
			feedback := map[string]any{"evaluation": eval, "best_verified_result": out.Recommended,
				"improvement": *out.Recommended.FinalScore - *original.FinalScore, "checks_remaining": MaxToolCalls - round - 1}
			feedback["comparison"] = map[string]any{
				"original_final_score":            original.FinalScore,
				"recommended_final_score":         out.Recommended.FinalScore,
				"original_cost":                   original.TotalCost,
				"recommended_cost":                out.Recommended.TotalCost,
				"original_critical_indicators":    original.CriticalAfter,
				"recommended_critical_indicators": out.Recommended.CriticalAfter,
				"original_breakdown":              original.FinalBreakdown,
				"recommended_breakdown":           out.Recommended.FinalBreakdown,
			}
			history = append(history, marshal(map[string]any{"type": "function_call_output", "call_id": call.ID, "output": string(marshal(feedback))}))
		}
	}
	out.Improvement = *out.Recommended.FinalScore - *original.FinalScore
	out.Improved = out.Improvement > 0
	if out.Analysis.Source != "llm" {
		text := template.Explain(ctx, out.Recommended)
		out.Analysis = Narrative{Source: "fallback", Text: text.Text}
	}
	if s != nil && s.reviewer != nil && out.Status != "refused" && ctx.Err() == nil {
		out.Review = Narrative{Source: "unavailable", Provider: "nvidia", Text: "Независимая AI-проверка недоступна; расчёт сохранён."}
		reviewInput := marshal(map[string]any{"scenario": simulation.DefaultScenario(), "original_result": original, "recommended_result": out.Recommended, "improvement": out.Improvement})
		reviewCtx, reviewCancel := context.WithTimeout(ctx, 10*time.Second)
		text, err := s.reviewer.Generate(reviewCtx, reviewInput)
		if err == nil && reviewCtx.Err() == nil && strings.TrimSpace(text) != "" {
			out.Review = Narrative{Source: "llm", Provider: "nvidia", Text: strings.TrimSpace(text)}
		}
		reviewCancel()
	}
	return out
}

func evaluate(call ToolCall) Evaluation {
	if call.Name != "simulate_scenario" {
		return Evaluation{Status: "unknown_tool"}
	}
	if len(call.Arguments) > 64<<10 {
		return Evaluation{Status: "invalid_arguments"}
	}
	// Tool schema requires district_id:null for city measures. Convert to the
	// simulator's canonical representation, which omits it for city measures.
	var args struct {
		Decisions []struct {
			MeasureID  string  `json:"measure_id"`
			DistrictID *string `json:"district_id"`
		} `json:"decisions"`
	}
	decoder := json.NewDecoder(strings.NewReader(call.Arguments))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&args) != nil {
		return Evaluation{Status: "invalid_arguments"}
	}
	if decoder.Decode(new(any)) != io.EOF {
		return Evaluation{Status: "invalid_arguments"}
	}
	decisions := make([]simulation.Decision, len(args.Decisions))
	for i, d := range args.Decisions {
		decisions[i] = simulation.Decision{MeasureID: d.MeasureID, DistrictID: d.DistrictID}
	}
	result := simulation.Simulate(decisions)
	status := "valid"
	if !result.Valid {
		status = "invalid_scenario"
	}
	return Evaluation{Status: status, Result: &result}
}

// Only internally constructed values with JSON-compatible types reach marshal.
func marshal(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
func message(role, content string) json.RawMessage {
	return marshal(map[string]string{"role": role, "content": content})
}
