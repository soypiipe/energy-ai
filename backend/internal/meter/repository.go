package meter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/soypiipe/energy-ai/backend/internal/httpx"
)

var ErrNotFound = errors.New("medidor no encontrado")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// summarySQL calcula consumo total, actual (últimas 24 h) y baseline por medidor, y le une la
// anomalía de mayor prioridad del último análisis completado. $1 = meter_id o NULL para todos.
//
// El "ahora" es el último timestamp del dataset, no now(): los datos son históricos.
const summarySQL = `
WITH bounds AS (
    SELECT min(ts) AS t0, max(ts) AS t1 FROM readings
),
agg AS (
    SELECT r.meter_id,
           sum(r.consumption_kwh)                                              AS total_kwh,
           sum(r.consumption_kwh) FILTER (WHERE r.ts > b.t1 - interval '24 hours') AS current_kwh,
           sum(r.consumption_kwh) FILTER (WHERE r.ts < b.t0 + make_interval(days => $2)) / $2 AS baseline_kwh,
           max(r.ts)                                                           AS last_reading_at
    FROM readings r CROSS JOIN bounds b
    WHERE $1::text IS NULL OR r.meter_id = $1
    GROUP BY r.meter_id
),
latest_run AS (
    SELECT id FROM analysis_runs
    WHERE status = 'COMPLETED'
    ORDER BY finished_at DESC
    LIMIT 1
),
top_anomaly AS (
    SELECT DISTINCT ON (a.meter_id) a.meter_id, a.type, a.severity
    FROM anomalies a JOIN latest_run l ON a.analysis_id = l.id
    ORDER BY a.meter_id, a.priority_score DESC
)
SELECT m.meter_id, m.name, m.location,
       agg.total_kwh, agg.current_kwh, agg.baseline_kwh, agg.last_reading_at,
       t.type, t.severity
FROM meters m
JOIN agg ON agg.meter_id = m.meter_id
LEFT JOIN top_anomaly t ON t.meter_id = m.meter_id
ORDER BY m.meter_id`

func (r *Repository) ListSummaries(ctx context.Context) ([]Summary, error) {
	return r.querySummaries(ctx, nil)
}

func (r *Repository) GetDetail(ctx context.Context, meterID string) (Detail, error) {
	summaries, err := r.querySummaries(ctx, &meterID)
	if err != nil {
		return Detail{}, err
	}
	if len(summaries) == 0 {
		return Detail{}, ErrNotFound
	}

	rows, err := r.pool.Query(ctx,
		`SELECT ts, type, description FROM events WHERE meter_id = $1 ORDER BY ts`, meterID)
	if err != nil {
		return Detail{}, fmt.Errorf("consultar eventos: %w", err)
	}
	events, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Event, error) {
		var e Event
		var ts time.Time
		err := row.Scan(&ts, &e.Type, &e.Description)
		e.Timestamp = httpx.LocalTime(ts)
		return e, err
	})
	if err != nil {
		return Detail{}, fmt.Errorf("leer eventos: %w", err)
	}

	return Detail{Summary: summaries[0], Events: events}, nil
}

// Readings devuelve las lecturas de un medidor en [from, to]; los límites son opcionales.
func (r *Repository) Readings(ctx context.Context, meterID string, from, to *time.Time) ([]Reading, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM meters WHERE meter_id = $1)`, meterID,
	).Scan(&exists); err != nil {
		return nil, fmt.Errorf("verificar medidor: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}

	rows, err := r.pool.Query(ctx, `
		SELECT ts, consumption_kwh, voltage_v, current_a, power_factor, status
		FROM readings
		WHERE meter_id = $1
		  AND ($2::timestamp IS NULL OR ts >= $2)
		  AND ($3::timestamp IS NULL OR ts <= $3)
		ORDER BY ts`, meterID, from, to)
	if err != nil {
		return nil, fmt.Errorf("consultar lecturas: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Reading, error) {
		var rd Reading
		var ts time.Time
		err := row.Scan(&ts, &rd.ConsumptionKWh, &rd.VoltageV, &rd.CurrentA, &rd.PowerFactor, &rd.Status)
		rd.Timestamp = httpx.LocalTime(ts)
		return rd, err
	})
}

func (r *Repository) querySummaries(ctx context.Context, meterID *string) ([]Summary, error) {
	rows, err := r.pool.Query(ctx, summarySQL, meterID, BaselineDays)
	if err != nil {
		return nil, fmt.Errorf("consultar resumen de medidores: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Summary, error) {
		var s Summary
		var last time.Time
		if err := row.Scan(&s.MeterID, &s.Name, &s.Location,
			&s.TotalKWh, &s.CurrentKWh, &s.BaselineKWh, &last,
			&s.AnomalyType, &s.Severity); err != nil {
			return s, err
		}
		s.LastReadingAt = httpx.LocalTime(last)
		s.TotalKWh = round(s.TotalKWh, 1)
		s.CurrentKWh = round(s.CurrentKWh, 1)
		s.BaselineKWh = round(s.BaselineKWh, 1)
		s.VariationPct = VariationPct(s.CurrentKWh, s.BaselineKWh)
		s.Status = StatusFor(s.AnomalyType, s.Severity)
		return s, nil
	})
}
