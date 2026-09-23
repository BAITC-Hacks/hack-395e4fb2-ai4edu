package autopilot

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestBudgetPersistsReservationsAndActualUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "usage.json")
	b, err := NewBudget(.05, path)
	if err != nil {
		t.Fatal(err)
	}
	if err = b.reserve(.03); err != nil {
		t.Fatal(err)
	}
	afterCrash, err := NewBudget(.05, path)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(afterCrash.reserve(.03), ErrBudget) {
		t.Fatal("restart forgot in-flight paid request")
	}
	if err = b.settle(.03, .01); err != nil {
		t.Fatal(err)
	}
	afterSettlement, err := NewBudget(.05, path)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(afterSettlement.spent-.01) > 1e-9 {
		t.Fatal("wrong settlement")
	}
	if err = afterSettlement.reserve(.03); err != nil {
		t.Fatal(err)
	}
}

func TestBudgetConcurrentReservationsCannotOverspend(t *testing.T) {
	b, _ := NewBudget(.05, "")
	var wg sync.WaitGroup
	var accepted atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.reserve(.02) == nil {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 2 {
		t.Fatalf("accepted %d", accepted.Load())
	}
}

func TestBudgetFailsClosedOnCorruptOrUnwritableLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger")
	if err := os.WriteFile(path, []byte(`{"version":1,"spent_usd":-1}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewBudget(50, path); err == nil {
		t.Fatal("corrupt ledger accepted")
	}
	b, _ := NewBudget(50, "")
	b.path = filepath.Join(path, "child")
	if err := b.reserve(.02); err == nil || b.spent != 0 {
		t.Fatal("paid request allowed without durable reservation")
	}
}
