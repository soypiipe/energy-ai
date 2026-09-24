// Package config carga la configuración desde variables de entorno.
// Es la única fuente de configuración: nada de secretos en el código.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port           string
	DatabaseURL    string
	DataDir        string   // carpeta con readings.csv y events.csv para el seed inicial
	AllowedOrigins []string // CORS: lista blanca de orígenes del frontend
}

// Load lee y valida la configuración. Falla al arrancar si falta algo obligatorio
// (mejor fallar temprano que en la primera petición).
func Load() (Config, error) {
	cfg := Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		DataDir:        getEnv("DATA_DIR", "../data"),
		AllowedOrigins: splitCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:5173")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL es obligatoria")
	}
	if len(cfg.AllowedOrigins) == 0 {
		return Config{}, fmt.Errorf("ALLOWED_ORIGINS no puede estar vacía")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
