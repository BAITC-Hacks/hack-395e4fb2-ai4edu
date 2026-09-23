// Package autopilot coordinates a master, planner and reviewer around verified
// simulator output. Only the Go engine can produce numerical plan results.
package autopilot

import (
	"context"
	"encoding/json"
	"time"

	"hack-395e4fb2-ai4edu/internal/optimizer"
	"hack-395e4fb2-ai4edu/internal/simulation"
)

type Request struct {
	Goal string `json:"goal"`
}

type Brief struct {
	Goal          optimizer.Goal `json:"goal"`
	Summary       string         `json:"summary"`
	Clarification string         `json:"clarification"`
}

type Proposal struct {
	CandidateIndex int    `json:"candidate_index"`
	Explanation    string `json:"explanation"`
}

type Review struct {
	Approved  bool     `json:"approved"`
	Feedback  string   `json:"feedback"`
	Tradeoffs []string `json:"tradeoffs"`
}

type Usage struct {
	Model            string  `json:"model"`
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	Uncertain        bool    `json:"uncertain"`
}

// Agents must honor context cancellation. Estimate reserves a conservative cost
// before a paid request; failed requests with unknown usage retain that reserve.
type Agents interface {
	Estimate(role string, input json.RawMessage) (float64, error)
	Call(ctx context.Context, role string, input json.RawMessage, output any) (Usage, error)
}

type Event struct {
	Agent     string    `json:"agent"`
	Stage     string    `json:"stage"`
	Message   string    `json:"message"`
	Iteration int       `json:"iteration"`
	At        time.Time `json:"at"`
}

type Run struct {
	ID               string             `json:"id"`
	Status           string             `json:"status"`
	Goal             string             `json:"goal"`
	Brief            *Brief             `json:"brief,omitempty"`
	Result           *simulation.Result `json:"result,omitempty"`
	Explanation      string             `json:"explanation"`
	Review           *Review            `json:"review,omitempty"`
	Events           []Event            `json:"events"`
	Usage            []Usage            `json:"usage"`
	EstimatedCostUSD float64            `json:"estimated_cost_usd"`
	BudgetUSD        float64            `json:"budget_usd"`
	Iterations       int                `json:"iterations"`
	Evaluated        int                `json:"evaluated"`
	Feasible         int                `json:"feasible"`
	Exhaustive       bool               `json:"exhaustive"`
	Message          string             `json:"message"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}
