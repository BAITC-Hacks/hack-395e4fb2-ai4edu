package autopilot

import (
	"strings"
	"testing"
)

func TestExplicitBudgetLimitSupportedPhrases(t *testing.T) {
	for _, tc := range []struct {
		text string
		want int
	}{
		{"Максимизируй Score при бюджете 100, устрани все критические показатели.", 100},
		{"Максимизируй Score при бюджете не больше 95", 95},
		{"бюджет: 90", 90},
		{"бюджет до 80", 80},
		{"Budget at most 95", 95},
		{"budget 100", 100},
		{"budget <= 95", 95},
		{"БЮДЖЕТ НЕ БОЛЕЕ 90", 90},
		{"С бюджетом не выше 85 помоги Нуре", 85},
		{"budget no more than 80", 80},
		{"budget not more than 75", 75},
		{"budget up to 70", 70},
		{"budget of 65", 65},
		{"budget ≤ 60", 60},
		{"бюджет = 55", 55},
		{"Не превышай бюджет 95", 95},
		{"бюджет100", 100},
		{"бюджете95", 95},
		{"бюджет:\t90", 90},
		{"бюджет\u00a0не\u202fбольше\u00a095", 95},
		{"5 районов, 8 кварталов; бюджет 95; критических показателей 0", 95},
		{"budget 1.", 1},
		{"budget 95!", 95},
		{"budget 95, budget 95", 95},
		{"бюджет 95; budget <= 95", 95},
	} {
		t.Run(tc.text, func(t *testing.T) {
			got, err := explicitBudgetLimit(tc.text)
			if err != nil || got == nil || *got != tc.want {
				t.Fatalf("got %v err=%v, want %d", got, err, tc.want)
			}
		})
	}
}

func TestExplicitBudgetLimitDoesNotInventCaps(t *testing.T) {
	for _, text := range []string{
		"", "Помоги 5 районам за 8 кварталов", "Score 95, устрани 2 критических показателя",
		"Бюджет не ограничен, 5 мероприятий", "бюджет не меньше 95", "бюджет не ниже 95",
		"budget at least 95", "budget more than 95", "budget not 95", "бюджет не 95",
		"budget is unspecified, district score 95", "budget approximately 90", "бюджет около 90",
		"бюджет от 80 до 100", "Предложи 100 решений без бюджета", "внебюджет 90",
		"безбюджетный план 90", "mybudget 95", "budget_limit 95", "budgets 95",
		"бюджетный показатель 95", "API бюджет: $50, Score 95", "бюджет девяносто",
		"плану95 нужен хороший бюджет", "budget? 95 districts",
	} {
		t.Run(text, func(t *testing.T) {
			got, err := explicitBudgetLimit(text)
			if got != nil || err != nil {
				t.Fatalf("unsupported phrase produced a cap/error: %v %v", got, err)
			}
		})
	}
}

func TestExplicitBudgetLimitAmbiguityRequiresClarification(t *testing.T) {
	for _, text := range []string{
		"бюджет 95, бюджет 90", "budget95,budget90", "бюджет 95; budget at most 100",
		"budget 0", "budget 101", "бюджет -1", "бюджет +95", "budget 9999999999999999999999999",
		"бюджет 95.5", "бюджет 95,5", "budget 95.0", "budget 9e1", "budget 9E+1",
		"budget 90-100", "бюджет 90–100", "бюджет 90 — 100", "budget 90/100",
		"бюджет 90 или 100", "budget 90 or 100", "budget 90 to 100", "бюджет 90 до 100",
		"бюджет 1 000", "budget 1\u00a0000", "бюджет 90 тыс.", "budget 90 thousand", "budget 90k",
		"budget 95abc", "бюджет 95районов", "budget 95_000", "бюджет 95,000",
		"не бюджет 95", "not budget 95", "no budget 95", "не бюджет 95, а бюджет 90", strings.Repeat("x", 16<<10+1), string([]byte{0xff}),
	} {
		t.Run(text, func(t *testing.T) {
			got, err := explicitBudgetLimit(text)
			if got != nil || err == nil {
				t.Fatalf("ambiguous/invalid amount accepted: %v %v", got, err)
			}
		})
	}
}
