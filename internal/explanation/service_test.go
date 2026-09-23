package explanation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"hack-395e4fb2-ai4edu/internal/simulation"
)

type clientFunc func(context.Context, json.RawMessage) (string, error)

func (f clientFunc) Generate(ctx context.Context, data json.RawMessage) (string, error) {
	return f(ctx, data)
}

func simulateJSON(t *testing.T, data string) simulation.Result {
	t.Helper()
	var request simulation.Request
	if err := json.Unmarshal([]byte(data), &request); err != nil {
		t.Fatal(err)
	}
	result := simulation.Simulate(request.Decisions)
	if !result.Valid {
		t.Fatal(result.ValidationErrors)
	}
	return result
}

func goldenResult(t *testing.T) simulation.Result {
	return simulateJSON(t, `{"decisions":[{"measure_id":"M7","district_id":"nura"},{"measure_id":"M8","district_id":"nura"},{"measure_id":"M10","district_id":"nura"},{"measure_id":"M12"},{"measure_id":"M5","district_id":"saryarka"}]}`)
}

func TestFallbackExplainsTradeoffs(t *testing.T) {
	result := goldenResult(t)
	got := New(nil).Explain(context.Background(), result)
	if got.Source != "fallback" {
		t.Fatal(got.Source)
	}
	for _, text := range []string{
		"Сильные стороны", "Нура — Безопасность улиц: +12.5", "Нура — Школы и детсады: +10",
		"Освещение и камеры (расширение Safe City)", "Единая цифровая платформа обращений", "Безопасность улиц +2",
		"Риски", "Самый слабый район по итоговому районному Score — Нура (52.9625)", "Критических значений не осталось",
		"Компромиссы", "Неиспользованный бюджет: 5", "Экология, Соцсфера, Безопасность, Сервисы",
		"Нет выбранных мер направлений: Транспорт",
	} {
		if !strings.Contains(got.Text, text) {
			t.Errorf("fallback missing %q: %s", text, got.Text)
		}
	}
	if repeated := New(nil).Explain(context.Background(), result); repeated != got {
		t.Fatal("fallback is not deterministic")
	}
}

func TestFallbackCriticalValuesAndLosses(t *testing.T) {
	r := simulateJSON(t, `{"decisions":[{"measure_id":"M9","district_id":"esil"},{"measure_id":"M11","district_id":"almaty"},{"measure_id":"M4","district_id":"esil"},{"measure_id":"M6"},{"measure_id":"M14"}]}`)
	got := New(nil).Explain(context.Background(), r)
	for _, text := range []string{"Алматы — Разгрузка дорог: 38.25", "Нура — Школы и детсады: 38", "Нура — Поликлиники и первичная медпомощь: 35", "Ухудшения показателей: Алматы — Разгрузка дорог: -1.75", "Синергии не активированы"} {
		if !strings.Contains(got.Text, text) {
			t.Errorf("fallback missing %q", text)
		}
	}
}

func TestServiceDeadlineAndExpiredResponse(t *testing.T) {
	result := goldenResult(t)
	client := clientFunc(func(ctx context.Context, _ json.RawMessage) (string, error) {
		deadline, ok := ctx.Deadline()
		remaining := time.Until(deadline)
		if !ok || remaining <= 0 || remaining > 10*time.Second {
			t.Fatalf("wrong deadline: %v", remaining)
		}
		return "Объяснение", nil
	})
	if got := New(client).Explain(context.Background(), result); got.Source != "llm" {
		t.Fatal(got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	lateClient := clientFunc(func(context.Context, json.RawMessage) (string, error) {
		cancel()
		return "late response", nil
	})
	defer cancel()
	if got := New(lateClient).Explain(ctx, result); got.Source != "fallback" {
		t.Fatal("expired response accepted")
	}
}

func TestServiceDoesNotExplainInvalidResult(t *testing.T) {
	client := clientFunc(func(context.Context, json.RawMessage) (string, error) {
		t.Fatal("invalid input reached provider")
		return "", nil
	})
	got := New(client).Explain(context.Background(), simulation.Simulate(nil))
	if got.Source != "fallback" || got.Text == "" {
		t.Fatal(got)
	}
}
