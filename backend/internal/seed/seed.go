// Package seed carga los CSV entregados en la base de datos la primera vez que arranca la API.
//
// Solo valida la ESTRUCTURA (columnas, números y fechas parseables). No descarta valores
// "raros" (voltajes imposibles, FP bajos...): detectarlos es trabajo del motor de análisis.
// Si los filtráramos aquí, esconderíamos justo los problemas de calidad que hay que encontrar.
package seed

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Reading struct {
	MeterID        string
	Timestamp      time.Time
	ConsumptionKWh float64
	VoltageV       float64
	CurrentA       float64
	PowerFactor    float64
	Status         string
}

type Event struct {
	MeterID     string
	Timestamp   time.Time
	Type        string
	Description string
}

// Run carga los datos solo si la base está vacía (idempotente: reiniciar la API no duplica nada).
func Run(ctx context.Context, pool *pgxpool.Pool, dataDir string) (bool, error) {
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM meters`).Scan(&count); err != nil {
		return false, fmt.Errorf("contar medidores: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	readings, err := parseFile(filepath.Join(dataDir, "readings.csv"), ParseReadings)
	if err != nil {
		return false, err
	}
	events, err := parseFile(filepath.Join(dataDir, "events.csv"), ParseEvents)
	if err != nil {
		return false, err
	}

	// Todo en una transacción: o quedan todos los datos o ninguno.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op si ya hubo commit

	for _, id := range uniqueMeterIDs(readings) {
		if _, err := tx.Exec(ctx,
			`INSERT INTO meters (meter_id, name) VALUES ($1, $2)`, id, "Medidor "+id,
		); err != nil {
			return false, fmt.Errorf("insertar medidor %s: %w", id, err)
		}
	}

	// COPY es el mecanismo de carga masiva de Postgres: mucho más rápido que 4.032 INSERTs.
	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"readings"},
		[]string{"meter_id", "ts", "consumption_kwh", "voltage_v", "current_a", "power_factor", "status"},
		pgx.CopyFromSlice(len(readings), func(i int) ([]any, error) {
			r := readings[i]
			return []any{r.MeterID, r.Timestamp, r.ConsumptionKWh, r.VoltageV, r.CurrentA, r.PowerFactor, r.Status}, nil
		}),
	); err != nil {
		return false, fmt.Errorf("copiar lecturas: %w", err)
	}

	for _, e := range events {
		if _, err := tx.Exec(ctx,
			`INSERT INTO events (meter_id, ts, type, description) VALUES ($1, $2, $3, $4)`,
			e.MeterID, e.Timestamp, e.Type, e.Description,
		); err != nil {
			return false, fmt.Errorf("insertar evento de %s: %w", e.MeterID, err)
		}
	}

	return true, tx.Commit(ctx)
}

func parseFile[T any](path string, parse func(io.Reader) ([]T, error)) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("abrir %s: %w", path, err)
	}
	defer f.Close()
	rows, err := parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return rows, nil
}

// ParseReadings convierte el CSV de lecturas. Formato de fecha: "2026-09-01 00:00:00".
func ParseReadings(r io.Reader) ([]Reading, error) {
	records, err := readCSV(r, []string{"meter_id", "timestamp", "consumption_kwh", "voltage_v", "current_a", "power_factor", "status"})
	if err != nil {
		return nil, err
	}
	out := make([]Reading, 0, len(records))
	for i, rec := range records {
		line := i + 2 // +1 por el encabezado, +1 porque las líneas empiezan en 1
		ts, err := time.Parse("2006-01-02 15:04:05", rec[1])
		if err != nil {
			return nil, fmt.Errorf("línea %d: timestamp inválido %q", line, rec[1])
		}
		nums, err := parseFloats(rec[2:6])
		if err != nil {
			return nil, fmt.Errorf("línea %d: %w", line, err)
		}
		out = append(out, Reading{
			MeterID: rec[0], Timestamp: ts,
			ConsumptionKWh: nums[0], VoltageV: nums[1], CurrentA: nums[2], PowerFactor: nums[3],
			Status: rec[6],
		})
	}
	return out, nil
}

// ParseEvents convierte el CSV de eventos. Formato de fecha: "2026-09-11 00:00".
func ParseEvents(r io.Reader) ([]Event, error) {
	records, err := readCSV(r, []string{"meter_id", "event_timestamp", "event_type", "description"})
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(records))
	for i, rec := range records {
		ts, err := time.Parse("2006-01-02 15:04", rec[1])
		if err != nil {
			return nil, fmt.Errorf("línea %d: timestamp inválido %q", i+2, rec[1])
		}
		out = append(out, Event{MeterID: rec[0], Timestamp: ts, Type: rec[2], Description: rec[3]})
	}
	return out, nil
}

// readCSV lee todo el archivo y verifica que el encabezado sea exactamente el esperado.
func readCSV(r io.Reader, header []string) ([][]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = len(header)
	cr.TrimLeadingSpace = true

	got, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("leer encabezado: %w", err)
	}
	if strings.Join(got, ",") != strings.Join(header, ",") {
		return nil, fmt.Errorf("encabezado inesperado: %v (se esperaba %v)", got, header)
	}

	records, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("el archivo no tiene filas")
	}
	return records, nil
}

func parseFloats(fields []string) ([]float64, error) {
	out := make([]float64, len(fields))
	for i, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return nil, fmt.Errorf("número inválido %q", f)
		}
		out[i] = v
	}
	return out, nil
}

func uniqueMeterIDs(readings []Reading) []string {
	seen := map[string]bool{}
	var ids []string
	for _, r := range readings {
		if !seen[r.MeterID] {
			seen[r.MeterID] = true
			ids = append(ids, r.MeterID)
		}
	}
	sort.Strings(ids)
	return ids
}
