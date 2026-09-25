package engine

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

// Los enums del motor deben coincidir con los CHECK de la migración; si no, el INSERT
// fallaría en tiempo de ejecución. El test lee el SQL para detectar el desfase antes.
func TestEnumsMatchDatabaseChecks(t *testing.T) {
	sql, err := os.ReadFile("../../db/migrations/001_init.sql")
	if err != nil {
		t.Fatalf("leyendo migración: %v", err)
	}

	cases := []struct {
		column string
		want   []string
	}{
		{"type", []string{string(AnomalyReal), string(AnomalyExplainable), string(AnomalyFalsePos), string(AnomalyDataQuality)}},
		{"severity", []string{string(SeverityLow), string(SeverityMedium), string(SeverityHigh)}},
	}
	for _, c := range cases {
		re := regexp.MustCompile(`CHECK \(` + c.column + ` IN \(([^)]*)\)\)`)
		m := re.FindSubmatch(sql)
		if m == nil {
			t.Fatalf("no se encontró el CHECK de %q", c.column)
		}
		got := regexp.MustCompile(`'([A-Z_]+)'`).FindAllStringSubmatch(string(m[1]), -1)
		var values []string
		for _, g := range got {
			values = append(values, g[1])
		}
		slices.Sort(values)
		want := slices.Clone(c.want)
		slices.Sort(want)
		if !slices.Equal(values, want) {
			t.Errorf("%s: la BD permite %v, el motor define %v", c.column, values, want)
		}
	}
}

func TestSeverityRank(t *testing.T) {
	if !(SeverityHigh.Rank() > SeverityMedium.Rank() && SeverityMedium.Rank() > SeverityLow.Rank()) {
		t.Error("el orden debe ser HIGH > MEDIUM > LOW")
	}
	if Severity("CRITICAL").Rank() != 0 {
		t.Error("un valor desconocido debe tener rango 0")
	}
}
