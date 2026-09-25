package engine

import (
	"math"
	"testing"
	"time"
)

var day0 = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

// series arma `days` días de lecturas horarias; f decide el consumo de cada (día, hora).
func series(days int, f func(day, hour int) float64) []Reading {
	var out []Reading
	for d := 0; d < days; d++ {
		for h := 0; h < 24; h++ {
			out = append(out, Reading{
				Timestamp:      day0.AddDate(0, 0, d).Add(time.Duration(h) * time.Hour),
				ConsumptionKWh: f(d, h),
			})
		}
	}
	return out
}

func TestMedianAndMAD(t *testing.T) {
	if got := median([]float64{5, 1, 3}); got != 3 {
		t.Errorf("mediana impar = %v, quería 3", got)
	}
	if got := median([]float64{4, 1, 3, 2}); got != 2.5 {
		t.Errorf("mediana par = %v, quería 2.5", got)
	}
	// desviaciones respecto a 3: 2,0,1,0,97 → ordenadas 0,0,1,2,97 → MAD = 1
	if got := mad([]float64{1, 3, 4, 3, 100}, 3); got != 1 {
		t.Errorf("MAD = %v, quería 1", got)
	}
}

func TestBuildBaselinePerHourOfDay(t *testing.T) {
	// noche 20 kWh, día 40 kWh: el baseline debe distinguir la hora.
	rs := series(10, func(_, h int) float64 {
		if h < 6 {
			return 20
		}
		return 40
	})
	b, err := BuildBaseline(rs, MetricConsumption)
	if err != nil {
		t.Fatal(err)
	}
	if b.Median[3] != 20 || b.Median[12] != 40 {
		t.Errorf("medianas = %v (h3) y %v (h12)", b.Median[3], b.Median[12])
	}
	if got := b.Expected(day0.Add(30 * time.Hour)); got != 40 { // día 2 a las 06:00
		t.Errorf("Expected(06:00) = %v, quería 40", got)
	}
}

func TestBuildBaselineIgnoresDataAfterDay7(t *testing.T) {
	// del día 8 en adelante el consumo se duplica; el baseline no debe enterarse.
	rs := series(14, func(d, _ int) float64 {
		if d >= 7 {
			return 100
		}
		return 30
	})
	b, err := BuildBaseline(rs, MetricConsumption)
	if err != nil {
		t.Fatal(err)
	}
	for h := 0; h < 24; h++ {
		if b.Median[h] != 30 {
			t.Fatalf("hora %d: mediana %v, quería 30", h, b.Median[h])
		}
	}
}

func TestBuildBaselineResistsOutlier(t *testing.T) {
	// un día base con un pico enorme a las 10:00 no mueve la mediana.
	rs := series(7, func(d, h int) float64 {
		if d == 3 && h == 10 {
			return 500
		}
		return 30 + float64(d%2)
	})
	b, err := BuildBaseline(rs, MetricConsumption)
	if err != nil {
		t.Fatal(err)
	}
	if b.Median[10] > 31 {
		t.Errorf("la mediana se contaminó: %v", b.Median[10])
	}
}

func TestBuildBaselineErrors(t *testing.T) {
	if _, err := BuildBaseline(nil, MetricConsumption); err == nil {
		t.Error("sin lecturas debe fallar")
	}
	if _, err := BuildBaseline(series(3, func(_, _ int) float64 { return 1 }), MetricConsumption); err == nil {
		t.Error("con 3 días (menos de 5 muestras por hora) debe fallar")
	}
}

func TestSigmaAndRobustZ(t *testing.T) {
	// valores constantes → MAD 0 → sigma cae al piso de 1% de la mediana.
	b, err := BuildBaseline(series(7, func(_, _ int) float64 { return 50 }), MetricConsumption)
	if err != nil {
		t.Fatal(err)
	}
	ts := day0.Add(9 * time.Hour)
	if got := b.Sigma(ts); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("sigma = %v, quería 0.5 (piso 1%%)", got)
	}
	if z := b.RobustZ(ts, 51); math.Abs(z-2) > 1e-9 {
		t.Errorf("z = %v, quería 2", z)
	}
	if z := b.RobustZ(ts, 50); z != 0 {
		t.Errorf("z en el valor esperado = %v, quería 0", z)
	}
}

func TestMetricValue(t *testing.T) {
	r := Reading{ConsumptionKWh: 1, VoltageV: 2, CurrentA: 3, PowerFactor: 4}
	for m, want := range map[Metric]float64{MetricConsumption: 1, MetricVoltage: 2, MetricCurrent: 3, MetricPowerFactor: 4} {
		if got := m.Value(r); got != want {
			t.Errorf("%s = %v, quería %v", m, got, want)
		}
	}
}
