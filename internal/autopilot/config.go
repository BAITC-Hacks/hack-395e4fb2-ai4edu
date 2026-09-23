package autopilot

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

func NewFromEnv() (*Service, error) {
	agents, err := NewAgentsFromEnv()
	if err != nil {
		return nil, err
	}
	if agents == nil {
		return nil, nil
	}
	runBudget, err := envNumber("AUTOPILOT_RUN_BUDGET_USD", .10, .001, 5)
	if err != nil {
		return nil, err
	}
	totalBudget, err := envNumber("AUTOPILOT_TOTAL_BUDGET_USD", 50, .001, 10000)
	if err != nil {
		return nil, err
	}
	iterations, err := envNumber("AUTOPILOT_MAX_ITERATIONS", 3, 1, 5)
	if err != nil || math.Trunc(iterations) != iterations {
		return nil, fmt.Errorf("AUTOPILOT_MAX_ITERATIONS must be an integer between 1 and 5")
	}
	path := strings.TrimSpace(os.Getenv("AUTOPILOT_BUDGET_FILE"))
	if path == "" {
		path = ".autopilot/usage.json"
	}
	budget, err := NewBudget(totalBudget, path)
	if err != nil {
		return nil, err
	}
	return New(agents, budget, Config{RunBudgetUSD: runBudget, MaxIterations: int(iterations)}), nil
}

func envNumber(name string, fallback, min, max float64) (float64, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return v, nil
}
