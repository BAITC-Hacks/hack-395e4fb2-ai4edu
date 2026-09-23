package autopilot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"hack-395e4fb2-ai4edu/internal/simulation"
)

type agentTransport func(*http.Request) (*http.Response, error)

func (f agentTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func configuredAgents(t *testing.T) *openAIAgents {
	t.Helper()
	for _, key := range []string{"OPENAI_MASTER_MODEL", "OPENAI_PLANNER_MODEL", "OPENAI_REVIEWER_MODEL", "OPENAI_AGENT_MODEL", "OPENAI_MODEL", "OPENAI_INPUT_USD_PER_MILLION", "OPENAI_OUTPUT_USD_PER_MILLION"} {
		t.Setenv(key, "")
	}
	t.Setenv("OPENAI_API_KEY", "test-only-secret")
	agents, err := NewAgentsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	return agents.(*openAIAgents)
}

const briefJSON = `{"goal":{"objective":"score","focus_district":"","budget_limit":100,"max_critical":null,"protect_districts":[]},"summary":"Повысить Score","clarification":""}`
const proposalJSON = `{"candidate_index":0,"explanation":"Проверенный вариант улучшает Score; ограничения модели сохраняются."}`
const reviewJSON = `{"approved":true,"feedback":"План соответствует цели и расчётам.","tradeoffs":["Синтетические данные."]}`

func agentResponse(status, text string) string {
	raw, _ := json.Marshal(map[string]any{
		"status": status,
		"usage":  map[string]int{"input_tokens": 1000, "output_tokens": 20},
		"output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]string{"type": "output_text", "text": text}}}},
	})
	return string(raw)
}

func agentHTTPResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

func TestAgentsStructuredProtocolAndUsage(t *testing.T) {
	for _, tc := range []struct {
		role, text, instruction string
		output                  any
	}{
		{"master", briefJSON, "Нельзя молча опускать", &Brief{}},
		{"planner", proposalJSON, "previous_review", &Proposal{}},
		{"reviewer", reviewJSON, "approved=true только", &Review{}},
	} {
		t.Run(tc.role, func(t *testing.T) {
			client := configuredAgents(t)
			input := json.RawMessage(`{"user_goal":"Помоги городу","scenario":{}}`)
			var sentBytes int
			client.http.Transport = agentTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != "https://api.openai.com/v1/responses" || r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-only-secret" || r.Header.Get("Content-Type") != "application/json" {
					t.Fatal("wrong endpoint, method or headers")
				}
				if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) > agentCallTimeout {
					t.Fatal("missing call deadline")
				}
				raw, _ := io.ReadAll(r.Body)
				sentBytes = len(raw)
				var request struct {
					Model        string `json:"model"`
					Store        *bool  `json:"store"`
					Instructions string `json:"instructions"`
					Input        string `json:"input"`
					MaxOutput    int    `json:"max_output_tokens"`
					Text         struct {
						Format struct {
							Type   string          `json:"type"`
							Name   string          `json:"name"`
							Strict bool            `json:"strict"`
							Schema json.RawMessage `json:"schema"`
						} `json:"format"`
					} `json:"text"`
				}
				if json.Unmarshal(raw, &request) != nil {
					t.Fatal("malformed request")
				}
				if request.Model != "gpt-4.1-mini" || request.Store == nil || *request.Store || request.Input != string(input) || request.MaxOutput != maxAgentOutputTokens {
					t.Fatal("wrong stateless model request")
				}
				if !strings.Contains(request.Instructions, tc.instruction) {
					t.Fatal("role instructions missing")
				}
				if request.Text.Format.Type != "json_schema" || !request.Text.Format.Strict || request.Text.Format.Name != "autopilot_"+tc.role || string(request.Text.Format.Schema) != string(schemaForRole(tc.role)) {
					t.Fatal("strict text.format schema missing")
				}
				return agentHTTPResponse(200, agentResponse("completed", tc.text)), nil
			})
			reservation, err := client.Estimate(tc.role, input)
			if err != nil {
				t.Fatal(err)
			}
			usage, err := client.Call(context.Background(), tc.role, input, tc.output)
			if err != nil {
				t.Fatal(err)
			}
			if usage.Uncertain || usage.Model != "gpt-4.1-mini" || usage.InputTokens != 1000 || usage.OutputTokens != 20 || math.Abs(usage.EstimatedCostUSD-.000432) > 1e-12 {
				t.Fatalf("wrong usage: %+v", usage)
			}
			wantReserve := (float64(sentBytes+1024)*.4 + 2048*1.6) / 1e6
			if reservation != wantReserve || reservation < usage.EstimatedCostUSD {
				t.Fatalf("bad reservation: %v want %v", reservation, wantReserve)
			}
			if client.http.Timeout != agentCallTimeout || client.http.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
				t.Fatal("timeout/redirect policy missing")
			}
		})
	}
}

