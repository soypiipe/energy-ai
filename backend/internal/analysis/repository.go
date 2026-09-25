package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRunNotFound = errors.New("análisis no encontrado")

// Repository es el acceso a la cola analysis_runs (y, más adelante, a las anomalías).
// La cola vive en PostgreSQL (docs/DESIGN.md ADR 4): es transaccional, sobrevive reinicios y permite
// varios workers sin infraestructura extra.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const runColumns = `id, status, current_step, error, summary, created_at, started_at, finished_at`

func scanRun(row pgx.Row) (Run, error) {
	var r Run
	err := row.Scan(&r.ID, &r.Status, &r.CurrentStep, &r.Error, &r.Summary, &r.CreatedAt, &r.StartedAt, &r.FinishedAt)
	return r, err
}

// Enqueue encola una ejecución nueva en estado PENDING.
func (r *Repository) Enqueue(ctx context.Context) (Run, error) {
	run, err := scanRun(r.pool.QueryRow(ctx,
		`INSERT INTO analysis_runs DEFAULT VALUES RETURNING `+runColumns))
	if err != nil {
		return Run{}, fmt.Errorf("encolar análisis: %w", err)
	}
	return run, nil
}

// LatestCompletedRun devuelve el último análisis COMPLETED, o nil si nunca se completó uno.
func (r *Repository) LatestCompletedRun(ctx context.Context) (*Run, error) {
	run, err := scanRun(r.pool.QueryRow(ctx,
		`SELECT `+runColumns+` FROM analysis_runs WHERE status = 'COMPLETED' ORDER BY finished_at DESC LIMIT 1`))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("consultar último análisis completado: %w", err)
	}
	return &run, nil
}

// ActiveRun devuelve la ejecución más antigua que sigue PENDING o RUNNING, o nil si no hay ninguna.
func (r *Repository) ActiveRun(ctx context.Context) (*Run, error) {
	run, err := scanRun(r.pool.QueryRow(ctx, `
		SELECT `+runColumns+` FROM analysis_runs
		WHERE status IN ('PENDING','RUNNING')
		ORDER BY created_at
		LIMIT 1`))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("consultar análisis activo: %w", err)
	}
	return &run, nil
}

// Claim toma la ejecución PENDING más antigua y la pasa a RUNNING, de forma atómica.
// FOR UPDATE SKIP LOCKED hace que varios workers no se pisen: si otro ya bloqueó esa fila, se salta
// a la siguiente en vez de esperar. Devuelve nil si no hay nada pendiente.
func (r *Repository) Claim(ctx context.Context) (*Run, error) {
	run, err := scanRun(r.pool.QueryRow(ctx, `
		UPDATE analysis_runs
		SET status = 'RUNNING', started_at = now(), current_step = NULL
		WHERE id = (
			SELECT id FROM analysis_runs
			WHERE status = 'PENDING'
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING `+runColumns))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("tomar análisis pendiente: %w", err)
	}
	return &run, nil
}

// SetStep actualiza el paso actual de una ejecución en curso (lo que ve el usuario como progreso).
func (r *Repository) SetStep(ctx context.Context, id, step string) error {
	return r.mustAffectOne(ctx, id,
		`UPDATE analysis_runs SET current_step = $2 WHERE id = $1 AND status = 'RUNNING'`, step)
}

// Complete marca la ejecución como COMPLETED con un resumen JSON.
func (r *Repository) Complete(ctx context.Context, id string, summary any) error {
	raw, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("serializar resumen: %w", err)
	}
	return r.mustAffectOne(ctx, id, `
		UPDATE analysis_runs
		SET status = 'COMPLETED', current_step = NULL, summary = $2, finished_at = now()
		WHERE id = $1 AND status = 'RUNNING'`, raw)
}

// Fail marca la ejecución como FAILED con el mensaje de error (para el log/diagnóstico interno).
func (r *Repository) Fail(ctx context.Context, id, message string) error {
	return r.mustAffectOne(ctx, id, `
		UPDATE analysis_runs
		SET status = 'FAILED', error = $2, finished_at = now()
		WHERE id = $1 AND status = 'RUNNING'`, message)
}

// Get devuelve una ejecución por id.
func (r *Repository) Get(ctx context.Context, id string) (Run, error) {
	run, err := scanRun(r.pool.QueryRow(ctx,
		`SELECT `+runColumns+` FROM analysis_runs WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrRunNotFound
	}
	if err != nil {
		return Run{}, fmt.Errorf("consultar análisis: %w", err)
	}
	return run, nil
}

// RequeueStuck devuelve a PENDING las ejecuciones que llevan más de olderThan en RUNNING: el worker
// que las tomaba murió (reinicio, crash). Devuelve cuántas se recuperaron.
func (r *Repository) RequeueStuck(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE analysis_runs
		SET status = 'PENDING', started_at = NULL, current_step = NULL
		WHERE status = 'RUNNING' AND started_at < now() - make_interval(secs => $1)`,
		olderThan.Seconds())
	if err != nil {
		return 0, fmt.Errorf("recuperar análisis atascados: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Requeue devuelve a PENDING una ejecución RUNNING (p. ej. el worker se apagó a mitad de camino).
func (r *Repository) Requeue(ctx context.Context, id string) error {
	return r.mustAffectOne(ctx, id, `
		UPDATE analysis_runs
		SET status = 'PENDING', started_at = NULL, current_step = NULL
		WHERE id = $1 AND status = 'RUNNING'`)
}

// mustAffectOne ejecuta un UPDATE por id y falla si no tocó exactamente una fila
// (id inexistente o estado incorrecto: p. ej. completar algo que no está RUNNING).
func (r *Repository) mustAffectOne(ctx context.Context, id, sql string, args ...any) error {
	tag, err := r.pool.Exec(ctx, sql, append([]any{id}, args...)...)
	if err != nil {
		return fmt.Errorf("actualizar análisis %s: %w", id, err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("actualizar análisis %s: no existe o no está en el estado esperado", id)
	}
	return nil
}
