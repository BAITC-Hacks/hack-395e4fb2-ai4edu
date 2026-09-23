package explanation

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"hack-395e4fb2-ai4edu/internal/simulation"
)

const Timeout = 10 * time.Second

// Client receives serialized data, never mutable engine state. Implementations
// must honor context cancellation. No network is needed to replace it in tests.
type Client interface {
	Generate(context.Context, json.RawMessage) (string, error)
}

type Explanation struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

type Service struct {
	client Client
}

func New(client Client) *Service {
	return &Service{client: client}
}

type payload struct {
	Result            simulation.Result               `json:"result"`
	Measures          []simulation.Measure            `json:"selected_measures"`
	IndicatorNames    map[simulation.Indicator]string `json:"indicator_names"`
	CategoryNames     map[simulation.Category]string  `json:"category_names"`
	CriticalThreshold float64                         `json:"critical_threshold"`
	Horizon           int                             `json:"horizon_quarters"`
}

// Explain accepts only validated engine results; HTTP validation happens before
// this boundary. The explanation can never replace scores or other result fields.
func (s *Service) Explain(ctx context.Context, result simulation.Result) Explanation {
	if !result.Valid {
		return Explanation{Source: "fallback", Text: "Сценарий не прошёл валидацию; объяснение недоступно."}
	}
	scenario := simulation.DefaultScenario()
	if s != nil && s.client != nil {
		selected := make(map[string]bool, len(result.Decisions))
		for _, d := range result.Decisions {
			selected[d.MeasureID] = true
		}
		input := payload{
			Result: result, IndicatorNames: scenario.IndicatorNames, CategoryNames: scenario.CategoryNames,
			CriticalThreshold: scenario.Scoring.CriticalThreshold, Horizon: scenario.Horizon,
		}
		for _, m := range scenario.Measures {
			if selected[m.ID] {
				input.Measures = append(input.Measures, m)
			}
		}
		data, err := json.Marshal(input)
		if err == nil {
			ctx, cancel := context.WithTimeout(ctx, Timeout)
			defer cancel()
			text, err := s.client.Generate(ctx, data)
			if err == nil && ctx.Err() == nil && strings.TrimSpace(text) != "" {
				return Explanation{Source: "llm", Text: strings.TrimSpace(text)}
			}
		}
	}
	return Explanation{Source: "fallback", Text: fallback(scenario, result)}
}
