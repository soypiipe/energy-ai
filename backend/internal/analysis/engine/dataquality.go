package engine

import (
	"math"
	"time"
)

const (
	// Física: kWh ≈ V·I·FP/1000. En medidores sanos la razón por lectura oscila entre 0.82 y 1.30
	// (promedio ~1.06). Fuera de [0.7, 1.6] las magnitudes no pueden describir el mismo consumo.
	// Se deja margen a propósito: M-109 tiene una razón sostenida de ~1.37 (cambio real de carga, no
	// un sensor roto) y debe seguir viéndose como anomalía real, no como calidad de datos.
	ratioMin = 0.7
	ratioMax = 1.6

	// Un medidor con el consumo idéntico ≥4 lecturas seguidas está congelado.
	stuckMinRun = 4

	// Mínimo de lecturas sospechosas para reportar: no se marca un medidor por un dato suelto.
	dqMinFlagged = 5

	// Si la última lectura sospechosa cae en las últimas 24 h, el problema se considera vigente.
	dqActiveWindow = 24 * time.Hour
)

// kwhRatio es consumo / (V·I·FP/1000). Devuelve ok=false si la potencia aparente es ≤0 o no es finita.
func kwhRatio(r Reading) (float64, bool) {
	p := r.VoltageV * r.CurrentA * r.PowerFactor / 1000
	if p <= 0 || math.IsNaN(p) || math.IsInf(p, 0) {
		return 0, false
	}
	return r.ConsumptionKWh / p, true
}

// impossible: valores que no existen físicamente.
func impossible(r Reading) bool {
	vals := []float64{r.ConsumptionKWh, r.VoltageV, r.CurrentA, r.PowerFactor}
	for _, v := range vals {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return true
		}
	}
	return r.ConsumptionKWh < 0 || r.VoltageV <= 0 || r.CurrentA < 0 || r.PowerFactor <= 0 || r.PowerFactor > 1
}

// DetectDataQuality revisa si las lecturas de un medidor son físicamente coherentes.
//
// Marca cada lectura con (a) valores imposibles, (b) razón kWh/(V·I·FP) fuera de [0.7, 1.6] o
// (c) consumo congelado ≥4 lecturas seguidas. Si hay ≥5 marcadas devuelve un único hallazgo desde
// la primera. La columna `status` del CSV no se usa: siempre dice OK (docs/DESIGN.md §2).
//
// readings debe estar ordenado por tiempo.
func DetectDataQuality(readings []Reading) []Finding {
	flagged := make([]bool, len(readings))
	for i, r := range readings {
		if impossible(r) {
			flagged[i] = true
			continue
		}
		if ratio, ok := kwhRatio(r); !ok || ratio < ratioMin || ratio > ratioMax {
			flagged[i] = true
		}
	}
	for i := 0; i < len(readings); {
		j := i + 1
		for j < len(readings) && readings[j].ConsumptionKWh == readings[i].ConsumptionKWh {
			j++
		}
		if j-i >= stuckMinRun {
			for k := i; k < j; k++ {
				flagged[k] = true
			}
		}
		i = j
	}

	var count int
	var first, last time.Time
	var ratios []float64
	for i, r := range readings {
		if !flagged[i] {
			continue
		}
		count++
		if first.IsZero() {
			first = r.Timestamp
		}
		last = r.Timestamp
		if ratio, ok := kwhRatio(r); ok {
			ratios = append(ratios, ratio)
		}
	}
	if count < dqMinFlagged {
		return nil
	}

	// Referencia sana: mediana de la razón durante la ventana base.
	var healthy []float64
	if len(readings) > 0 {
		end := readings[0].Timestamp.AddDate(0, 0, BaselineDays)
		for i, r := range readings {
			if r.Timestamp.Before(end) && !flagged[i] {
				if ratio, ok := kwhRatio(r); ok {
					healthy = append(healthy, ratio)
				}
			}
		}
	}
	f := Finding{Kind: FindingDataQuality, Start: first}
	if len(healthy) > 0 && len(ratios) > 0 {
		f.Evidence = append(f.Evidence, newEvidence(Metric("kwh_vi_ratio"), median(healthy), median(ratios)))
	}
	// Cuántas lecturas se marcaron (baseline 0: en un medidor sano no debería haber ninguna).
	f.Evidence = append(f.Evidence, Evidence{Metric: "flagged_readings", Baseline: 0, Observed: float64(count)})

	lastTs := readings[len(readings)-1].Timestamp
	if lastTs.Sub(last) >= dqActiveWindow {
		f.End = last
	}
	return []Finding{f}
}
