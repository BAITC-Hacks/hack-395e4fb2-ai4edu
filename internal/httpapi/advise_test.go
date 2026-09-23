package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hack-395e4fb2-ai4edu/internal/advisor"
)

type plannerFunc func(context.Context, []json.RawMessage, bool, bool) (advisor.Turn, error)

func (f plannerFunc) Next(c context.Context, h []json.RawMessage, a, r bool) (advisor.Turn, error) {
	return f(c, h, a, r)
}

func TestAdviseOfflineAndValidation(t *testing.T) {
	h := NewHandler()
	want := call(h, "POST", "/api/simulate", goldenJSON)
	got := call(h, "POST", "/api/advise", goldenJSON)
	var body struct {
		Result   json.RawMessage `json:"result"`
		Status   string          `json:"status"`
		Improved bool            `json:"improved"`
	}
	if got.Code != 200 || json.Unmarshal(got.Body.Bytes(), &body) != nil || !bytes.Equal(append(body.Result, '\n'), want.Body.Bytes()) || body.Status != "disabled" || body.Improved {
		t.Fatalf("%d %s", got.Code, got.Body.String())
	}
	h = NewHandlerWithOptions(Options{Advisor: advisor.New(plannerFunc(func(context.Context, []json.RawMessage, bool, bool) (advisor.Turn, error) {
		t.Fatal("invalid scenario reached AI")
		return advisor.Turn{}, nil
	}), nil)})
	for _, input := range []string{`{"decisions":[]}`, goldenJSON + `{}`, `{"decisions":`, strings.Repeat(" ", maxRequestBytes) + goldenJSON} {
		want := call(h, "POST", "/api/simulate", input)
		got := call(h, "POST", "/api/advise", input)
		if got.Code != want.Code || !bytes.Equal(got.Body.Bytes(), want.Body.Bytes()) {
			t.Fatal("validation contracts differ")
		}
	}
}

func TestAdvisorConcurrencyLimit(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	done := make(chan struct{}, 2)
	defer func() {
		close(release)
		for i := 0; i < 2; i++ {
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Error("request failed to stop")
			}
		}
	}()
	planner := plannerFunc(func(ctx context.Context, _ []json.RawMessage, _, _ bool) (advisor.Turn, error) {
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return advisor.Turn{}, context.Canceled
	})
	h := NewHandlerWithOptions(Options{Advisor: advisor.New(planner, nil)})
	for i := 0; i < 2; i++ {
		go func() { call(h, "POST", "/api/advise", goldenJSON); done <- struct{}{} }()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("request not started")
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/api/advise", strings.NewReader(goldenJSON))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
