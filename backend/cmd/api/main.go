// Punto de entrada de la API. Aquí se hace el "wiring" de dependencias a mano:
// es el equivalente explícito de los módulos/providers de NestJS.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/soypiipe/energy-ai/backend/internal/analysis"
	"github.com/soypiipe/energy-ai/backend/internal/config"
	"github.com/soypiipe/energy-ai/backend/internal/dashboard"
	"github.com/soypiipe/energy-ai/backend/internal/db"
	"github.com/soypiipe/energy-ai/backend/internal/explain"
	"github.com/soypiipe/energy-ai/backend/internal/httpx"
	"github.com/soypiipe/energy-ai/backend/internal/meter"
	"github.com/soypiipe/energy-ai/backend/internal/seed"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("la API terminó con error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// El contexto se cancela con Ctrl+C o SIGTERM (docker stop) → apagado ordenado.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	loaded, err := seed.Run(ctx, pool, cfg.DataDir)
	if err != nil {
		return err
	}
	slog.Info("datos listos", "seed_aplicado", loaded)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			httpx.WriteError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Base de datos no disponible")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	meterRepo := meter.NewRepository(pool)
	meter.NewHandler(meterRepo).Register(mux)

	analysisRepo := analysis.NewRepository(pool)
	analysis.NewHandler(analysisRepo).Register(mux)
	dashboard.NewHandler(meterRepo, analysisRepo).Register(mux)

	// Explicaciones: plantilla determinista siempre; con LLM configurado se intenta primero y la plantilla
	// queda de respaldo. La clave nunca se loguea.
	var explainer explain.Explainer = explain.NewTemplate()
	if cfg.LLMEnabled() {
		explainer = explain.NewFallback(explain.NewLLM(cfg.LLMBaseURL, cfg.LLMModel, cfg.LLMAPIKey), explainer)
		slog.Info("explicaciones con LLM (respaldo: plantilla)", "model", cfg.LLMModel)
	} else {
		slog.Info("explicaciones con plantilla (LLM_API_KEY no definida)")
	}

	// El worker corre en segundo plano en el mismo proceso; se detiene con el mismo contexto de apagado.
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		analysis.NewWorker(analysisRepo, analysis.NewAnalyzer(analysisRepo, explainer).Process).Run(ctx)
	}()
	defer workers.Wait() // el pool se cierra (defer anterior) solo después de que el worker termine

	handler := httpx.Chain(mux,
		httpx.Recoverer,
		httpx.Logger,
		httpx.CORS(cfg.AllowedOrigins),
		httpx.LimitBody(1<<20), // 1 MB
	)

	// Timeouts explícitos: el servidor por defecto de Go no tiene ninguno (riesgo de Slowloris).
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("API escuchando", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("apagando API...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
