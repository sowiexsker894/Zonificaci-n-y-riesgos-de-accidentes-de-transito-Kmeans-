package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// JWT HS256 mínimo con la biblioteca estándar.
var b64 = base64.RawURLEncoding

func firmar(secret, data string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(data))
	return b64.EncodeToString(m.Sum(nil))
}

func NuevoToken(secret, usuario, rol string, dur time.Duration) string {
	h := b64.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	p, _ := json.Marshal(map[string]any{"sub": usuario, "rol": rol, "exp": time.Now().Add(dur).Unix()})
	body := h + "." + b64.EncodeToString(p)
	return body + "." + firmar(secret, body)
}

func ValidarToken(secret, tok string) (usuario string, err error) {
	p := strings.Split(tok, ".")
	if len(p) != 3 || !hmac.Equal([]byte(firmar(secret, p[0]+"."+p[1])), []byte(p[2])) {
		return "", errors.New("token inválido")
	}
	raw, err := b64.DecodeString(p[1])
	if err != nil {
		return "", err
	}
	var c struct {
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}
	if json.Unmarshal(raw, &c) != nil || time.Now().Unix() > c.Exp {
		return "", errors.New("token expirado")
	}
	return c.Sub, nil
}
