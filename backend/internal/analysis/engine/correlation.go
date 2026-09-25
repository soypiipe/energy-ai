package engine

import "time"

// eventWindow: un evento se relaciona con un hallazgo si ocurre a ±2 h del inicio del hallazgo.
const eventWindow = 2 * time.Hour

// explains dice si un tipo de evento justifica un cambio en el consumo.
// UNKNOWN ("No operational event reported") no explica nada; DATA_QUALITY describe un problema del
// sensor, no un cambio real de operación, así que tampoco.
func (t EventType) explains() bool {
	return t == EventOperationalChange || t == EventScheduledOutage
}

// CorrelateEvent busca el evento del medidor más cercano al inicio del hallazgo dentro de ±2 h.
//
// Devuelve el evento relacionado (nil si no hay ninguno en la ventana) y si ese evento explica el
// hallazgo. Si hay varios, gana uno que explique; a igualdad, el más cercano en el tiempo.
// Un evento UNKNOWN se devuelve como relacionado pero con explains=false: el operador ve que se buscó
// una causa y no había ninguna, que es justamente lo que hace sospechosa a una anomalía.
func CorrelateEvent(f Finding, events []Event) (related *Event, explains bool) {
	for i := range events {
		e := &events[i]
		if abs64(e.Timestamp.Sub(f.Start)) > eventWindow {
			continue
		}
		if related == nil || betterEvent(e, related, f.Start) {
			related = e
		}
	}
	if related == nil {
		return nil, false
	}
	return related, related.Type.explains()
}

func betterEvent(a, b *Event, at time.Time) bool {
	if a.Type.explains() != b.Type.explains() {
		return a.Type.explains()
	}
	return abs64(a.Timestamp.Sub(at)) < abs64(b.Timestamp.Sub(at))
}

func abs64(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
