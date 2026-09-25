package analysis

import (
	"context"
	"errors"
	"testing"
	"time"
)

// runWorker arranca el worker y devuelve una función que lo detiene y espera a que termine.
func runWorker(t *testing.T, w *Worker) (stop func()) {
	t.Helper()
	w.poll = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("el worker no se detuvo")
		}
	}
}

func waitStatus(t *testing.T, repo *Repository, id, want string) Run {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		run, err := repo.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status == want {
			return run
		}
		if time.Now().After(deadline) {
			t.Fatalf("estado = %s, esperaba %s", run.Status, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWorkerCompletesRun(t *testing.T) {
	repo := newRepo(t)
	w := NewWorker(repo, func(_ context.Context, _ Run, step func(string) error) (any, error) {
		if err := step("uno"); err != nil {
			return nil, err
		}
		return map[string]int{"anomalies": 4}, nil
	})
	stop := runWorker(t, w)
	defer stop()

	run, _ := repo.Enqueue(context.Background())
	got := waitStatus(t, repo, run.ID, StatusCompleted)
	if string(got.Summary) != `{"anomalies": 4}` {
		t.Errorf("summary = %s", got.Summary)
	}
}

func TestWorkerMarksFailure(t *testing.T) {
	repo := newRepo(t)
	w := NewWorker(repo, func(context.Context, Run, func(string) error) (any, error) {
		return nil, errors.New("boom")
	})
	stop := runWorker(t, w)
	defer stop()

	run, _ := repo.Enqueue(context.Background())
	got := waitStatus(t, repo, run.ID, StatusFailed)
	if got.Error == nil || *got.Error != "boom" {
		t.Errorf("error = %v", got.Error)
	}
}

func TestWorkerSurvivesPanic(t *testing.T) {
	repo := newRepo(t)
	calls := 0
	w := NewWorker(repo, func(context.Context, Run, func(string) error) (any, error) {
		calls++
		if calls == 1 {
			panic("kaboom")
		}
		return "ok", nil
	})
	stop := runWorker(t, w)
	defer stop()

	first, _ := repo.Enqueue(context.Background())
	waitStatus(t, repo, first.ID, StatusFailed)
	second, _ := repo.Enqueue(context.Background())
	waitStatus(t, repo, second.ID, StatusCompleted) // el worker sigue vivo tras el panic
}

func TestWorkerGracefulShutdownRequeues(t *testing.T) {
	repo := newRepo(t)
	started := make(chan struct{})
	w := NewWorker(repo, func(ctx context.Context, _ Run, _ func(string) error) (any, error) {
		close(started)
		<-ctx.Done() // trabajo largo que respeta la cancelación
		return nil, ctx.Err()
	})
	stop := runWorker(t, w)

	run, _ := repo.Enqueue(context.Background())
	<-started
	stop() // apagado con la ejecución en curso

	got, _ := repo.Get(context.Background(), run.ID)
	if got.Status != StatusPending {
		t.Errorf("estado tras el apagado = %s, quería PENDING (no FAILED)", got.Status)
	}
}

func TestWorkerRecoversStuckRunOnStart(t *testing.T) {
	repo := newRepo(t)
	run, _ := repo.Enqueue(context.Background())
	repo.Claim(context.Background()) // queda RUNNING sin worker
	if _, err := repo.pool.Exec(context.Background(),
		`UPDATE analysis_runs SET started_at = now() - interval '1 hour' WHERE id = $1`, run.ID); err != nil {
		t.Fatal(err)
	}

	w := NewWorker(repo, func(context.Context, Run, func(string) error) (any, error) { return "ok", nil })
	stop := runWorker(t, w)
	defer stop()
	waitStatus(t, repo, run.ID, StatusCompleted)
}
