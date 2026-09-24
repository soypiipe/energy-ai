package seed

import (
	"strings"
	"testing"
)

func TestParseReadings(t *testing.T) {
	csv := "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status\n" +
		"M-101,2026-09-01 00:00:00,23.5,221.9,101.28,0.954,OK\n"

	got, err := ParseReadings(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(got) != 1 || got[0].MeterID != "M-101" || got[0].ConsumptionKWh != 23.5 || got[0].PowerFactor != 0.954 {
		t.Fatalf("lectura mal parseada: %+v", got)
	}
}

func TestParseReadingsRejectsBadInput(t *testing.T) {
	header := "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status\n"
	cases := map[string]string{
		"encabezado distinto": "id,ts,kwh,v,a,pf,status\nM-101,2026-09-01 00:00:00,1,1,1,1,OK\n",
		"número inválido":     header + "M-101,2026-09-01 00:00:00,abc,221,100,0.9,OK\n",
		"fecha inválida":      header + "M-101,01/09/2026,23,221,100,0.9,OK\n",
		"columnas faltantes":  header + "M-101,2026-09-01 00:00:00,23\n",
		"sin filas":           header,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseReadings(strings.NewReader(input)); err == nil {
				t.Fatal("se esperaba error")
			}
		})
	}
}

func TestParseEvents(t *testing.T) {
	csv := "meter_id,event_timestamp,event_type,description\n" +
		"M-106,2026-09-08 00:00,SCHEDULED_OUTAGE,Scheduled maintenance outage for 12 hours\n"

	got, err := ParseEvents(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got[0].Type != "SCHEDULED_OUTAGE" || got[0].Timestamp.Hour() != 0 || got[0].Timestamp.Day() != 8 {
		t.Fatalf("evento mal parseado: %+v", got[0])
	}
}
