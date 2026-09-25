package config

import "testing"

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for _, k := range []string{"DATABASE_URL", "LLM_BASE_URL", "LLM_MODEL", "LLM_API_KEY", "AUTH_USER", "AUTH_PASSWORD_HASH", "JWT_SECRET", "JWT_TTL"} {
		t.Setenv(k, "")
	}
	if _, ok := kv["DATABASE_URL"]; ok {
		t.Setenv("AUTH_PASSWORD_HASH", "hash")
		t.Setenv("JWT_SECRET", "secreto")
	}
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func TestLLMConfig(t *testing.T) {
	setEnv(t, map[string]string{"DATABASE_URL": "postgres://x"})
	cfg, err := Load()
	if err != nil || cfg.LLMEnabled() {
		t.Fatalf("sin clave el LLM debe estar apagado: %+v %v", cfg, err)
	}

	setEnv(t, map[string]string{"DATABASE_URL": "postgres://x", "LLM_API_KEY": "k", "LLM_MODEL": "m"})
	if cfg, err = Load(); err != nil || !cfg.LLMEnabled() || cfg.LLMBaseURL == "" {
		t.Errorf("con clave y modelo debe estar activo: %+v %v", cfg, err)
	}

	setEnv(t, map[string]string{"DATABASE_URL": "postgres://x", "LLM_API_KEY": "k"})
	if _, err = Load(); err == nil {
		t.Error("clave sin modelo debe fallar al arrancar")
	}
	setEnv(t, map[string]string{"DATABASE_URL": "postgres://x", "LLM_API_KEY": "k", "LLM_MODEL": "m", "LLM_BASE_URL": "ftp://raro"})
	if _, err = Load(); err == nil {
		t.Error("URL no http(s) debe fallar")
	}
}

func TestAuthConfigRequired(t *testing.T) {
	setEnv(t, map[string]string{"DATABASE_URL": "postgres://x"})
	cfg, err := Load()
	if err != nil || cfg.AuthUser != "demo" || cfg.JWTTTL.Hours() != 8 {
		t.Fatalf("valores por defecto: %+v %v", cfg, err)
	}

	setEnv(t, map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": ""})
	if _, err := Load(); err == nil {
		t.Error("sin JWT_SECRET debe fallar")
	}
	setEnv(t, map[string]string{"DATABASE_URL": "postgres://x", "AUTH_PASSWORD_HASH": ""})
	if _, err := Load(); err == nil {
		t.Error("sin AUTH_PASSWORD_HASH debe fallar")
	}
	setEnv(t, map[string]string{"DATABASE_URL": "postgres://x", "JWT_TTL": "mucho"})
	if _, err := Load(); err == nil {
		t.Error("JWT_TTL inválido debe fallar")
	}
}
