package engine

import (
	"testing"
	"time"
)

// healthy arma 14 días de lecturas coherentes: 30 kWh, 220 V, 0.95 FP y corriente que cierra la física.
func healthyReadings() []Reading {
	rs := series(14, func(d, h int) float64 { return 30 + float64((d+h)%5) }) // varía para no parecer congelado
	for i := range rs {
		rs[i].VoltageV, rs[i].PowerFactor = 220, 0.95
		rs[i].CurrentA = rs[i].ConsumptionKWh * 1000 / (220 * 0.95) / 1.06
	}
	return rs
}

func TestDataQualityHealthy(t *testing.T) {
	if fs := DetectDataQuality(healthyReadings()); len(fs) != 0 {
		t.Errorf("no debía haber hallazgos: %+v", fs)
	}
}

func TestDataQualityScatteredIncoherence(t *testing.T) {
	rs := healthyReadings()
	start := 12*24 + 3
	for k := 0; k < 8; k++ { // 8 saltos sueltos de voltaje/corriente incoherentes
		i := start + k*3
		rs[i].CurrentA *= 3
	}
	fs := DetectDataQuality(rs)
	if len(fs) != 1 {
		t.Fatalf("hallazgos = %d, quería 1", len(fs))
	}
	f := fs[0]
	if f.Kind != FindingDataQuality || !f.Start.Equal(rs[start].Timestamp) {
		t.Errorf("hallazgo = %+v", f)
	}
	var flagged float64
	for _, e := range f.Evidence {
		if e.Metric == "flagged_readings" {
			flagged = e.Observed
		}
	}
	if flagged != 8 {
		t.Errorf("flagged_readings = %v, quería 8", flagged)
	}
}

func TestDataQualityFewIsNotEnough(t *testing.T) {
	rs := healthyReadings()
	for _, i := range []int{300, 305, 310, 320} { // 4 < 5
		rs[i].CurrentA *= 3
	}
	if fs := DetectDataQuality(rs); len(fs) != 0 {
		t.Errorf("4 lecturas sueltas no bastan: %+v", fs)
	}
}

func TestDataQualityImpossibleValues(t *testing.T) {
	rs := healthyReadings()
	for i := 200; i < 205; i++ {
		rs[i].PowerFactor = 1.4 // FP > 1 no existe
	}
	if fs := DetectDataQuality(rs); len(fs) != 1 {
		t.Errorf("FP>1 debía marcarse: %+v", fs)
	}
}

func TestDataQualityStuckMeter(t *testing.T) {
	rs := healthyReadings()
	for i := 100; i < 105; i++ { // 5 lecturas de consumo idéntico
		rs[i].ConsumptionKWh = 31.5
		rs[i].CurrentA = rs[100].CurrentA
	}
	fs := DetectDataQuality(rs)
	if len(fs) != 1 {
		t.Fatalf("un medidor congelado debía marcarse: %+v", fs)
	}
	if fs[0].End.IsZero() {
		t.Error("terminó hace días: End no debería ser cero")
	}
}

func TestDataQualitySustainedRatioShiftIsNotDataQuality(t *testing.T) {
	// razón sostenida de 1.37 (como M-109): cambio de carga, no un sensor roto.
	rs := healthyReadings()
	for i := 12 * 24; i < len(rs); i++ {
		rs[i].CurrentA /= 1.3
	}
	if fs := DetectDataQuality(rs); len(fs) != 0 {
		t.Errorf("razón 1.37 sostenida no es calidad de datos: %+v", fs)
	}
}

func TestDataQualityRealData(t *testing.T) {
	readings, _ := realData(t)
	fs := DetectDataQuality(sortReadings(readings["M-112"]))
	if len(fs) != 1 {
		t.Fatalf("M-112: hallazgos = %d, quería 1", len(fs))
	}
	if s := fs[0].Start; s.Before(time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("M-112 Start = %v, antes del 13-sep", s)
	}
	for _, m := range append([]string{"M-104", "M-106", "M-109"}, controlMeters...) {
		if fs := DetectDataQuality(sortReadings(readings[m])); len(fs) != 0 {
			t.Errorf("%s: no debía tener calidad de datos: %+v", m, fs)
		}
	}
}
