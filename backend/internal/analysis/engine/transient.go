package engine

const (
	// transientZ: z robusto mínimo para considerar una lectura fuera de lo normal.
	transientZ = 4.0

	// transientMinHours: las lecturas sueltas no cuentan. Con solo 7 muestras por hora el MAD es
	// inestable y los medidores sanos tienen picos aislados de z alto; una desviación real dura varias horas.
	transientMinHours = 3

	// transientMinChange: además del z, la desviación debe ser material (≥25%, igual que el cambio persistente).
	transientMinChange = shiftThreshold
)

// DetectTransientDeviation busca desviaciones que se salen de lo normal durante unas horas y luego vuelven.
//
// Es una racha de ≥3 lecturas seguidas con |z| ≥ 4 del mismo lado y ≥25% de cambio medio. Solo se
// reporta si dura menos de 24 h y vuelve a lo normal antes de terminar los datos; si no, es un cambio de
// nivel y lo cubre DetectPersistentShift, así un mismo evento no sale dos veces.
func DetectTransientDeviation(readings []Reading, b Baseline) []Finding {
	dir := make([]int, len(readings))
	for i, r := range readings {
		z := b.RobustZ(r.Timestamp, r.ConsumptionKWh)
		switch {
		case z >= transientZ:
			dir[i] = 1
		case z <= -transientZ:
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
		if n := j - i; n >= transientMinHours && n < shiftMinHours && j < len(dir) {
			f := shiftFinding(readings, b, i, j, len(readings))
			f.Kind = FindingTransientDeviation
			if abs(f.Evidence[0].ChangePct)/100 >= transientMinChange {
				out = append(out, f)
			}
		}
		i = j
	}
	return out
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
