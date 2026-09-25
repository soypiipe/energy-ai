package engine

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// realData carga data/*.csv agrupado por medidor. Es la misma fuente que usa el seed.
func realData(t *testing.T) (map[string][]Reading, map[string][]Event) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "..", "data")
	readings := map[string][]Reading{}
	for _, row := range readCSV(t, filepath.Join(dir, "readings.csv")) {
		readings[row[0]] = append(readings[row[0]], Reading{
			Timestamp:      mustTime(t, row[1]),
			ConsumptionKWh: mustFloat(t, row[2]),
			VoltageV:       mustFloat(t, row[3]),
			CurrentA:       mustFloat(t, row[4]),
			PowerFactor:    mustFloat(t, row[5]),
		})
	}
	events := map[string][]Event{}
	for _, row := range readCSV(t, filepath.Join(dir, "events.csv")) {
		events[row[0]] = append(events[row[0]], Event{
			Timestamp:   mustTime(t, row[1]),
			Type:        EventType(row[2]),
			Description: row[3],
		})
	}
	return readings, events
}

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows[1:]
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if v, err := time.Parse(layout, s); err == nil {
			return v
		}
	}
	t.Fatalf("fecha inválida %q", s)
	return time.Time{}
}

func mustFloat(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// controlMeters son los 8 medidores sin anomalías esperadas.
var controlMeters = []string{"M-101", "M-102", "M-103", "M-105", "M-107", "M-108", "M-110", "M-111"}
