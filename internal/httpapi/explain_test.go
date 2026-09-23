package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hack-395e4fb2-ai4edu/internal/explanation"
)

type fakeClient func(context.Context, json.RawMessage) (string, error)

func (f fakeClient) Generate(ctx context.Context, input json.RawMessage) (string, error) {
	return f(ctx, input)
}

func TestExplainPreservesResult(t *testing.T) {
	for _, mode := range []string{"success", "provider error", "timeout", "empty", "no key"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			var client explanation.Client = fakeClient(func(ctx context.Context, input json.RawMessage) (string, error) {
				calls++
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > explanation.Timeout {
					t.Error("LLM must have a bounded deadline")
				}
				var payload map[string]json.RawMessage
				if err := json.Unmarshal(input, &payload); err != nil {
					t.Fatal(err)
				}
				for _, key := range []string{"result", "selected_measures", "indicator_names", "category_names"} {
					if _, ok := payload[key]; !ok {
						t.Errorf("missing AI input %s", key)
					}
				}
				if !bytes.Contains(input, []byte("Школа + детсад (модульное строительство)")) || !bytes.Contains(input, []byte("Поликлиники и первичная медпомощь")) || !bytes.Contains(input, []byte("final_breakdown")) || !bytes.Contains(input, []byte("applied_synergies")) {
					t.Error("AI input must include names and result details")
				}
				// The provider only owns JSON bytes and cannot mutate engine state.
				for i := range input {
					input[i] = ' '
				}
				switch mode {
				case "provider error":
					return "", errors.New("provider failed")
				case "timeout":
					<-ctx.Done()
					return "", ctx.Err()
				case "empty":
					return " \n\t", nil
				default:
					return "Сильные стороны: улучшена социальная инфраструктура.", nil
				}
			})
			if mode == "no key" {
				t.Setenv("OPENAI_API_KEY", "")
				client = explanation.NewOpenAIFromEnv()
				if client != nil {
					t.Fatal("missing key must disable client")
				}
			}
			handler := NewHandlerWithOptions(Options{Explainer: explanation.New(client)})
			want := call(handler, "POST", "/api/simulate", goldenJSON)
			r := httptest.NewRequest("POST", "/api/explain", strings.NewReader(goldenJSON))
			if mode == "timeout" {
				// A shorter parent deadline exercises real cancellation without a 10s test.
				ctx, cancel := context.WithTimeout(r.Context(), 20*time.Millisecond)
				defer cancel()
				r = r.WithContext(ctx)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			var response struct {
				Result      json.RawMessage   `json:"result"`
				Explanation map[string]string `json:"explanation"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(append(response.Result, '\n'), want.Body.Bytes()) {
				t.Fatal("explain result differs byte-for-byte from simulate")
			}
			expectedSource := "fallback"
			if mode == "success" {
				expectedSource = "llm"
			}
			if len(response.Explanation) != 2 || response.Explanation["source"] != expectedSource || response.Explanation["text"] == "" {
				t.Fatalf("unexpected explanation: %+v", response.Explanation)
			}
			if mode != "no key" && calls != 1 {
				t.Errorf("provider calls = %d, want 1", calls)
			}
		})
	}
}

func TestExplainInvalidNeverCallsLLM(t *testing.T) {
	handler := NewHandlerWithOptions(Options{Explainer: explanation.New(fakeClient(func(context.Context, json.RawMessage) (string, error) {
		t.Fatal("invalid scenario called LLM")
		return "", nil
	}))})
	for _, body := range []string{`{"decisions":[]}`, strings.Replace(goldenJSON, `"M7"`, `"M8"`, 1), `{"decisions":`, goldenJSON + `{}`, strings.Repeat(" ", maxRequestBytes) + goldenJSON} {
		want := call(handler, "POST", "/api/simulate", body)
		got := call(handler, "POST", "/api/explain", body)
		if got.Code != want.Code || !bytes.Equal(got.Body.Bytes(), want.Body.Bytes()) || got.Code == http.StatusOK {
			t.Fatalf("invalid explain differs from simulate: %d %s", got.Code, got.Body.String())
		}
	}
}

func TestExplainDefaultFallbackAndConcurrency(t *testing.T) {
	handler := NewHandler()
	want := call(handler, "POST", "/api/explain", goldenJSON)
	if want.Code != http.StatusOK || !strings.Contains(want.Body.String(), `"source":"fallback"`) {
		t.Fatal(want.Body.String())
	}
	for i := 0; i < 10; i++ {
		t.Run("parallel", func(t *testing.T) {
			t.Parallel()
			got := call(handler, "POST", "/api/explain", goldenJSON)
			if got.Code != want.Code || got.Body.String() != want.Body.String() {
				t.Fatal("parallel explanation changed result")
			}
		})
	}
}
