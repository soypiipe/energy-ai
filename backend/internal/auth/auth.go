// Package auth implementa el login del usuario demo (bcrypt) y los tokens JWT que protegen la API.
//
// Un solo usuario configurado por variables de entorno (docs/DESIGN.md ADR 10): cumple el flujo de
// login con buenas prácticas sin montar un sistema de usuarios.
package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const issuer = "energy-ai"

var ErrInvalidCredentials = errors.New("credenciales inválidas")

// Service valida credenciales y emite/verifica tokens.
type Service struct {
	username     string
	passwordHash []byte
	secret       []byte
	ttl          time.Duration
	now          func() time.Time // inyectable en tests
}

// MinSecretLen: HS256 con menos de 32 bytes de clave es débil.
const MinSecretLen = 32

func NewService(username, passwordHash, secret string, ttl time.Duration) (*Service, error) {
	switch {
	case username == "":
		return nil, errors.New("usuario vacío")
	case len(secret) < MinSecretLen:
		return nil, fmt.Errorf("JWT_SECRET debe tener al menos %d caracteres", MinSecretLen)
	case ttl <= 0:
		return nil, errors.New("la duración del token debe ser positiva")
	}
	// Se valida el hash al arrancar: mejor fallar aquí que en el primer login.
	if _, err := bcrypt.Cost([]byte(passwordHash)); err != nil {
		return nil, errors.New("AUTH_PASSWORD_HASH no es un hash bcrypt válido")
	}
	return &Service{username: username, passwordHash: []byte(passwordHash), secret: []byte(secret), ttl: ttl, now: time.Now}, nil
}

// Token es la respuesta de un login correcto.
type Token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"` // segundos
}

// Login verifica usuario y contraseña y emite un JWT con expiración.
//
// Siempre se ejecuta bcrypt, aunque el usuario no coincida: así el tiempo de respuesta no revela si
// el usuario existe. El usuario se compara en tiempo constante.
func (s *Service) Login(username, password string) (Token, error) {
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(s.username)) == 1
	passErr := bcrypt.CompareHashAndPassword(s.passwordHash, []byte(password))
	if !userOK || passErr != nil {
		return Token{}, ErrInvalidCredentials
	}

	now := s.now()
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   s.username,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return Token{}, fmt.Errorf("firmar token: %w", err)
	}
	return Token{AccessToken: signed, TokenType: "Bearer", ExpiresIn: int(s.ttl.Seconds())}, nil
}

// Verify valida firma, algoritmo (solo HS256), emisor y expiración, y devuelve el usuario del token.
func (s *Service) Verify(tokenString string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), // rechaza "none" y otros algoritmos
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		return "", fmt.Errorf("token inválido: %w", err)
	}
	if claims.Subject != s.username {
		return "", errors.New("token inválido: usuario desconocido")
	}
	return claims.Subject, nil
}
