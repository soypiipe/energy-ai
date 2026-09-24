// Package meter expone la lectura de medidores, sus lecturas y eventos.
//
// No tiene capa de servicio a propósito: aquí no hay reglas de negocio que orquestar,
// solo consultas. Agregar un "service" que solo reenvía llamadas sería una abstracción vacía.
package meter

import (
	"math"

	"github.com/soypiipe/energy-ai/backend/internal/httpx"
)

// BaselineDays: días iniciales del dataset usados como comportamiento esperado.
const BaselineDays = 7

// Estados operativos que ve el usuario en la tabla de medidores.
const (
	StatusOK       = "OK"
	StatusAlert    = "ALERT"
	StatusCritical = "CRITICAL"
)

type Summary struct {
	MeterID       string          `json:"meter_id"`
	Name          string          `json:"name"`
	Location      *string         `json:"location"`
	TotalKWh      float64         `json:"total_kwh"`     // todo el periodo
	CurrentKWh    float64         `json:"current_kwh"`   // últimas 24 h del dataset
	BaselineKWh   float64         `json:"baseline_kwh"`  // promedio diario de los primeros BaselineDays días
	VariationPct  float64         `json:"variation_pct"` // (actual / baseline - 1) * 100
	Status        string          `json:"status"`        // OK | ALERT | CRITICAL (según el último análisis)
	AnomalyType   *string         `json:"anomaly_type"`  // null si no hay anomalía
	Severity      *string         `json:"severity"`      // null si no hay anomalía
	LastReadingAt httpx.LocalTime `json:"last_reading_at"`
}

type Reading struct {
	Timestamp      httpx.LocalTime `json:"timestamp"`
	ConsumptionKWh float64         `json:"consumption_kwh"`
	VoltageV       float64         `json:"voltage_v"`
	CurrentA       float64         `json:"current_a"`
	PowerFactor    float64         `json:"power_factor"`
	Status         string          `json:"status"`
}

type Event struct {
	Timestamp   httpx.LocalTime `json:"timestamp"`
	Type        string          `json:"type"`
	Description string          `json:"description"`
}

type Detail struct {
	Summary
	Events []Event `json:"events"`
}

// VariationPct calcula la variación porcentual contra el baseline, redondeada a 1 decimal.
func VariationPct(current, baseline float64) float64 {
	if baseline <= 0 {
		return 0
	}
	return round((current/baseline-1)*100, 1)
}

// StatusFor traduce la anomalía principal de un medidor al estado operativo:
// una anomalía real y alta es crítica; cualquier otra que requiera atención es alerta;
// un falso positivo (o nada) es OK.
func StatusFor(anomalyType, severity *string) string {
	if anomalyType == nil || *anomalyType == "FALSE_POSITIVE" {
		return StatusOK
	}
	if *anomalyType == "REAL_ANOMALY" && severity != nil && *severity == "HIGH" {
		return StatusCritical
	}
	return StatusAlert
}

func round(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}
