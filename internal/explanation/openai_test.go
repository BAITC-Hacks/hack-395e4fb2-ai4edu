package explanation

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

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestOpenAIRequestAndOutputParsing(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-secret")
	t.Setenv("OPENAI_MODEL", "test-model")
	client := NewOpenAIFromEnv().(*openAIClient)
	client.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://api.openai.com/v1/responses" || r.Method != http.MethodPost {
			t.Fatal("wrong endpoint")
		}
		if r.Header.Get("Authorization") != "Bearer test-secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatal("missing authentication or content type")
		}
		var body struct {
			Model           string `json:"model"`
			Instructions    string `json:"instructions"`
			Input           string `json:"input"`
			Store           *bool  `json:"store"`
			MaxOutputTokens int    `json:"max_output_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "test-model" || body.Input != `{"result":{}}` || body.Store == nil || *body.Store || body.MaxOutputTokens != 1200 {
			t.Fatalf("wrong request: %+v", body)
		}
		for _, clause := range []string{"Не рассчитывай и не пересчитывай Score", "Не придумывай и не называй числа", "Сильные стороны", "Риски", "Компромиссы", "без HTML"} {
			if !strings.Contains(body.Instructions, clause) {
				t.Errorf("missing prompt constraint %q", clause)
			}
		}
		return response(200, `{"status":"completed","output":[{"type":"reasoning"},{"type":"message","role":"assistant","content":[{"type":"output_text","text":" Первое. "},{"type":"output_text","text":"Второе."}]}]}`), nil
	})
	text, err := client.Generate(context.Background(), json.RawMessage(`{"result":{}}`))
	if err != nil || text != "Первое.\n\nВторое." {
		t.Fatalf("text=%q error=%v", text, err)
	}
	if client.http.Timeout != Timeout {
		t.Fatal("provider timeout is not bounded")
	}
	if err := client.http.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Fatal("redirects must not forward credentials")
	}
}

func TestOpenAIFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"unauthorized", 401, `test-secret`},
		{"rate limit", 429, `test-secret`},
		{"server error", 500, `test-secret`},
		{"redirect", 302, ""},
		{"malformed", 200, "not json"},
		{"empty output", 200, `{"status":"completed","output":[]}`},
		{"blank text", 200, `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"  "}]}]}`},
		{"truncated", 200, `{"status":"incomplete","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"unfinished"}]}]}`},
		{"refusal", 200, `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"refusal","refusal":"no"}]}]}`},
		{"oversize", 200, strings.Repeat(" ", maxResponseBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &openAIClient{key: "test-secret", model: defaultModel, http: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				return response(tc.status, tc.body), nil
			})}}
			text, err := client.Generate(context.Background(), json.RawMessage(`{}`))
			if err == nil || text != "" {
				t.Fatalf("expected failure, text=%q err=%v", text, err)
			}
			if strings.Contains(err.Error(), "test-secret") {
				t.Fatal("credential leaked in error")
			}
			if got := New(client).Explain(context.Background(), goldenResult(t)); got.Source != "fallback" {
				t.Fatal("provider failure did not trigger fallback")
			}
		})
	}
}

func TestOpenAITransportCancellation(t *testing.T) {
	client := &openAIClient{key: "test-secret", model: defaultModel, http: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, errors.New("transport error includes test-secret")
	})}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Generate(ctx, json.RawMessage(`{}`))
	if err == nil || strings.Contains(err.Error(), "test-secret") {
		t.Fatal("unsafe transport error")
	}
}

func TestOpenAIEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "  ")
	if NewOpenAIFromEnv() != nil {
		t.Fatal("blank key must disable provider")
	}
	t.Setenv("OPENAI_API_KEY", "test-secret")
	t.Setenv("OPENAI_MODEL", "")
	client := NewOpenAIFromEnv().(*openAIClient)
	if client.model != defaultModel {
		t.Fatal("missing default model")
	}
}
