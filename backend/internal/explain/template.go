package explain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/soypiipe/energy-ai/backend/internal/analysis/engine"
)

// Template es el Explainer determinista: mismo input, mismo texto, sin red ni claves.
// Es la garantía de que el análisis siempre termina con explicaciones legibles.
type Template struct{}

func NewTemplate() *Template { return &Template{} }

func (Template) Explain(_ context.Context, a engine.Anomaly) (Explanation, error) {
	var e Explanation
	switch a.Type {
	case engine.AnomalyDataQuality:
		e = dataQuality(a)
	case engine.AnomalyFalsePos:
		e = falsePositive(a)
	case engine.AnomalyExplainable:
		e = explainable(a)
	default:
		e = realAnomaly(a)
	}
	return e, e.Validate()
}

func realAnomaly(a engine.Anomaly) Explanation {
	f := primary(a)
	var b strings.Builder
	b.WriteString(describe(f, a))
	if a.RelatedEvent != nil && a.RelatedEvent.Type == engine.EventUnknown {
		b.WriteString(" El único evento cercano es \"" + a.RelatedEvent.Description + "\", que no explica el cambio.")
	} else {
		b.WriteString(" No hay ningún evento operativo que lo explique.")
	}

	action := fmt.Sprintf("Inspeccionar en sitio el medidor %s y revisar la carga conectada (equipos nuevos, fallas o fugas).", a.MeterID)
	if a.Severity != engine.SeverityHigh {
		action = fmt.Sprintf("Programar la inspección del medidor %s y revisar la carga conectada (equipos nuevos, fallas o fugas).", a.MeterID)
	}
	if hasMetric(a, string(engine.MetricPowerFactor)) {
		action += " Verificar también el factor de potencia: si sigue bajo puede generar penalizaciones."
	}
	return Explanation{Reason: b.String(), RecommendedAction: action}
}

func explainable(a engine.Anomaly) Explanation {
	f := primary(a)
	reason := describe(f, a)
	action := "Confirmar con operaciones que el cambio es el previsto y, si es permanente, actualizar el consumo de referencia del medidor."
	if e := a.RelatedEvent; e != nil {
		reason += fmt.Sprintf(" Coincide con un evento reportado (%s, %s): \"%s\".", eventName(e.Type), fmtTime(e.Timestamp), e.Description)
		action = fmt.Sprintf("Confirmar con operaciones que \"%s\" justifica el nuevo nivel de consumo y, si es permanente, actualizar el consumo de referencia de %s.", e.Description, a.MeterID)
	}
	return Explanation{Reason: reason, RecommendedAction: action}
}

func falsePositive(a engine.Anomaly) Explanation {
	f := primary(a)
	reason := describe(f, a)
	action := "No requiere acción: la desviación estaba prevista. Se puede descartar."
	if e := a.RelatedEvent; e != nil {
		reason += fmt.Sprintf(" Coincide con un evento reportado (%s, %s): \"%s\".", eventName(e.Type), fmtTime(e.Timestamp), e.Description)
	}
	return Explanation{Reason: reason, RecommendedAction: action}
}

func dataQuality(a engine.Anomaly) Explanation {
	f := primary(a)
	var count float64
	var ratio *engine.Evidence
	for i, ev := range f.Evidence {
		switch ev.Metric {
		case "flagged_readings":
			count = ev.Observed
		case "kwh_vi_ratio":
			ratio = &f.Evidence[i]
		}
	}
	reason := fmt.Sprintf("Desde %s el medidor entrega lecturas físicamente incoherentes (%d lecturas marcadas).", fmtTime(f.Start), int(count))
	if ratio != nil {
		reason += fmt.Sprintf(" La razón entre el consumo y V·I·FP debería estar cerca de %s y llega a %s: el consumo no cuadra con voltaje, corriente y factor de potencia.", num(ratio.Baseline), num(ratio.Observed))
	}
	reason += " Sus datos no son confiables mientras no se corrija."
	return Explanation{
		Reason:            reason,
		RecommendedAction: fmt.Sprintf("Revisar los sensores de voltaje y corriente y la comunicación del medidor %s, y no usar sus lecturas para decisiones hasta corregirlo.", a.MeterID),
	}
}

