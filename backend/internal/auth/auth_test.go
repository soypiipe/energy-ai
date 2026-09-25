package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const testSecret = "0123456789abcdef0123456789abcdef" // 32 caracteres

func newTestService(t *testing.T) *Service {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("clave-correcta"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewService("demo", string(hash), testSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewServiceValidation(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("x"), bcrypt.MinCost)
	cases := map[string][4]string{
		"usuario vacío":   {"", string(hash), testSecret, "1h"},
		"secreto corto":   {"demo", string(hash), "corto", "1h"},
		"hash inválido":   {"demo", "no-es-bcrypt", testSecret, "1h"},
		"ttl no positivo": {"demo", string(hash), testSecret, "0s"},
	}
	for name, c := range cases {
		ttl, _ := time.ParseDuration(c[3])
		if _, err := NewService(c[0], c[1], c[2], ttl); err == nil {
			t.Errorf("%s: debía fallar", name)
		}
	}
}

func TestLoginAndVerify(t *testing.T) {
	s := newTestService(t)
	tok, err := s.Login("demo", "clave-correcta")
	if err != nil {
		t.Fatal(err)
	}
	if tok.TokenType != "Bearer" || tok.ExpiresIn != 3600 || tok.AccessToken == "" {
		t.Errorf("token = %+v", tok)
	}
	if user, err := s.Verify(tok.AccessToken); err != nil || user != "demo" {
		t.Errorf("Verify = %q, %v", user, err)
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	s := newTestService(t)
	for _, c := range [][2]string{{"demo", "mala"}, {"otro", "clave-correcta"}, {"", ""}, {"demo", ""}} {
		if _, err := s.Login(c[0], c[1]); err != ErrInvalidCredentials {
			t.Errorf("Login(%q,%q) err = %v", c[0], c[1], err)
		}
	}
}

func TestVerifyRejectsBadTokens(t *testing.T) {
	s := newTestService(t)
	valid, _ := s.Login("demo", "clave-correcta")

	sign := func(method jwt.SigningMethod, key any, claims jwt.RegisteredClaims) string {
		out, err := jwt.NewWithClaims(method, claims).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	now := time.Now()
	good := jwt.RegisteredClaims{Issuer: issuer, Subject: "demo", ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}
	with := func(f func(*jwt.RegisteredClaims)) jwt.RegisteredClaims { c := good; f(&c); return c }

	tampered := valid.AccessToken[:len(valid.AccessToken)-2] + "xx"
	cases := map[string]string{
		"vacío":           "",
		"basura":          "no.es.un.jwt",
		"firma alterada":  tampered,
		"otra clave":      sign(jwt.SigningMethodHS256, []byte("otra-clave-otra-clave-otra-clave-!!"), good),
		"expirado":        sign(jwt.SigningMethodHS256, []byte(testSecret), with(func(c *jwt.RegisteredClaims) { c.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Minute)) })),
		"sin expiración":  sign(jwt.SigningMethodHS256, []byte(testSecret), with(func(c *jwt.RegisteredClaims) { c.ExpiresAt = nil })),
		"otro emisor":     sign(jwt.SigningMethodHS256, []byte(testSecret), with(func(c *jwt.RegisteredClaims) { c.Issuer = "otro" })),
		"otro usuario":    sign(jwt.SigningMethodHS256, []byte(testSecret), with(func(c *jwt.RegisteredClaims) { c.Subject = "admin" })),
		"algoritmo none":  sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, good),
		"algoritmo HS512": sign(jwt.SigningMethodHS512, []byte(testSecret), good),
	}
	for name, tok := range cases {
		if _, err := s.Verify(tok); err == nil {
			t.Errorf("%s: debía rechazarse", name)
		}
	}
}

func TestTokenExpiresWithTime(t *testing.T) {
	s := newTestService(t)
	tok, _ := s.Login("demo", "clave-correcta")
	s.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if _, err := s.Verify(tok.AccessToken); err == nil {
		t.Error("el token debía expirar tras el TTL")
	}
}

func post(h http.Handler, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.7:5555"
	h.ServeHTTP(rec, req)
	return rec
}

func loginMux(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	NewHandler(newTestService(t)).Register(mux)
	return mux
}

func TestLoginEndpoint(t *testing.T) {
	h := loginMux(t)

	rec := post(h, `{"username":"demo","password":"clave-correcta"}`)
	var tok Token
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &tok) != nil || tok.AccessToken == "" {
		t.Fatalf("login válido = %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("la respuesta con el token no debe cachearse")
	}

	// mismo mensaje y código para usuario o contraseña incorrectos
	a := post(h, `{"username":"demo","password":"mala"}`)
	b := post(h, `{"username":"noexiste","password":"mala"}`)
	if a.Code != 401 || b.Code != 401 || a.Body.String() != b.Body.String() {
		t.Errorf("no debe distinguirse usuario de contraseña: %d %q vs %d %q", a.Code, a.Body, b.Code, b.Body)
	}
	if strings.Contains(a.Body.String(), "clave-correcta") {
		t.Error("la respuesta no debe reflejar secretos")
	}

	for name, body := range map[string]string{
		"no es JSON": `nope`, "vacío": `{}`, "campo extra": `{"username":"demo","password":"x","admin":true}`, "sin contraseña": `{"username":"demo"}`,
	} {
		if rec := post(h, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s → %d, quería 400", name, rec.Code)
		}
	}
}

func TestLoginRateLimit(t *testing.T) {
	h := loginMux(t)
	var last int
	for i := 0; i < loginMaxAttempts+3; i++ {
		last = post(h, `{"username":"demo","password":"mala"}`).Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("tras muchos intentos → %d, quería 429", last)
	}
	// otra IP no se ve afectada
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"username":"demo","password":"clave-correcta"}`))
	req.RemoteAddr = "198.51.100.9:1234"
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("otra IP → %d", rec.Code)
	}
}

func TestLimiterWindowExpires(t *testing.T) {
	now := time.Now()
	l := newAttemptLimiter(2, time.Minute)
	l.now = func() time.Time { return now }
	if !l.allow("ip") || !l.allow("ip") || l.allow("ip") {
		t.Fatal("2 permitidos y el 3.º bloqueado")
	}
	now = now.Add(61 * time.Second)
	if !l.allow("ip") {
		t.Error("tras la ventana debe volver a permitirse")
	}
	if len(l.seen) != 1 {
		t.Errorf("el barrido debía dejar 1 entrada, hay %d", len(l.seen))
	}
}
