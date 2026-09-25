// Package config carga la configuración desde variables de entorno.
// Es la única fuente de configuración: nada de secretos en el código.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Port           string
	DatabaseURL    string
	DataDir        string   // carpeta con readings.csv y events.csv para el seed inicial
	AllowedOrigins []string // CORS: lista blanca de orígenes del frontend

	// LLM compatible con la API de OpenAI. Es opcional: sin clave, las explicaciones salen de la plantilla.
	LLMBaseURL string
	LLMModel   string
	LLMAPIKey  string
}

// LLMEnabled dice si hay configuración suficiente para usar el LLM.
func (c Config) LLMEnabled() bool {
	return c.LLMAPIKey != "" && c.LLMModel != "" && c.LLMBaseURL != ""
}

// Load lee y valida la configuración. Falla al arrancar si falta algo obligatorio
// (mejor fallar temprano que en la primera petición).
func Load() (Config, error) {
	cfg := Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		DataDir:        getEnv("DATA_DIR", "../data"),
		AllowedOrigins: splitCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:5173")),
		LLMBaseURL:     getEnv("LLM_BASE_URL", "https://openrouter.ai/api/v1"),
		LLMModel:       os.Getenv("LLM_MODEL"),
		LLMAPIKey:      os.Getenv("LLM_API_KEY"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL es obligatoria")
	}
	if len(cfg.AllowedOrigins) == 0 {
		return Config{}, fmt.Errorf("ALLOWED_ORIGINS no puede estar vacía")
	}
	if cfg.LLMAPIKey != "" {
		if u, err := url.Parse(cfg.LLMBaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return Config{}, errors.New("LLM_BASE_URL debe ser una URL http(s) válida")
		}
		if cfg.LLMModel == "" {
			return Config{}, errors.New("LLM_MODEL es obligatoria cuando LLM_API_KEY está definida")
		}
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
