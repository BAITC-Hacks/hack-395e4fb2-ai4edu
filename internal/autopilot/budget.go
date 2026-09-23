package autopilot

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
)

var ErrBudget = errors.New("autopilot budget exhausted")

// Budget reserves before sending a paid request. Persisted reservations remain
// charged after a crash: an interrupted HTTP request may still have been billed.
// One server process owns this ledger. It does not track other API clients.
type Budget struct {
	mu           sync.Mutex
	limit, spent float64
	path         string
}

func NewBudget(limit float64, path string) (*Budget, error) {
	if math.IsNaN(limit) || math.IsInf(limit, 0) || limit <= 0 {
		return nil, errors.New("invalid autopilot total budget")
	}
	b := &Budget{limit: limit, path: path}
	if path == "" {
		return b, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return b, nil
	}
	if err != nil {
		return nil, errors.New("cannot read autopilot budget ledger")
	}
	var state struct {
		Version int     `json:"version"`
		Spent   float64 `json:"spent_usd"`
	}
	if json.Unmarshal(data, &state) != nil || state.Version != 1 || state.Spent < 0 || math.IsNaN(state.Spent) || math.IsInf(state.Spent, 0) {
		return nil, errors.New("invalid autopilot budget ledger")
	}
	b.spent = state.Spent
	return b, nil
}

func (b *Budget) reserve(amount float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return errors.New("invalid API cost reservation")
	}
	if b.spent+amount > b.limit {
		return ErrBudget
	}
	previous := b.spent
	b.spent += amount
	if err := b.persist(); err != nil {
		b.spent = previous
		return err
	}
	return nil
}

func (b *Budget) settle(reserved, actual float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if actual < 0 || math.IsNaN(actual) || math.IsInf(actual, 0) {
		return errors.New("invalid API usage cost")
	}
	previous := b.spent
	b.spent = math.Max(0, b.spent-reserved+actual)
	if err := b.persist(); err != nil {
		b.spent = previous
		return err
	}
	return nil
}

func (b *Budget) persist() error {
	if b.path == "" {
		return nil
	}
	if os.MkdirAll(filepath.Dir(b.path), 0700) != nil {
		return errors.New("cannot create autopilot budget directory")
	}
	data, _ := json.Marshal(struct {
		Version int     `json:"version"`
		Spent   float64 `json:"spent_usd"`
	}{1, b.spent})
	f, err := os.CreateTemp(filepath.Dir(b.path), "usage-*.tmp")
	if err != nil {
		return errors.New("cannot save autopilot budget")
	}
	name := f.Name()
	defer os.Remove(name)
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return errors.New("cannot save autopilot budget")
	}
	if os.Rename(name, b.path) != nil {
		return errors.New("cannot replace autopilot budget ledger")
	}
	return nil
}