func TestAgentsProviderFailuresRetainUsage(t *testing.T) {
	refusal := `{"status":"completed","usage":{"input_tokens":1000,"output_tokens":20},"output":[{"type":"message","role":"assistant","content":[{"type":"refusal","refusal":"test-only-secret"}]}]}`
	for _, tc := range []struct {
		name, body string
		status     int
		uncertain  bool
	}{
		{"refusal", refusal, 200, false},
		{"incomplete", agentResponse("incomplete", proposalJSON), 200, false},
		{"malformed output", agentResponse("completed", `{`), 200, false},
		{"empty output", agentResponse("completed", ` `), 200, false},
		{"missing output", `{"status":"completed","usage":{"input_tokens":1000,"output_tokens":20},"output":[]}`, 200, false},
		{"malformed message", `{"status":"completed","usage":{"input_tokens":1000,"output_tokens":20},"output":[{"type":"message","role":"assistant","content":123}]}`, 200, false},
		{"unknown output field", agentResponse("completed", `{"candidate_index":0,"explanation":"ok","extra":true}`), 200, false},
		{"trailing output", agentResponse("completed", proposalJSON+`{}`), 200, false},
		{"malformed envelope", "test-only-secret", 200, true},
		{"provider error", `{"error":{"message":"test-only-secret"}}`, 401, true},
		{"provider error with usage", agentResponse("failed", proposalJSON), 429, false},
		{"oversized", strings.Repeat("x", maxAgentResponseBytes+1), 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := configuredAgents(t)
			client.http.Transport = agentTransport(func(*http.Request) (*http.Response, error) { return agentHTTPResponse(tc.status, tc.body), nil })
			var output Proposal
			usage, err := client.Call(context.Background(), "planner", json.RawMessage(`{}`), &output)
			if err == nil || strings.Contains(err.Error(), "test-only-secret") || usage.Uncertain != tc.uncertain {
				t.Fatalf("unsafe or accepted failure: usage=%+v error=%v", usage, err)
			}
			if !tc.uncertain && (usage.InputTokens != 1000 || usage.EstimatedCostUSD <= 0) {
				t.Fatal("known billed usage was lost")
			}
			if output.Explanation != "" {
				t.Fatal("failed response changed output")
			}
		})
	}
}

func TestAgentsUnknownUsageRetainsReservation(t *testing.T) {
	for _, usage := range []any{nil, map[string]int{"input_tokens": 1000}, map[string]int{"input_tokens": -1, "output_tokens": 10}, map[string]int{"input_tokens": 0, "output_tokens": 0}} {
		client := configuredAgents(t)
		var body map[string]any
		_ = json.Unmarshal([]byte(agentResponse("completed", proposalJSON)), &body)
		body["usage"] = usage
		raw, _ := json.Marshal(body)
		client.http.Transport = agentTransport(func(*http.Request) (*http.Response, error) { return agentHTTPResponse(200, string(raw)), nil })
		var output Proposal
		got, err := client.Call(context.Background(), "planner", json.RawMessage(`{}`), &output)
		if err != nil || !got.Uncertain || got.EstimatedCostUSD != 0 || output.Explanation == "" {
			t.Fatalf("missing usage was treated as known: %+v %v", got, err)
		}
	}
}

