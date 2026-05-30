package adminauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid admin credentials")
var ErrInvalidToken = errors.New("invalid admin token")

type Config struct {
	Username          string
	PasswordHash      string
	TokenSecret       string
	AccessTokenExpiry time.Duration
}

type Claims struct {
	Username  string `json:"username"`
	ExpiresAt int64  `json:"expires_at"`
}

type Service struct {
	cfg Config
	now func() time.Time
}

func NewService(cfg Config) *Service {
	return &Service{cfg: cfg, now: time.Now}
}

func (s *Service) Login(username, password string) (string, error) {
	if username != s.cfg.Username {
		return "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(s.cfg.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}
	claims := Claims{
		Username:  username,
		ExpiresAt: s.now().Add(s.cfg.AccessTokenExpiry).Unix(),
	}
	return s.sign(claims)
}

func (s *Service) Validate(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Claims{}, ErrInvalidToken
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	expected := s.signature(parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(body, &claims); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if claims.ExpiresAt <= s.now().Unix() {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

func (s *Service) sign(claims Claims) (string, error) {
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(body)
	return payload + "." + s.signature(payload), nil
}

func (s *Service) signature(payload string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.TokenSecret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
