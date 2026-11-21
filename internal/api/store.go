package api

import (
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

type Zona struct {
	Ranking       int     `json:"ranking"`
	Cluster       int     `json:"cluster"`
	Lat           float64 `json:"lat"`
	Lon           float64 `json:"lon"`
	Siniestros    int     `json:"siniestros"`
	Fallecidos    int     `json:"fallecidos"`
	PctFallecidos float64 `json:"pct_fallecidos_nacional"`
	RadioKm       float64 `json:"radio_medio_km"`
	Departamento  string  `json:"departamento"`
	Provincia     string  `json:"provincia"`
	Distrito      string  `json:"distrito"`
	HoraPico      int     `json:"hora_pico"`
	FranjaPico    string  `json:"franja_pico_3h"`
	Clase         string  `json:"clase_principal"`
	Causa         string  `json:"causa_principal"`
	PorHora       []int   `json:"siniestros_por_hora,omitempty"`
}

const esquema = `
CREATE TABLE IF NOT EXISTS usuarios(usuario TEXT PRIMARY KEY, clave_sha256 TEXT NOT NULL, rol TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS zonas(
  ranking INT PRIMARY KEY, cluster INT, lat DOUBLE PRECISION, lon DOUBLE PRECISION, siniestros INT, fallecidos INT,
  pct_fallecidos DOUBLE PRECISION, radio_km DOUBLE PRECISION, departamento TEXT, provincia TEXT, distrito TEXT,
  hora_pico INT, franja_pico TEXT, clase TEXT, causa TEXT, por_hora JSONB);
CREATE TABLE IF NOT EXISTS siniestros(
  id SERIAL PRIMARY KEY, lat DOUBLE PRECISION NOT NULL, lon DOUBLE PRECISION NOT NULL, hora INT NOT NULL,
  fallecidos INT NOT NULL DEFAULT 0, departamento TEXT, zona_ranking INT REFERENCES zonas(ranking),
  creado TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE INDEX IF NOT EXISTS ix_sin_depto ON siniestros(departamento);
CREATE TABLE IF NOT EXISTS entrenamientos(
  id SERIAL PRIMARY KEY, k INT, iteraciones INT, sse DOUBLE PRECISION, convergio BOOL, duracion_ms DOUBLE PRECISION,
  nodos INT, centroides JSONB, creado TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS consultas(
  id BIGSERIAL PRIMARY KEY, lat DOUBLE PRECISION, lon DOUBLE PRECISION, hora INT, zona_ranking INT,
  riesgo DOUBLE PRECISION, latencia_ms DOUBLE PRECISION, creado TIMESTAMPTZ NOT NULL DEFAULT now());
`

func sha(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// Migrar crea el esquema y siembra datos si las tablas están vacías (idempotente).
func Migrar(db *sql.DB, dataDir string) error {
	if _, err := db.Exec(esquema); err != nil {
		return err
	}
	db.Exec(`INSERT INTO usuarios VALUES('admin',$1,'admin'),('analista',$2,'lector') ON CONFLICT DO NOTHING`, sha("admin123"), sha("analista123"))

	var n int
	db.QueryRow(`SELECT count(*) FROM zonas`).Scan(&n)
	if n == 0 {
		raw, err := os.ReadFile(filepath.Join(dataDir, "kmeans_clusters.json"))
		if err != nil {
			return err
		}
		var zs []Zona
		if err := json.Unmarshal(raw, &zs); err != nil {
			return err
		}
		for _, z := range zs {
			h, _ := json.Marshal(z.PorHora)
			if _, err := db.Exec(`INSERT INTO zonas VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
				z.Ranking, z.Cluster, z.Lat, z.Lon, z.Siniestros, z.Fallecidos, z.PctFallecidos, z.RadioKm, z.Departamento,
				z.Provincia, z.Distrito, z.HoraPico, z.FranjaPico, z.Clase, z.Causa, h); err != nil {
				return err
			}
		}
		log.Printf("seed: %d zonas", len(zs))
	}
	db.QueryRow(`SELECT count(*) FROM siniestros`).Scan(&n)
	if n == 0 {
		f, err := os.Open(filepath.Join(dataDir, "kmeans_puntos_reales.csv"))
		if err != nil {
			return err
		}
		defer f.Close()
		cl2r := map[int]int{}
		rows, _ := db.Query(`SELECT cluster, ranking FROM zonas`)
		for rows.Next() {
			var c, r int
			rows.Scan(&c, &r)
			cl2r[c] = r
		}
		rows.Close()
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		st, err := tx.Prepare(`INSERT INTO siniestros(lat,lon,hora,fallecidos,departamento,zona_ranking) VALUES($1,$2,$3,$4,$5,$6)`)
		if err != nil {
			return err
		}
		r := csv.NewReader(f)
		r.Read()
		cnt := 0
		for {
			rec, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			la, _ := strconv.ParseFloat(rec[0], 64)
			lo, _ := strconv.ParseFloat(rec[1], 64)
			cl, _ := strconv.Atoi(rec[2])
			h, _ := strconv.Atoi(rec[3])
			fa, _ := strconv.Atoi(rec[4])
			var zr any
			if v, ok := cl2r[cl]; ok {
				zr = v
			}
			if _, err := st.Exec(la, lo, h, fa, rec[5], zr); err != nil {
				tx.Rollback()
				return err
			}
			cnt++
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		log.Printf("seed: %d siniestros reales", cnt)
	}
	return nil
}

func CargarZonas(db *sql.DB) ([]Zona, error) {
	rows, err := db.Query(`SELECT ranking,cluster,lat,lon,siniestros,fallecidos,pct_fallecidos,radio_km,departamento,provincia,distrito,hora_pico,franja_pico,clase,causa,por_hora FROM zonas ORDER BY ranking`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Zona
	for rows.Next() {
		var z Zona
		var h []byte
		if err := rows.Scan(&z.Ranking, &z.Cluster, &z.Lat, &z.Lon, &z.Siniestros, &z.Fallecidos, &z.PctFallecidos, &z.RadioKm,
			&z.Departamento, &z.Provincia, &z.Distrito, &z.HoraPico, &z.FranjaPico, &z.Clase, &z.Causa, &h); err != nil {
			return nil, err
		}
		json.Unmarshal(h, &z.PorHora)
		out = append(out, z)
	}
	return out, nil
}