func TestAgentsCancellationTransportAndDestinations(t *testing.T) {
	client := configuredAgents(t)
	client.http.Transport = agentTransport(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	var proposal Proposal
	usage, err := client.Call(ctx, "planner", json.RawMessage(`{}`), &proposal)
	if !errors.Is(err, context.DeadlineExceeded) || !usage.Uncertain {
		t.Fatalf("cancellation/usage: %+v %v", usage, err)
	}
	client.http.Transport = agentTransport(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport failed with test-only-secret")
	})
	if _, err = client.Call(context.Background(), "planner", json.RawMessage(`{}`), &proposal); err == nil || strings.Contains(err.Error(), "test-only-secret") {
		t.Fatal("transport error leaked")
	}
	client.http.Transport = agentTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid input reached provider")
		return nil, nil
	})
	for _, dst := range []any{nil, (*Proposal)(nil), &Brief{}, &map[string]any{}} {
		if _, err = client.Call(context.Background(), "planner", json.RawMessage(`{}`), dst); err == nil {
			t.Fatal("wrong output destination accepted")
		}
	}
	for _, tc := range []struct {
		role  string
		input json.RawMessage
	}{
		{"unknown", json.RawMessage(`{}`)},
		{"planner", json.RawMessage(`{`)},
		{"planner", json.RawMessage(`{"text":"` + strings.Repeat("x", maxAgentRequestBytes) + `"}`)},
	} {
		if _, err = client.Estimate(tc.role, tc.input); err == nil {
			t.Fatal("bad estimate input accepted")
		}
		if _, err = client.Call(context.Background(), tc.role, tc.input, &proposal); err == nil {
			t.Fatal("bad call input accepted")
		}
	}
}

func TestAgentsGoalAndVerdictValidation(t *testing.T) {
	for _, raw := range []string{
		briefJSON,
		strings.Replace(briefJSON, `"objective":"score","focus_district":""`, `"objective":"focus_district","focus_district":"nura"`, 1),
		strings.Replace(briefJSON, `"max_critical":null`, `"max_critical":0`, 1),
		strings.Replace(briefJSON, `"clarification":""`, `"clarification":"Какой из конфликтующих приоритетов важнее?"`, 1),
	} {
		if err := validateAgentOutput("master", []byte(raw)); err != nil {
			t.Fatalf("valid brief rejected: %v", err)
		}
	}
	for _, raw := range []string{
		`{}`,
		strings.Replace(briefJSON, `,"clarification":""`, "", 1),
		strings.Replace(briefJSON, `"budget_limit":100`, `"budget_limit":0`, 1),
		strings.Replace(briefJSON, `"budget_limit":100`, `"budget_limit":101`, 1),
		strings.Replace(briefJSON, `"budget_limit":100`, `"budget_limit":99.5`, 1),
		strings.Replace(briefJSON, `"max_critical":null`, `"max_critical":51`, 1),
		strings.Replace(briefJSON, `"max_critical":null`, `"max_critical":-1`, 1),
		strings.Replace(briefJSON, `"objective":"score"`, `"objective":"unknown"`, 1),
		strings.Replace(briefJSON, `"focus_district":""`, `"focus_district":"nura"`, 1),
		strings.Replace(briefJSON, `"objective":"score"`, `"objective":"focus_district"`, 1),
		strings.Replace(briefJSON, `"protect_districts":[]`, `"protect_districts":["nura","nura"]`, 1),
		strings.Replace(briefJSON, `"protect_districts":[]`, `"protect_districts":["unknown"]`, 1),
		strings.Replace(briefJSON, `"protect_districts":[]`, `"protect_districts":null`, 1),
		strings.Replace(briefJSON, `"summary":"Повысить Score"`, `"summary":" "`, 1),
	} {
		if err := validateAgentOutput("master", []byte(raw)); err == nil {
			t.Fatalf("invalid brief accepted: %s", raw)
		}
	}
	for _, raw := range []string{`{"candidate_index":-1,"explanation":"ok"}`, `{"candidate_index":5,"explanation":"ok"}`, `{"candidate_index":0.5,"explanation":"ok"}`, `{"candidate_index":0,"explanation":" "}`} {
		if validateAgentOutput("planner", []byte(raw)) == nil {
			t.Fatal("invalid proposal accepted")
		}
	}
	for _, raw := range []string{`{"approved":true,"feedback":"","tradeoffs":[]}`, `{"approved":null,"feedback":"ok","tradeoffs":[]}`, `{"approved":true,"feedback":"ok","tradeoffs":null}`, `{"approved":true,"feedback":"ok"}`} {
		if validateAgentOutput("reviewer", []byte(raw)) == nil {
			t.Fatal("invalid review accepted")
		}
	}
}

