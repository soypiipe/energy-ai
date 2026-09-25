// Package engine es el motor de análisis: funciones puras sobre lecturas y eventos.
//
// No conoce la base de datos ni HTTP. Recibe datos ya cargados y devuelve anomalías
// clasificadas, así que todo se prueba con tests unitarios y el resultado es determinista.
//
// Los timestamps representan la hora local de la planta (ver docs/DESIGN.md): se manejan
// como time.Time en UTC sin convertir zonas, igual que el TIMESTAMP sin zona de PostgreSQL.
package engine

import "time"

// Reading es una lectura horaria de un medidor.
type Reading struct {
	Timestamp      time.Time
	ConsumptionKWh float64
	VoltageV       float64
	CurrentA       float64
	PowerFactor    float64
}

// EventType es el tipo de evento operativo reportado para un medidor.
type EventType string

const (
	EventOperationalChange EventType = "OPERATIONAL_CHANGE"
	EventScheduledOutage   EventType = "SCHEDULED_OUTAGE"
	EventDataQuality       EventType = "DATA_QUALITY"
	EventUnknown           EventType = "UNKNOWN"
)

type Event struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        EventType `json:"type"`
	Description string    `json:"description"`
}

// FindingKind es lo que detectó un detector, antes de clasificar.
type FindingKind string

const (
	FindingPersistentShift    FindingKind = "PERSISTENT_SHIFT"    // cambio de nivel sostenido (≥24 h)
	FindingTransientDeviation FindingKind = "TRANSIENT_DEVIATION" // se sale de lo normal y vuelve
	FindingElectricalChange   FindingKind = "ELECTRICAL_CHANGE"   // FP, corriente o voltaje
	FindingDataQuality        FindingKind = "DATA_QUALITY"        // lecturas físicamente incoherentes
)

// Evidence es una métrica comparada contra su baseline. Es lo que se muestra en la
// investigación y lo único que recibe el LLM para redactar: no inventa números.
type Evidence struct {
	Metric    string  `json:"metric"` // consumption_kwh, power_factor, current_a, voltage_v, kwh_vi_ratio
	Baseline  float64 `json:"baseline"`
	Observed  float64 `json:"observed"`
	ChangePct float64 `json:"change_pct"` // (observed / baseline - 1) * 100
}

// Finding es un hallazgo de un detector sobre un medidor.
type Finding struct {
	Kind     FindingKind `json:"kind"`
	Start    time.Time   `json:"start"`
	End      time.Time   `json:"end"` // cero si sigue activo al final de los datos
	Evidence []Evidence  `json:"evidence"`
}

// AnomalyType es la clasificación final. Los valores coinciden con el CHECK de la tabla anomalies.
type AnomalyType string

const (
	AnomalyReal        AnomalyType = "REAL_ANOMALY"
	AnomalyExplainable AnomalyType = "EXPLAINABLE_ANOMALY"
	AnomalyFalsePos    AnomalyType = "FALSE_POSITIVE"
	AnomalyDataQuality AnomalyType = "DATA_QUALITY"
)

type Severity string

const (
	SeverityLow    Severity = "LOW"
	SeverityMedium Severity = "MEDIUM"
	SeverityHigh   Severity = "HIGH"
)

// Rank ordena severidades (mayor = más grave); 0 si el valor no es válido.
func (s Severity) Rank() int {
	switch s {
	case SeverityLow:
		return 1
	case SeverityMedium:
		return 2
	case SeverityHigh:
		return 3
	}
	return 0
}

// Anomaly es el resultado del motor para un medidor. El texto para el operador
// (reason, recommended_action) no vive aquí: lo produce el Explainer a partir de esto.
type Anomaly struct {
	MeterID       string      `json:"meter_id"`
	DetectedAt    time.Time   `json:"detected_at"` // inicio del primer hallazgo
	Type          AnomalyType `json:"type"`
	Severity      Severity    `json:"severity"`
	Confidence    float64     `json:"confidence"`     // 0..1
	PriorityScore float64     `json:"priority_score"` // mayor = revisar primero
	Findings      []Finding   `json:"findings"`
	RelatedEvent  *Event      `json:"related_event"` // nil si ningún evento cae en la ventana
}
