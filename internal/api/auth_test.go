package api

import (
	"testing"
	"time"
)

func TestJWT(t *testing.T) {
	tok := NuevoToken("s", "admin", "admin", time.Minute)
	if u, err := ValidarToken("s", tok); err != nil || u != "admin" {
		t.Fatalf("token válido rechazado: %v", err)
	}
	if _, err := ValidarToken("otro", tok); err == nil {
		t.Fatal("firma incorrecta aceptada")
	}
	if _, err := ValidarToken("s", NuevoToken("s", "a", "r", -time.Minute)); err == nil {
		t.Fatal("token expirado aceptado")
	}
}
