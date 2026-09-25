package config

import "testing"

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for _, k := range []string{"DATABASE_URL", "LLM_BASE_URL", "LLM_MODEL", "LLM_API_KEY"} {
		t.Setenv(k, "")
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
