package engine

import (
	"testing"
	"time"
)

func TestPersistentShiftSynthetic(t *testing.T) {
	// 14 días a 30 kWh; desde el día 10 a las 08:00 sube a 45 (+50%) y sigue hasta el final.
	shiftAt := 9*24 + 8
	rs := series(14, func(d, h int) float64 {
		if d*24+h >= shiftAt {
			return 45
		}
		return 30
	})
	b, err := BuildBaseline(rs, MetricConsumption)
	if err != nil {
		t.Fatal(err)
	}
	fs := DetectPersistentShift(rs, b)
	if len(fs) != 1 {
		t.Fatalf("hallazgos = %d, quería 1", len(fs))
	}
	f := fs[0]
	if want := day0.Add(time.Duration(shiftAt) * time.Hour); !f.Start.Equal(want) {
		t.Errorf("Start = %v, quería %v", f.Start, want)
	}
	if !f.End.IsZero() {
		t.Errorf("End = %v, debe ser cero (sigue activo)", f.End)
	}
	if ev := f.Evidence[0]; ev.ChangePct != 50 || ev.Baseline != 30 || ev.Observed != 45 {
		t.Errorf("evidencia = %+v", ev)
	}
}

func TestPersistentShiftIgnoresShortDrop(t *testing.T) {
	// caída de 12 h (como el mantenimiento programado): no es persistente.
	rs := series(14, func(d, h int) float64 {
		if d == 8 && h < 12 {
			return 6
		}
		return 30
	})
	b, _ := BuildBaseline(rs, MetricConsumption)
	if fs := DetectPersistentShift(rs, b); len(fs) != 0 {
		t.Errorf("no debía haber hallazgos, hay %+v", fs)
	}
}

func TestPersistentShiftEndsWhenItReturns(t *testing.T) {
	// 30 h a -40% y luego vuelve: hallazgo con End definido.
	rs := series(14, func(d, h int) float64 {
		if i := d*24 + h; i >= 9*24 && i < 9*24+30 {
			return 18
		}
		return 30
	})
	b, _ := BuildBaseline(rs, MetricConsumption)
	fs := DetectPersistentShift(rs, b)
	if len(fs) != 1 || fs[0].End.IsZero() || fs[0].Evidence[0].ChangePct != -40 {
		t.Fatalf("hallazgos = %+v", fs)
	}
}

func TestPersistentShiftIgnoresSingleSpike(t *testing.T) {
	rs := series(14, func(d, h int) float64 {
		if d == 9 && h == 10 {
			return 300
		}
		return 30
	})
	b, _ := BuildBaseline(rs, MetricConsumption)
	if fs := DetectPersistentShift(rs, b); len(fs) != 0 {
		t.Errorf("un pico suelto no es un cambio de nivel: %+v", fs)
	}
}

func TestPersistentShiftRealData(t *testing.T) {
	readings, _ := realData(t)
	detect := func(m string) []Finding {
		rs := sortReadings(readings[m])
		b, err := BuildBaseline(rs, MetricConsumption)
		if err != nil {
			t.Fatal(err)
		}
		return DetectPersistentShift(rs, b)
	}
	want := map[string]time.Time{
		"M-104": time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC),
		"M-109": time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC),
	}
	for m, start := range want {
		fs := detect(m)
		if len(fs) != 1 {
			t.Errorf("%s: hallazgos = %d, quería 1", m, len(fs))
			continue
		}
		if d := fs[0].Start.Sub(start); d < -time.Hour || d > time.Hour {
			t.Errorf("%s: Start = %v, quería ~%v", m, fs[0].Start, start)
		}
	}
	// M-106 (caída de 12 h) y M-112 (kWh estable) y los 8 controles: nada.
	for _, m := range append([]string{"M-106", "M-112"}, controlMeters...) {
		if fs := detect(m); len(fs) != 0 {
			t.Errorf("%s: no debía tener cambio persistente: %+v", m, fs)
		}
	}
}
