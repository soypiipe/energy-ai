package engine

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

const (
	// BaselineDays es la ventana de referencia: los primeros 7 días de datos (ver docs/DESIGN.md §6).
	BaselineDays = 7

	// minSamplesPerHour es el mínimo de días con lectura válida para fiarse de la mediana de una hora.
	minSamplesPerHour = 5

	// madToSigma vuelve el MAD comparable con una desviación estándar en datos normales.
	madToSigma = 1.4826

	// sigmaFloorPct evita dividir por ~0 cuando el MAD de una hora es casi nulo:
	// el ruido mínimo asumido es 1% de la mediana.
	sigmaFloorPct = 0.01
)

// Metric elige qué magnitud de una Reading se analiza.
type Metric string

const (
	MetricConsumption Metric = "consumption_kwh"
	MetricPowerFactor Metric = "power_factor"
	MetricCurrent     Metric = "current_a"
	MetricVoltage     Metric = "voltage_v"
)

// Value extrae la magnitud de una lectura.
func (m Metric) Value(r Reading) float64 {
	switch m {
	case MetricConsumption:
		return r.ConsumptionKWh
	case MetricPowerFactor:
		return r.PowerFactor
	case MetricCurrent:
		return r.CurrentA
	case MetricVoltage:
		return r.VoltageV
	}
	return math.NaN()
}

// Baseline es el comportamiento normal de una magnitud: mediana y MAD por hora del día.
// Mediana y MAD (en vez de media y desviación) resisten lecturas atípicas dentro de la ventana.
type Baseline struct {
	Metric Metric
	Median [24]float64
	MAD    [24]float64
}

// BuildBaseline calcula el baseline con las lecturas de los primeros BaselineDays días,
// contados desde la medianoche de la primera lectura.
func BuildBaseline(readings []Reading, metric Metric) (Baseline, error) {
	b := Baseline{Metric: metric}
	if len(readings) == 0 {
		return b, errors.New("sin lecturas")
	}
	first := readings[0].Timestamp
	for _, r := range readings[1:] {
		if r.Timestamp.Before(first) {
			first = r.Timestamp
		}
	}
	start := time.Date(first.Year(), first.Month(), first.Day(), 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, BaselineDays)

	var samples [24][]float64
	for _, r := range readings {
		if r.Timestamp.Before(start) || !r.Timestamp.Before(end) {
			continue
		}
		v := metric.Value(r)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		h := r.Timestamp.Hour()
		samples[h] = append(samples[h], v)
	}
	for h := range samples {
		if len(samples[h]) < minSamplesPerHour {
			return b, fmt.Errorf("hora %02d: %d muestras en la ventana base, se necesitan %d", h, len(samples[h]), minSamplesPerHour)
		}
		b.Median[h] = median(samples[h])
		b.MAD[h] = mad(samples[h], b.Median[h])
	}
	return b, nil
}

// Expected es el valor normal esperado a esa hora del día.
func (b Baseline) Expected(t time.Time) float64 { return b.Median[t.Hour()] }

// Sigma es la dispersión robusta a esa hora (MAD escalado, con piso relativo a la mediana).
func (b Baseline) Sigma(t time.Time) float64 {
	h := t.Hour()
	floor := math.Abs(b.Median[h]) * sigmaFloorPct
	return math.Max(madToSigma*b.MAD[h], floor)
}

// RobustZ es cuántas "sigmas" se aleja el valor observado de lo esperado a esa hora.
func (b Baseline) RobustZ(t time.Time, observed float64) float64 {
	s := b.Sigma(t)
	if s == 0 {
		return 0
	}
	return (observed - b.Expected(t)) / s
}

func median(xs []float64) float64 {
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func mad(xs []float64, med float64) float64 {
	d := make([]float64, len(xs))
	for i, x := range xs {
		d[i] = math.Abs(x - med)
	}
	return median(d)
}
