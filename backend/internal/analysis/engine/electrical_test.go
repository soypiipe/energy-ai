package engine

import (
	"testing"
	"time"
)

func TestElectricalChangesMergesOverlapping(t *testing.T) {
	// desde el día 10 la corriente se duplica y el FP baja 20%; el voltaje no cambia.
	from := 9 * 24
	rs := series(14, func(_, _ int) float64 { return 30 })
	for i := range rs {
		rs[i].VoltageV, rs[i].CurrentA, rs[i].PowerFactor = 220, 100, 0.95
		if i >= from {
			rs[i].CurrentA, rs[i].PowerFactor = 200, 0.76
		}
	}
	fs, err := DetectElectricalChanges(rs)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 {
		t.Fatalf("hallazgos = %d, quería 1 (unidos): %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Kind != FindingElectricalChange || !f.Start.Equal(day0.Add(time.Duration(from)*time.Hour)) || !f.End.IsZero() {
		t.Errorf("hallazgo = %+v", f)
	}
	got := map[string]float64{}
	for _, e := range f.Evidence {
		got[e.Metric] = e.ChangePct
	}
	if len(got) != 2 || got["current_a"] != 100 || got["power_factor"] != -20 {
		t.Errorf("evidencia = %v, quería current_a=+100 y power_factor=-20", got)
	}
}

func TestElectricalChangesQuietMeter(t *testing.T) {
	rs := series(14, func(_, _ int) float64 { return 30 })
	for i := range rs {
		rs[i].VoltageV, rs[i].CurrentA, rs[i].PowerFactor = 220, 100, 0.95
	}
	if fs, err := DetectElectricalChanges(rs); err != nil || len(fs) != 0 {
		t.Errorf("fs=%+v err=%v", fs, err)
	}
}

func TestElectricalChangesRealData(t *testing.T) {
	readings, _ := realData(t)
	detect := func(m string) []Finding {
		fs, err := DetectElectricalChanges(sortReadings(readings[m]))
		if err != nil {
			t.Fatal(err)
		}
		return fs
	}
	fs := detect("M-109")
	if len(fs) != 1 {
		t.Fatalf("M-109: hallazgos = %d, quería 1: %+v", len(fs), fs)
	}
	metrics := map[string]Evidence{}
	for _, e := range fs[0].Evidence {
		metrics[e.Metric] = e
	}
	if c := metrics["current_a"]; c.ChangePct < 90 {
		t.Errorf("M-109 corriente = %+v, quería ~+100%%", c)
	}
	if p := metrics["power_factor"]; p.ChangePct > -15 {
		t.Errorf("M-109 FP = %+v, quería ~-20%%", p)
	}
	// M-104 y M-106 mueven la corriente porque cambia la carga (coherente con su consumo): lo decide la
	// clasificación. Los controles y M-112 (sus saltos son lecturas sueltas) no deben disparar nada.
	for _, m := range append([]string{"M-112"}, controlMeters...) {
		if fs := detect(m); len(fs) != 0 {
			t.Errorf("%s: no debía tener cambios eléctricos: %+v", m, fs)
		}
	}
}
