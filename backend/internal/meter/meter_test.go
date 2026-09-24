package meter

import "testing"

func TestVariationPct(t *testing.T) {
	cases := []struct {
		current, baseline, want float64
	}{
		{2180, 1070, 103.7}, // ejemplo del enunciado (M-109)
		{1000, 1000, 0},
		{500, 1000, -50},
		{100, 0, 0}, // sin baseline no se divide por cero
	}
	for _, c := range cases {
		if got := VariationPct(c.current, c.baseline); got != c.want {
			t.Errorf("VariationPct(%v, %v) = %v; se esperaba %v", c.current, c.baseline, got, c.want)
		}
	}
}

func TestStatusFor(t *testing.T) {
	s := func(v string) *string { return &v }
	cases := []struct {
		name          string
		typ, severity *string
		want          string
	}{
		{"sin anomalía", nil, nil, StatusOK},
		{"falso positivo", s("FALSE_POSITIVE"), s("LOW"), StatusOK},
		{"anomalía real alta", s("REAL_ANOMALY"), s("HIGH"), StatusCritical},
		{"calidad de datos alta", s("DATA_QUALITY"), s("HIGH"), StatusAlert},
		{"explicable media", s("EXPLAINABLE_ANOMALY"), s("MEDIUM"), StatusAlert},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StatusFor(c.typ, c.severity); got != c.want {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}