func TestAgentsModelConfiguration(t *testing.T) {
	client := configuredAgents(t)
	if client.roles["master"].model != "gpt-4.1-mini" {
		t.Fatal("wrong default")
	}
	t.Setenv("OPENAI_MODEL", "gpt-6-luna")
	t.Setenv("OPENAI_AGENT_MODEL", "gpt-6-sol")
	t.Setenv("OPENAI_REVIEWER_MODEL", "gpt-6-astra")
	agents, err := NewAgentsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	client = agents.(*openAIAgents)
	if client.roles["master"].model != "gpt-6-sol" || client.roles["planner"].rates.input != 2 || client.roles["reviewer"].model != "gpt-6-astra" || client.roles["reviewer"].rates.output != 50 {
		t.Fatal("wrong role precedence/prices")
	}
	t.Setenv("OPENAI_AGENT_MODEL", "")
	agents, err = NewAgentsFromEnv()
	if err != nil || agents.(*openAIAgents).roles["master"].model != "gpt-6-luna" {
		t.Fatal("OPENAI_MODEL fallback missing")
	}
	t.Setenv("OPENAI_MASTER_MODEL", "unknown-model")
	if _, err = NewAgentsFromEnv(); err == nil {
		t.Fatal("unknown pricing silently guessed")
	}
	t.Setenv("OPENAI_INPUT_USD_PER_MILLION", "3")
	t.Setenv("OPENAI_OUTPUT_USD_PER_MILLION", "12")
	agents, err = NewAgentsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	client = agents.(*openAIAgents)
	if client.roles["master"].rates != (tokenRates{3, 12}) || client.roles["reviewer"].rates.output != 50 {
		t.Fatal("custom fallback must not underprice known models")
	}
	for _, rates := range [][2]string{{"", "2"}, {"1", ""}, {"NaN", "2"}, {"1", "Inf"}, {"-1", "2"}, {"0", "2"}, {"1", "1000001"}} {
		t.Setenv("OPENAI_INPUT_USD_PER_MILLION", rates[0])
		t.Setenv("OPENAI_OUTPUT_USD_PER_MILLION", rates[1])
		if _, err = NewAgentsFromEnv(); err == nil {
			t.Fatal("invalid prices accepted")
		}
	}
	t.Setenv("OPENAI_API_KEY", " ")
	if agents, err = NewAgentsFromEnv(); err != nil || agents != nil {
		t.Fatal("empty key should disable agents without parsing other config")
	}
}

func TestAgentsFiveVerifiedCandidatesFitPayload(t *testing.T) {
	client := configuredAgents(t)
	district := "nura"
	result := simulation.Simulate([]simulation.Decision{{MeasureID: "M14"}, {MeasureID: "M2"}, {MeasureID: "M3", DistrictID: &district}, {MeasureID: "M8", DistrictID: &district}, {MeasureID: "M9", DistrictID: &district}})
	if !result.Valid {
		t.Fatal("test candidate invalid")
	}
	input, _ := json.Marshal(map[string]any{"candidates": []simulation.Result{result, result, result, result, result}, "user_goal": strings.Repeat("я", 2000), "previous_review": Review{Feedback: strings.Repeat("я", 2048), Tradeoffs: []string{}}, "iteration": 3})
	if _, err := client.Estimate("planner", input); err != nil {
		t.Fatalf("realistic candidates don't fit: %d bytes, %v", len(input), err)
	}
}
