package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"siniestros/internal/api"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	db, err := sql.Open("postgres", env("DATABASE_URL", "postgres://siniestros:siniestros@localhost:5432/siniestros?sslmode=disable"))
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(20)
	for i := 0; ; i++ { // espera a Postgres (útil en docker compose)
		if err = db.Ping(); err == nil {
			break
		}
		if i > 30 {
			log.Fatal("postgres no responde: ", err)
		}
		time.Sleep(time.Second)
	}
	dataDir := env("DATA_DIR", "data")
	if err := api.Migrar(db, dataDir); err != nil {
		log.Fatal("migración: ", err)
	}
	rd := redis.NewClient(&redis.Options{Addr: env("REDIS_ADDR", "localhost:6379")})
	s, err := api.Nuevo(api.Config{DB: db, Redis: rd, Nodos: strings.Split(env("NODOS", "localhost:9001,localhost:9002,localhost:9003"), ","),
		Secret: env("JWT_SECRET", "cambia-esto-en-produccion"), DataDir: dataDir})
	if err != nil {
		log.Fatal(err)
	}
	addr := ":" + env("PORT", "8080")
	log.Printf("API lista en %s  (Swagger: /swagger)", addr)
	log.Fatal(http.ListenAndServe(addr, s.Handler()))
}