// describe cuenta qué pasó según el hallazgo principal, con los números de la evidencia.
func describe(f engine.Finding, a engine.Anomaly) string {
	var b strings.Builder
	consumption := evidenceFor(f, engine.MetricConsumption)
	switch f.Kind {
	case engine.FindingTransientDeviation:
		b.WriteString(fmt.Sprintf("El consumo %s durante %d horas desde %s", direction(consumption), hours(f), fmtTime(f.Start)))
		if consumption != nil {
			b.WriteString(fmt.Sprintf(" (%s, de %s a %s kWh por hora en promedio)", pct(consumption.ChangePct), num(consumption.Baseline), num(consumption.Observed)))
		}
		b.WriteString(" y luego volvió a lo normal.")
	default:
		if consumption != nil {
			b.WriteString(fmt.Sprintf("El consumo %s %s (de %s a %s kWh por hora en promedio) desde %s", direction(consumption), pctAbs(consumption.ChangePct), num(consumption.Baseline), num(consumption.Observed), fmtTime(f.Start)))
		} else {
			b.WriteString(fmt.Sprintf("Se detectó un cambio eléctrico sostenido desde %s", fmtTime(f.Start)))
		}
		if f.End.IsZero() {
			b.WriteString(" y sigue así.")
		} else {
			b.WriteString(fmt.Sprintf(" hasta %s.", fmtTime(f.End)))
		}
	}
	if extra := electricalSentence(a); extra != "" {
		b.WriteString(" " + extra)
	}
	return b.String()
}

// electricalSentence resume los cambios eléctricos (corriente, FP, voltaje) de la anomalía.
func electricalSentence(a engine.Anomaly) string {
	var parts []string
	for _, f := range a.Findings {
		if f.Kind != engine.FindingElectricalChange {
			continue
		}
		for _, ev := range f.Evidence {
			parts = append(parts, fmt.Sprintf("%s %s %s (de %s a %s)", metricName(ev.Metric), verb(ev.ChangePct), pctAbs(ev.ChangePct), num(ev.Baseline), num(ev.Observed)))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "Además: " + strings.Join(parts, "; ") + "."
}

// primary elige el hallazgo que define la anomalía (mismo criterio que el motor: el de inicio más temprano
// entre los de consumo; si no hay, el primero).
func primary(a engine.Anomaly) engine.Finding {
	for _, kind := range []engine.FindingKind{engine.FindingDataQuality, engine.FindingPersistentShift, engine.FindingTransientDeviation, engine.FindingElectricalChange} {
		for _, f := range a.Findings {
			if f.Kind == kind {
				return f
			}
		}
	}
	return engine.Finding{}
}

func evidenceFor(f engine.Finding, m engine.Metric) *engine.Evidence {
	for i := range f.Evidence {
		if f.Evidence[i].Metric == string(m) {
			return &f.Evidence[i]
		}
	}
	return nil
}

func hasMetric(a engine.Anomaly, metric string) bool {
	for _, f := range a.Findings {
		for _, ev := range f.Evidence {
			if ev.Metric == metric {
				return true
			}
		}
	}
	return false
}

func hours(f engine.Finding) int {
	if f.End.IsZero() {
		return 0
	}
	return int(f.End.Sub(f.Start).Hours()) + 1
}

func direction(ev *engine.Evidence) string {
	if ev != nil && ev.ChangePct < 0 {
		return "cayó"
	}
	return "subió"
}

func verb(change float64) string {
	if change < 0 {
		return "baja"
	}
	return "sube"
}

func metricName(m string) string {
	switch m {
	case "consumption_kwh":
		return "el consumo"
	case "power_factor":
		return "el factor de potencia"
	case "current_a":
		return "la corriente"
	case "voltage_v":
		return "el voltaje"
	}
	return m
}

func eventName(t engine.EventType) string {
	switch t {
	case engine.EventOperationalChange:
		return "cambio operativo"
	case engine.EventScheduledOutage:
		return "apagado programado"
	case engine.EventDataQuality:
		return "problema de datos"
	}
	return "evento sin clasificar"
}

var monthsES = [...]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}

// fmtTime: "12 sep 14:00". Hora local de planta, sin zona.
func fmtTime(t time.Time) string {
	return fmt.Sprintf("%d %s %02d:%02d", t.Day(), monthsES[t.Month()-1], t.Hour(), t.Minute())
}

// num formatea con coma decimal y hasta 2 decimales, sin ceros sobrantes.
func num(v float64) string {
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
	return strings.Replace(s, ".", ",", 1)
}

func pct(v float64) string {
	sign := ""
	if v > 0 {
		sign = "+"
	}
	return sign + num(v) + "%"
}

func pctAbs(v float64) string {
	if v < 0 {
		v = -v
	}
	return num(v) + "%"
}
