package apidocs

import (
	"strings"
	"testing"
)

// Todo código de error que emite la API debe estar documentado en la especificación.
// Si se agrega uno nuevo en un handler, este test recuerda actualizar openapi.yaml.
func TestSpecDocumentsEveryErrorCode(t *testing.T) {
	codes := []string{
		"INVALID_BODY", "INVALID_CREDENTIALS", "TOO_MANY_ATTEMPTS", "UNAUTHORIZED",
		"INVALID_METER_ID", "METER_NOT_FOUND", "INVALID_FROM", "INVALID_TO", "INVALID_RANGE",
		"INVALID_ID", "ANALYSIS_NOT_FOUND", "ANOMALY_NOT_FOUND",
		"INVALID_TYPE", "INVALID_SEVERITY", "INVALID_STATUS", "DB_UNAVAILABLE", "INTERNAL",
	}
	for _, c := range codes {
		if !strings.Contains(string(spec), "code: "+c) {
			t.Errorf("el código %s no tiene ningún ejemplo en openapi.yaml", c)
		}
	}
}

func TestSpecDocumentsEveryRoute(t *testing.T) {
	routes := []string{
		"/health:", "/auth/login:", "/meters:", "/meters/{meterId}:", "/meters/{meterId}/readings:",
		"/ai/analyze:", "/ai/analysis/{id}:", "/anomalies:", "/anomalies/{id}:", "/dashboard/summary:",
	}
	for _, r := range routes {
		if !strings.Contains(string(spec), "\n  "+r) {
			t.Errorf("la ruta %s no está documentada", r)
		}
	}
}
