package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPlannerProtocol(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "secret")
	t.Setenv("OPENAI_AGENT_MODEL", "agent-model")
	c := NewPlannerFromEnv().(*openAIPlanner)
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer secret" || r.URL.String() != "https://api.openai.com/v1/responses" {
			t.Fatal("wrong auth/endpoint")
		}
		var body map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Fatal("bad body")
		}
		if string(body["tool_choice"]) != `"required"` || string(body["store"]) != "false" || string(body["parallel_tool_calls"]) != "false" || string(body["model"]) != `"agent-model"` {
			t.Fatal("wrong request flags")
		}
		turn := toolTurn("one", baseArgs)
		raw := marshal(map[string]any{"status": "completed", "output": append([]json.RawMessage{json.RawMessage(`{"type":"reasoning","encrypted_content":"opaque"}`)}, turn.Output...)})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
	})
	got, err := c.Next(context.Background(), []json.RawMessage{message("user", "{}")}, true, true)
	if err != nil || len(got.Calls) != 1 || got.Calls[0].Name != "simulate_scenario" || len(got.Output) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	if err := c.http.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Fatal("unsafe redirects")
	}
}

func TestPlannerRejectsBadResponses(t *testing.T) {
	for _, body := range []string{`{`, `{"status":"incomplete"}`, `{"status":"completed","output":[]}`} {
		if _, err := parseTurn([]byte(body)); err == nil {
			t.Fatal("bad response accepted")
		}
	}
	_, err := parseTurn([]byte(`{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"refusal"}]}]}`))
	if !errors.Is(err, errRefused) {
		t.Fatal(err)
	}
}
