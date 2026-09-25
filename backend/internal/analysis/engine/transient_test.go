package engine

import (
	"testing"
	"time"
)

func TestTransientDeviationSynthetic(t *testing.T) {
	// 12 h a 6 kWh (-80%) el día 9 desde las 00:00, luego vuelve.
	rs := series(14, func(d, h int) float64 {
		if d == 8 && h < 12 {
			return 6
		}
		return 30
	})
	b, _ := BuildBaseline(rs, MetricConsumption)
	fs := DetectTransientDeviation(rs, b)
	if len(fs) != 1 {
		t.Fatalf("hallazgos = %d, quería 1", len(fs))
	}
	f := fs[0]
	if f.Kind != FindingTransientDeviation {
		t.Errorf("Kind = %s", f.Kind)
	}
	if want := day0.AddDate(0, 0, 8); !f.Start.Equal(want) {
		t.Errorf("Start = %v, quería %v", f.Start, want)
	}
	if want := day0.AddDate(0, 0, 8).Add(11 * time.Hour); !f.End.Equal(want) {
		t.Errorf("End = %v, quería %v", f.End, want)
	}
	if f.Evidence[0].ChangePct != -80 {
		t.Errorf("ChangePct = %v, quería -80", f.Evidence[0].ChangePct)
	}
}

func TestTransientDeviationIgnores(t *testing.T) {
	cases := map[string]func(d, h int) float64{
		"lectura suelta":       func(d, h int) float64 { return pick(d == 9 && h == 5, 300, 30) },
		"2 horas":              func(d, h int) float64 { return pick(d == 9 && (h == 5 || h == 6), 300, 30) },
		"cambio persistente":   func(d, h int) float64 { return pick(d*24+h >= 9*24, 60, 30) },
		"racha hasta el final": func(d, h int) float64 { return pick(d == 13 && h >= 14, 90, 30) },
		"racha de 30 h":        func(d, h int) float64 { return pick(d*24+h >= 9*24 && d*24+h < 9*24+30, 90, 30) },
	}
	for name, f := range cases {
		rs := series(14, f)
		b, _ := BuildBaseline(rs, MetricConsumption)
		if fs := DetectTransientDeviation(rs, b); len(fs) != 0 {
			t.Errorf("%s: no debía haber hallazgos, hay %+v", name, fs)
		}
	}
}

func pick(cond bool, a, b float64) float64 {
	if cond {
		return a
	}
	return b
}

func TestTransientDeviationRealData(t *testing.T) {
	readings, _ := realData(t)
	detect := func(m string) []Finding {
		rs := sortReadings(readings[m])
		b, err := BuildBaseline(rs, MetricConsumption)
		if err != nil {
			t.Fatal(err)
		}
		return DetectTransientDeviation(rs, b)
	}
	fs := detect("M-106")
	if len(fs) != 1 {
		t.Fatalf("M-106: hallazgos = %d, quería 1", len(fs))
	}
	if want := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC); !fs[0].Start.Equal(want) {
		t.Errorf("M-106 Start = %v, quería %v", fs[0].Start, want)
	}
	if h := fs[0].End.Sub(fs[0].Start).Hours() + 1; h != 12 {
		t.Errorf("M-106 duración = %v h, quería 12", h)
	}
	for _, m := range append([]string{"M-104", "M-109", "M-112"}, controlMeters...) {
		if fs := detect(m); len(fs) != 0 {
			t.Errorf("%s: no debía tener desviación transitoria: %+v", m, fs)
		}
	}
}
