// Nodo de cómputo ML: sirve por TCP la fase MAP de K-Means sobre su partición.
package main

import (
	"log"
	"os"
	"strconv"

	"siniestros/internal/red"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	id, _ := strconv.Atoi(env("NODO_ID", "0"))
	total, _ := strconv.Atoi(env("NODO_TOTAL", "1"))
	pts, err := red.CargarPuntos(env("DATA_FILE", "data/kmeans_puntos_reales.csv"))
	if err != nil {
		log.Fatal(err)
	}
	a, b := id*len(pts)/total, (id+1)*len(pts)/total
	part := pts[a:b]
	addr := ":" + env("NODO_PORT", "9000")
	log.Printf("nodo %d/%d: partición [%d,%d) = %d puntos, escuchando en %s", id, total, a, b, len(part), addr)
	log.Fatal(red.Servir(addr, id, part))
}
