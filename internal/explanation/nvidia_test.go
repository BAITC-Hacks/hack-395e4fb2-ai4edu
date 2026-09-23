package explanation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNVIDIAProtocol(t *testing.T) {
	t.Setenv("NVIDIA_API_KEY", "nvidia-secret")
	t.Setenv("NVIDIA_MODEL", "test-model")
	provider, err := NewNVIDIAFromEnv(true)
	if err != nil {
		t.Fatal(err)
	}
	c := provider.(*nvidiaClient)
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://integrate.api.nvidia.com/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer nvidia-secret" {
			t.Fatal("wrong endpoint/auth")
		}
		var body struct {
			Model    string                           `json:"model"`
			Stream   bool                             `json:"stream"`
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Model != "test-model" || body.Stream || len(body.Messages) != 2 {
			t.Fatal("bad request")
		}
		if body.Messages[0].Content != reviewInstructions || body.Messages[1].Content != `{"result":{}}` {
			t.Fatal("wrong reviewer input")
		}
		return response(200, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":" Риски остаются. "}}]}`), nil
	})
	text, err := c.Generate(context.Background(), json.RawMessage(`{"result":{}}`))
	if err != nil || text != "Риски остаются." {
		t.Fatalf("%s %v", text, err)
	}
	if c.http.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("unsafe redirects")
	}
}

func TestNVIDIAErrorsAndConfiguration(t *testing.T) {
	t.Setenv("NVIDIA_API_KEY", "")
	t.Setenv("NVIDIA_MODEL", "")
	if c, err := NewNVIDIAFromEnv(false); c != nil || err != nil {
		t.Fatal("absent config must disable NVIDIA")
	}
	t.Setenv("NVIDIA_MODEL", "model")
	if _, err := NewNVIDIAFromEnv(false); err == nil {
		t.Fatal("partial config accepted")
	}
	for _, body := range []string{`{`, `{"choices":[]}`, `{"choices":[{"finish_reason":"length","message":{"role":"assistant","content":"partial"}}]}`, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":""}}]}`} {
		c := &nvidiaClient{http: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return response(200, body), nil })}}
		if text, err := c.Generate(context.Background(), json.RawMessage(`{}`)); text != "" || err == nil {
			t.Fatal("bad response accepted")
		}
	}
	c := &nvidiaClient{http: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return response(401, "nvidia-secret"), nil })}}
	if _, err := c.Generate(context.Background(), json.RawMessage(`{}`)); err == nil || strings.Contains(err.Error(), "nvidia-secret") {
		t.Fatal("unsafe HTTP error")
	}
}

func TestProviderBackup(t *testing.T) {
	for _, mode := range []string{"success", "failure", "blank", "refusal", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			first := clientFunc(func(ctx context.Context, data json.RawMessage) (string, error) {
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 4*time.Second {
					t.Error("missing primary timeout")
				}
				data[0] = 'X'
				switch mode {
				case "success":
					return "primary", nil
				case "blank":
					return " ", nil
				case "refusal":
					return "", ErrRefused
				default:
					return "", errors.New("failed")
				}
			})
			second := clientFunc(func(_ context.Context, data json.RawMessage) (string, error) {
				calls++
				if string(data) != `{}` {
					t.Error("input mutated")
				}
				return "backup", nil
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			_, _ = (backupClient{first, second}).Generate(ctx, json.RawMessage(`{}`))
			want := 0
			if mode == "failure" || mode == "blank" {
				want = 1
			}
			if calls != want {
				t.Fatalf("backup calls %d want %d", calls, want)
			}
		})
	}
}
