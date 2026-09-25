package engine

import (
	"math"
	"sort"
)

const (
	// shiftThreshold: un cambio de nivel cuenta si el consumo se aleja ≥25% de lo esperado a esa hora.
	shiftThreshold = 0.25

	// shiftMinHours: y se sostiene al menos 24 h seguidas. Una caída de 12 h no es un cambio persistente.
	shiftMinHours = 24
)

// DetectPersistentShift busca cambios de nivel sostenidos en el consumo respecto al baseline.
//
// Para cada lectura calcula ratio = observado / esperado (por hora del día). Suaviza con la mediana
// de 3 lecturas vecinas para que una lectura suelta no rompa ni cree una racha. Un hallazgo es una
// racha de ≥24 h seguidas donde el ratio queda fuera de 1±25% siempre del mismo lado.
// El ruido normal de los datos llega a ~±20%, por debajo del umbral.
//
// readings debe estar ordenado por tiempo. Devuelve los hallazgos en orden cronológico.
func DetectPersistentShift(readings []Reading, b Baseline) []Finding {
	return detectShifts(readings, b, shiftThreshold, FindingPersistentShift)
}

// detectShifts es la lógica común de "racha sostenida fuera de 1±threshold" sobre la métrica del baseline.
func detectShifts(readings []Reading, b Baseline, threshold float64, kind FindingKind) []Finding {
	if len(readings) < shiftMinHours {
		return nil
	}
	ratio := make([]float64, len(readings))
	for i, r := range readings {
		exp := b.Expected(r.Timestamp)
		if exp <= 0 {
			ratio[i] = 1 // sin referencia válida: no aporta señal
			continue
		}
		ratio[i] = b.Metric.Value(r) / exp
	}
	dir := make([]int, len(readings)) // +1 sube, -1 baja, 0 normal
	for i := range ratio {
		lo, hi := max(i-1, 0), min(i+1, len(ratio)-1)
		m := median(ratio[lo : hi+1])
		switch {
		case m >= 1+threshold:
			dir[i] = 1
		case m <= 1-threshold:
			dir[i] = -1
		}
	}

	var out []Finding
	for i := 0; i < len(dir); {
		if dir[i] == 0 {
			i++
			continue
		}
		j := i
		for j < len(dir) && dir[j] == dir[i] {
			j++
		}
		if j-i >= shiftMinHours {
			f := shiftFinding(readings, b, i, j, len(readings))
			f.Kind = kind
			out = append(out, f)
		}
		i = j
	}
	return out
}

// shiftFinding arma el hallazgo para readings[i:j]. End queda en cero si la racha llega al final de los datos.
func shiftFinding(readings []Reading, b Baseline, i, j, n int) Finding {
	var sumObs, sumExp float64
	for _, r := range readings[i:j] {
		sumObs += b.Metric.Value(r)
		sumExp += b.Expected(r.Timestamp)
	}
	cnt := float64(j - i)
	f := Finding{
		Kind:     FindingPersistentShift,
		Start:    readings[i].Timestamp,
		Evidence: []Evidence{newEvidence(b.Metric, sumExp/cnt, sumObs/cnt)},
	}
	if j < n {
		f.End = readings[j-1].Timestamp
	}
	return f
}

// newEvidence calcula el cambio porcentual de forma consistente para todos los detectores.
func newEvidence(metric Metric, baseline, observed float64) Evidence {
	return Evidence{
		Metric:    string(metric),
		Baseline:  round(baseline, 2),
		Observed:  round(observed, 2),
		ChangePct: changePct(baseline, observed),
	}
}

func changePct(baseline, observed float64) float64 {
	if baseline == 0 {
		return 0
	}
	return round((observed/baseline-1)*100, 1)
}

func round(x float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(x*p) / p
}

// sortReadings devuelve una copia ordenada por tiempo; los detectores asumen orden cronológico.
func sortReadings(rs []Reading) []Reading {
	out := append([]Reading(nil), rs...)
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	return out
}
