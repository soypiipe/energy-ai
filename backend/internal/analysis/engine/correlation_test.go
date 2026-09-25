package engine

import (
	"testing"
	"time"
)

func TestCorrelateEvent(t *testing.T) {
	at := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	f := Finding{Start: at}
	ev := func(off time.Duration, typ EventType) Event {
		return Event{Timestamp: at.Add(off), Type: typ, Description: string(typ)}
	}
	tests := []struct {
		name        string
		events      []Event
		wantType    EventType // "" = ninguno
		wantExplain bool
	}{
		{"sin eventos", nil, "", false},
		{"operativo exacto", []Event{ev(0, EventOperationalChange)}, EventOperationalChange, true},
		{"apagado programado a +2h (borde)", []Event{ev(2*time.Hour, EventScheduledOutage)}, EventScheduledOutage, true},
		{"-2h (borde)", []Event{ev(-2*time.Hour, EventOperationalChange)}, EventOperationalChange, true},
		{"fuera de ventana (+3h)", []Event{ev(3*time.Hour, EventOperationalChange)}, "", false},
		{"UNKNOWN se relaciona pero no explica", []Event{ev(0, EventUnknown)}, EventUnknown, false},
		{"DATA_QUALITY no explica consumo", []Event{ev(0, EventDataQuality)}, EventDataQuality, false},
		{"gana el que explica aunque esté más lejos", []Event{ev(0, EventUnknown), ev(90*time.Minute, EventOperationalChange)}, EventOperationalChange, true},
		{"entre dos que explican, el más cercano", []Event{ev(90*time.Minute, EventScheduledOutage), ev(-30*time.Minute, EventOperationalChange)}, EventOperationalChange, true},
	}
	for _, tc := range tests {
		got, explains := CorrelateEvent(f, tc.events)
		if tc.wantType == "" {
			if got != nil {
				t.Errorf("%s: relacionado = %+v, quería nil", tc.name, got)
			}
			continue
		}
		if got == nil || got.Type != tc.wantType || explains != tc.wantExplain {
			t.Errorf("%s: got %+v explains=%v, quería %s explains=%v", tc.name, got, explains, tc.wantType, tc.wantExplain)
		}
	}
}

func TestCorrelateEventRealData(t *testing.T) {
	readings, events := realData(t)

	// M-104: cambio persistente desde 11-sep coincide con OPERATIONAL_CHANGE.
	rs := sortReadings(readings["M-104"])
	b, _ := BuildBaseline(rs, MetricConsumption)
	fs := DetectPersistentShift(rs, b)
	if ev, ok := CorrelateEvent(fs[0], events["M-104"]); ev == nil || !ok || ev.Type != EventOperationalChange {
		t.Errorf("M-104: %+v explains=%v", ev, ok)
	}

	// M-106: desviación transitoria coincide con SCHEDULED_OUTAGE.
	rs = sortReadings(readings["M-106"])
	b, _ = BuildBaseline(rs, MetricConsumption)
	fs = DetectTransientDeviation(rs, b)
	if ev, ok := CorrelateEvent(fs[0], events["M-106"]); ev == nil || !ok || ev.Type != EventScheduledOutage {
		t.Errorf("M-106: %+v explains=%v", ev, ok)
	}

	// M-109: el único evento es UNKNOWN: se relaciona, pero no explica.
	rs = sortReadings(readings["M-109"])
	b, _ = BuildBaseline(rs, MetricConsumption)
	fs = DetectPersistentShift(rs, b)
	if ev, ok := CorrelateEvent(fs[0], events["M-109"]); ev == nil || ok || ev.Type != EventUnknown {
		t.Errorf("M-109: %+v explains=%v", ev, ok)
	}
}
