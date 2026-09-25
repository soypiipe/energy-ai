package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ProcessFunc hace el trabajo de una ejecución: reporta su avance con setStep y devuelve el resumen
// que se guarda en analysis_runs.summary. Es una función (no una interfaz) porque solo hay una
// implementación real; en los tests se reemplaza por una falsa.
type ProcessFunc func(ctx context.Context, run Run, setStep func(step string) error) (summary any, err error)

const (
	defaultPollInterval = time.Second
	// stuckAfter: una ejecución RUNNING más vieja que esto se considera abandonada al arrancar.
	// Un análisis de 12 medidores tarda milisegundos, así que 5 minutos es un margen enorme.
	stuckAfter = 5 * time.Minute
	// finishTimeout: tiempo para escribir el estado final aunque el contexto ya esté cancelado.
	finishTimeout = 5 * time.Second
)

// Worker consume la cola analysis_runs: toma una ejecución, la procesa y guarda el resultado.
type Worker struct {
	repo    *Repository
	process ProcessFunc
	poll    time.Duration
}

func NewWorker(repo *Repository, process ProcessFunc) *Worker {
	return &Worker{repo: repo, process: process, poll: defaultPollInterval}
}

// Run procesa ejecuciones hasta que ctx se cancele. Es el apagado ordenado: si llega la señal
// mientras hay una ejecución en curso, se cancela su contexto y la ejecución vuelve a PENDING para
// que la retome el próximo arranque, en vez de quedar como fallida por un simple reinicio.
func (w *Worker) Run(ctx context.Context) {
	if n, err := w.repo.RequeueStuck(ctx, stuckAfter); err != nil {
		slog.Error("worker: recuperar ejecuciones atascadas", "err", err)
	} else if n > 0 {
		slog.Warn("worker: ejecuciones atascadas devueltas a la cola", "count", n)
	}

	slog.Info("worker de análisis iniciado", "poll", w.poll)
	for {
		run, err := w.repo.Claim(ctx)
		switch {
		case ctx.Err() != nil:
			slog.Info("worker de análisis detenido")
			return
		case err != nil:
			slog.Error("worker: tomar ejecución", "err", err)
		case run != nil:
			w.handle(ctx, *run)
			continue // puede haber más trabajo: no esperar
		}

		select {
		case <-ctx.Done():
			slog.Info("worker de análisis detenido")
			return
		case <-time.After(w.poll):
		}
	}
}

// handle procesa una ejecución y deja su estado final. Nunca propaga panics ni errores: el worker sigue vivo.
func (w *Worker) handle(ctx context.Context, run Run) {
	log := slog.With("run_id", run.ID)
	log.Info("análisis iniciado")

	summary, err := w.safeProcess(ctx, run)

	// El estado final se escribe con un contexto propio: ctx puede estar cancelado por el apagado.
	fin, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()

	switch {
	case err != nil && ctx.Err() != nil:
		log.Warn("análisis interrumpido por el apagado; vuelve a la cola", "err", err)
		if e := w.repo.Requeue(fin, run.ID); e != nil {
			log.Error("devolver a la cola", "err", e)
		}
	case err != nil:
		log.Error("análisis falló", "err", err)
		if e := w.repo.Fail(fin, run.ID, err.Error()); e != nil {
			log.Error("marcar como fallido", "err", e)
		}
	default:
		if e := w.repo.Complete(fin, run.ID, summary); e != nil {
			log.Error("marcar como completado", "err", e)
			return
		}
		log.Info("análisis completado")
	}
}

func (w *Worker) safeProcess(ctx context.Context, run Run) (summary any, err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("panic durante el análisis: %v", v)
		}
	}()
	return w.process(ctx, run, func(step string) error { return w.repo.SetStep(ctx, run.ID, step) })
}
