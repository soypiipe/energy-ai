package analysis

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/soypiipe/energy-ai/backend/internal/pgtest"
)

func newRepo(t *testing.T) *Repository {
	t.Helper()
	return NewRepository(pgtest.New(t))
}

func TestRunLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)

	run, err := repo.Enqueue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != StatusPending || run.ID == "" || run.StartedAt != nil {
		t.Fatalf("recién encolada: %+v", run)
	}

	claimed, err := repo.Claim(ctx)
	if err != nil || claimed == nil || claimed.ID != run.ID || claimed.Status != StatusRunning || claimed.StartedAt == nil {
		t.Fatalf("Claim = %+v, %v", claimed, err)
	}

	if err := repo.SetStep(ctx, run.ID, "baseline"); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(ctx, run.ID)
	if got.CurrentStep == nil || *got.CurrentStep != "baseline" {
		t.Errorf("current_step = %v", got.CurrentStep)
	}

	if err := repo.Complete(ctx, run.ID, map[string]int{"anomalies": 4}); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.Get(ctx, run.ID)
	if got.Status != StatusCompleted || got.FinishedAt == nil || got.CurrentStep != nil || string(got.Summary) != `{"anomalies": 4}` {
		t.Errorf("completada: %+v summary=%s", got, got.Summary)
	}

	// una ejecución completada no se puede volver a completar ni fallar
	if err := repo.Complete(ctx, run.ID, nil); err == nil {
		t.Error("completar dos veces debe fallar")
	}
	if err := repo.Fail(ctx, run.ID, "x"); err == nil {
		t.Error("fallar una ejecución completada debe fallar")
	}
}

func TestFail(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)
	run, _ := repo.Enqueue(ctx)
	if _, err := repo.Claim(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repo.Fail(ctx, run.ID, "boom"); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(ctx, run.ID)
	if got.Status != StatusFailed || got.Error == nil || *got.Error != "boom" || got.FinishedAt == nil {
		t.Errorf("fallida: %+v", got)
	}
}

func TestClaimOrderAndEmpty(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)
	if c, err := repo.Claim(ctx); err != nil || c != nil {
		t.Fatalf("cola vacía: %+v, %v", c, err)
	}
	first, _ := repo.Enqueue(ctx)
	second, _ := repo.Enqueue(ctx)
	a, _ := repo.Claim(ctx)
	b, _ := repo.Claim(ctx)
	if a.ID != first.ID || b.ID != second.ID {
		t.Errorf("orden FIFO roto: %s,%s vs %s,%s", a.ID, b.ID, first.ID, second.ID)
	}
	if c, _ := repo.Claim(ctx); c != nil {
		t.Errorf("no debía quedar nada: %+v", c)
	}
}

func TestClaimConcurrentNoDuplicates(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)
	const n = 10
	for i := 0; i < n; i++ {
		if _, err := repo.Enqueue(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	seen := map[string]int{}
	var wg sync.WaitGroup
	for w := 0; w < 5; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				run, err := repo.Claim(ctx)
				if err != nil {
					t.Error(err)
					return
				}
				if run == nil {
					return
				}
				mu.Lock()
				seen[run.ID]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(seen) != n {
		t.Errorf("se tomaron %d ejecuciones distintas, quería %d", len(seen), n)
	}
	for id, c := range seen {
		if c != 1 {
			t.Errorf("%s se tomó %d veces", id, c)
		}
	}
}

func TestGetNotFound(t *testing.T) {
	repo := newRepo(t)
	if _, err := repo.Get(context.Background(), "00000000-0000-0000-0000-000000000000"); err != ErrRunNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestRequeueStuck(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)
	run, _ := repo.Enqueue(ctx)
	repo.Claim(ctx)

	if n, err := repo.RequeueStuck(ctx, time.Hour); err != nil || n != 0 {
		t.Fatalf("reciente: n=%d err=%v", n, err)
	}
	time.Sleep(20 * time.Millisecond)
	if n, err := repo.RequeueStuck(ctx, 10*time.Millisecond); err != nil || n != 1 {
		t.Fatalf("atascada: n=%d err=%v", n, err)
	}
	again, _ := repo.Claim(ctx)
	if again == nil || again.ID != run.ID {
		t.Errorf("debía poder tomarse de nuevo: %+v", again)
	}
}
