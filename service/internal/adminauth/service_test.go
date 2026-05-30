package adminauth

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestLoginAndValidate(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(Config{
		Username:          "admin",
		PasswordHash:      string(hash),
		TokenSecret:       "test-secret",
		AccessTokenExpiry: time.Hour,
	})
	token, err := svc.Login("admin", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	claims, err := svc.Validate(token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.Username != "admin" {
		t.Fatalf("username = %q", claims.Username)
	}
}

func TestLoginRejectsBadPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(Config{
		Username:          "admin",
		PasswordHash:      string(hash),
		TokenSecret:       "test-secret",
		AccessTokenExpiry: time.Hour,
	})
	if _, err := svc.Login("admin", "wrong"); err == nil {
		t.Fatal("expected bad password error")
	}
}
