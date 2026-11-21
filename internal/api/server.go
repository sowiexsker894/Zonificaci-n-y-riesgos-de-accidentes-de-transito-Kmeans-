// Package api es el coordinador del clúster: expone HTTP/JSON + WebSocket,
// reparte el entrenamiento a los nodos por TCP y persiste en Postgres/Redis.
package api

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"siniestros/internal/kmeans"
	"siniestros/internal/red"
)

//go:embed openapi.yaml
var openapiYAML []byte

type Config struct {
	DB      *sql.DB
	Redis   *redis.Client
	Nodos   []string
	Secret  string
	DataDir string
}

type Servidor struct {
	Config
	zonas   []Zona
	pts     []kmeans.Punto // solo para inicializar K-Means++
	maxPct  float64
	met     *Metricas
	logCons chan consulta
	entMu   sync.Mutex // un entrenamiento a la vez
}

type consulta struct {
	lat, lon   float64
	hora, zr   int
	riesgo, ms float64
}

func Nuevo(c Config) (*Servidor, error) {
	s := &Servidor{Config: c, met: NuevaMetricas(), logCons: make(chan consulta, 1024)}
	zs, err := CargarZonas(c.DB)
	if err != nil {
		return nil, err
	}
	s.zonas = zs
	for _, z := range zs {
		s.maxPct = math.Max(s.maxPct, z.PctFallecidos)
	}
	s.pts, _ = red.CargarPuntos(filepath.Join(c.DataDir, "kmeans_puntos_reales.csv"))
	go s.escribirConsultas()
	return s, nil
}

// escribirConsultas persiste el historial fuera de la ruta crítica (no suma latencia).
func (s *Servidor) escribirConsultas() {
	for q := range s.logCons {
		s.DB.Exec(`INSERT INTO consultas(lat,lon,hora,zona_ranking,riesgo,latencia_ms) VALUES($1,$2,$3,$4,$5,$6)`,
			q.lat, q.lon, q.hora, q.zr, q.riesgo, q.ms)
	}
}

func (s *Servidor) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /health", s.health)
	m.HandleFunc("POST /api/login", s.login)
	m.HandleFunc("GET /api/zonas", s.listarZonas)
	m.HandleFunc("GET /api/zonas/{ranking}", s.zona)
	m.HandleFunc("POST /api/predict", s.predict)
	m.HandleFunc("GET /api/siniestros", s.listarSiniestros)
	m.HandleFunc("POST /api/siniestros", s.auth(s.crearSiniestro))
	m.HandleFunc("DELETE /api/siniestros/{id}", s.auth(s.borrarSiniestro))
	m.HandleFunc("POST /api/entrenar", s.auth(s.entrenar))
	m.HandleFunc("GET /api/entrenamientos", s.auth(s.entrenamientos))
	m.HandleFunc("GET /api/admin/metricas", s.auth(s.metricas))
	m.HandleFunc("GET /ws/metricas", s.ws)
	m.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(openapiYAML)
	})
	m.HandleFunc("GET /swagger", swaggerUI)
	m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/swagger", http.StatusFound) })
	return s.mw(m)
}

// mw: CORS, cronómetro (X-Response-Time) y registro de métricas por ruta.
func (s *Servidor) mw(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/ws/") {
			next.ServeHTTP(w, r) // WebSocket necesita el ResponseWriter original (Hijack)
			return
		}
		sw := &statusWriter{ResponseWriter: w, t0: time.Now(), status: 200}
		next.ServeHTTP(sw, r)
		ms := float64(time.Since(sw.t0).Microseconds()) / 1000
		s.met.Registrar(r.Method+" "+r.Pattern, ms, sw.status)
	})
}

// statusWriter fija X-Response-Time justo antes de escribir la cabecera.
type statusWriter struct {
	http.ResponseWriter
	t0     time.Time
	status int
	done   bool
}

func (w *statusWriter) WriteHeader(c int) {
	if !w.done {
		w.done = true
		w.status = c
		w.Header().Set("X-Response-Time", fmt.Sprintf("%.3fms", float64(time.Since(w.t0).Microseconds())/1000))
	}
	w.ResponseWriter.WriteHeader(c)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.done {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}

func js(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func errj(w http.ResponseWriter, code int, msg string) { js(w, code, map[string]string{"error": msg}) }

func (s *Servidor) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if _, err := ValidarToken(s.Secret, tok); err != nil {
			errj(w, 401, "se requiere JWT válido (POST /api/login)")
			return
		}
		h(w, r)
	}
}

func (s *Servidor) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	pg, rd := "ok", "ok"
	if s.DB.PingContext(ctx) != nil {
		pg = "caído"
	}
	if s.Redis == nil || s.Redis.Ping(ctx).Err() != nil {
		rd = "caído"
	}
	nodos := red.Sondear(s.Nodos)
	act := 0
	for _, n := range nodos {
		if n.Activo {
			act++
		}
	}
	js(w, 200, map[string]any{"api": "ok", "postgres": pg, "redis": rd, "nodos_activos": act, "nodos_total": len(nodos), "nodos": nodos})
}

func (s *Servidor) login(w http.ResponseWriter, r *http.Request) {
	var b struct{ Usuario, Clave string }
	if json.NewDecoder(r.Body).Decode(&b) != nil {
		errj(w, 400, "JSON inválido")
		return
	}
	var rol string
	err := s.DB.QueryRow(`SELECT rol FROM usuarios WHERE usuario=$1 AND clave_sha256=$2`, b.Usuario, sha(b.Clave)).Scan(&rol)
	if err != nil {
		errj(w, 401, "credenciales inválidas")
		return
	}
	js(w, 200, map[string]any{"token": NuevoToken(s.Secret, b.Usuario, rol, 2*time.Hour), "tipo": "Bearer", "expira_en_s": 7200, "rol": rol})
}

// ---- Zonas (caché Redis) ----

func (s *Servidor) cacheado(w http.ResponseWriter, r *http.Request, clave string, gen func() (any, int)) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if s.Redis != nil {
		if v, err := s.Redis.Get(ctx, clave).Bytes(); err == nil {
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(200)
			w.Write(v)
			return
		}
	}
	v, code := gen()
	b, _ := json.Marshal(v)
	if code == 200 && s.Redis != nil {
		if s.Redis.Set(ctx, clave, b, 60*time.Second).Err() == nil {
			w.Header().Set("X-Cache", "MISS")
		} else {
			w.Header().Set("X-Cache", "BYPASS")
		}
	} else {
		w.Header().Set("X-Cache", "BYPASS")
	}
	w.WriteHeader(code)
	w.Write(b)
}

func (s *Servidor) listarZonas(w http.ResponseWriter, r *http.Request) {
	s.cacheado(w, r, "zonas:all", func() (any, int) {
		out := make([]Zona, len(s.zonas))
		for i, z := range s.zonas {
			z.PorHora = nil
			out[i] = z
		}
		return map[string]any{"total": len(out), "zonas": out}, 200
	})
}

func (s *Servidor) zona(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("ranking"))
	if err != nil || n < 1 || n > len(s.zonas) {
		errj(w, 404, "zona no existe (ranking 1.."+strconv.Itoa(len(s.zonas))+")")
		return
	}
	s.cacheado(w, r, "zona:"+r.PathValue("ranking"), func() (any, int) { return s.zonas[n-1], 200 })
}

// zonaCercana devuelve el índice de la zona más cercana y la distancia en km.
func (s *Servidor) zonaCercana(lat, lon float64) (int, float64) {
	p := red.Proyectar(lat, lon)
	mejor, dmin := 0, math.MaxFloat64
	for i, z := range s.zonas {
		c := red.Proyectar(z.Lat, z.Lon)
		if d := math.Hypot(p.X-c.X, p.Y-c.Y); d < dmin {
			mejor, dmin = i, d
		}
	}
	return mejor, dmin
}

// ---- Predicción (todo en memoria: sin I/O en la ruta crítica) ----

func (s *Servidor) predict(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	var b struct {
		Lat, Lon *float64
		Hora     *int
	}
	if json.NewDecoder(r.Body).Decode(&b) != nil || b.Lat == nil || b.Lon == nil {
		errj(w, 400, "se requieren lat y lon (hora opcional 0-23)")
		return
	}
	hora := -1
	if b.Hora != nil {
		hora = *b.Hora
		if hora < 0 || hora > 23 {
			errj(w, 400, "hora debe estar entre 0 y 23")
			return
		}
	}
	if *b.Lat < -19 || *b.Lat > 1 || *b.Lon < -82 || *b.Lon > -68 {
		errj(w, 400, "coordenadas fuera del Perú")
		return
	}
	mejor, dmin := s.zonaCercana(*b.Lat, *b.Lon)
	z := s.zonas[mejor]
	base := z.PctFallecidos / s.maxPct
	factorHora := 1.0
	if hora >= 0 && len(z.PorHora) == 24 {
		mx := 1
		for _, v := range z.PorHora {
			if v > mx {
				mx = v
			}
		}
		factorHora = float64(z.PorHora[hora]) / float64(mx)
	}
	riesgo := math.Round((0.6*base+0.4*factorHora)*1000) / 1000
	nivel := "BAJO"
	if riesgo >= 0.66 {
		nivel = "ALTO"
	} else if riesgo >= 0.33 {
		nivel = "MEDIO"
	}
	ms := float64(time.Since(t0).Microseconds()) / 1000
	select {
	case s.logCons <- consulta{*b.Lat, *b.Lon, hora, z.Ranking, riesgo, ms}:
	default:
	}
	js(w, 200, map[string]any{
		"zona_ranking": z.Ranking, "zona": z.Distrito + ", " + z.Departamento, "distancia_km": math.Round(dmin*10) / 10,
		"riesgo": riesgo, "nivel": nivel, "hora_consultada": hora, "franja_pico": z.FranjaPico,
		"clase_principal": z.Clase, "causa_principal": z.Causa, "calculo_ms": ms,
	})
}

// ---- Siniestros (CRUD en Postgres) ----

func (s *Servidor) listarSiniestros(w http.ResponseWriter, r *http.Request) {
	lim, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if lim < 1 || lim > 200 {
		lim = 20
	}
	dep := strings.ToUpper(r.URL.Query().Get("departamento"))
	var total int
	s.DB.QueryRow(`SELECT count(*) FROM siniestros WHERE ($1='' OR departamento=$1)`, dep).Scan(&total)
	rows, err := s.DB.Query(`SELECT id,lat,lon,hora,fallecidos,coalesce(departamento,''),coalesce(zona_ranking,0) FROM siniestros WHERE ($1='' OR departamento=$1) ORDER BY id DESC LIMIT $2`, dep, lim)
	if err != nil {
		errj(w, 500, err.Error())
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, h, f, zr int
		var la, lo float64
		var d string
		rows.Scan(&id, &la, &lo, &h, &f, &d, &zr)
		items = append(items, map[string]any{"id": id, "lat": la, "lon": lo, "hora": h, "fallecidos": f, "departamento": d, "zona_ranking": zr})
	}
	js(w, 200, map[string]any{"total": total, "devueltos": len(items), "items": items})
}

func (s *Servidor) crearSiniestro(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Lat, Lon     *float64
		Hora         *int
		Fallecidos   int
		Departamento string
	}
	if json.NewDecoder(r.Body).Decode(&b) != nil || b.Lat == nil || b.Lon == nil || b.Hora == nil || *b.Hora < 0 || *b.Hora > 23 {
		errj(w, 400, "se requieren lat, lon y hora (0-23)")
		return
	}
	mejor, _ := s.zonaCercana(*b.Lat, *b.Lon)
	zr := s.zonas[mejor].Ranking
	var id int
	err := s.DB.QueryRow(`INSERT INTO siniestros(lat,lon,hora,fallecidos,departamento,zona_ranking) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		*b.Lat, *b.Lon, *b.Hora, b.Fallecidos, strings.ToUpper(b.Departamento), zr).Scan(&id)
	if err != nil {
		errj(w, 500, err.Error())
		return
	}
	js(w, 201, map[string]any{"id": id, "zona_ranking_asignada": zr, "mensaje": "siniestro registrado"})
}

func (s *Servidor) borrarSiniestro(w http.ResponseWriter, r *http.Request) {
	res, err := s.DB.Exec(`DELETE FROM siniestros WHERE id=$1`, r.PathValue("id"))
	if err != nil {
		errj(w, 404, "siniestro no existe")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		errj(w, 404, "siniestro no existe")
		return
	}
	js(w, 200, map[string]any{"eliminado": r.PathValue("id")})
}

// ---- Clúster ML ----

func (s *Servidor) entrenar(w http.ResponseWriter, r *http.Request) {
	var b struct {
		K       int `json:"k"`
		MaxIter int `json:"max_iter"`
	}
	json.NewDecoder(r.Body).Decode(&b)
	if b.K == 0 {
		b.K = 15
	}
	if b.MaxIter == 0 {
		b.MaxIter = 50
	}
	if b.K < 2 || b.K > 60 || b.MaxIter < 1 || b.MaxIter > 200 {
		errj(w, 400, "k en 2..60 y max_iter en 1..200")
		return
	}
	if !s.entMu.TryLock() {
		errj(w, 409, "ya hay un entrenamiento en curso")
		return
	}
	defer s.entMu.Unlock()
	ini := kmeans.Inicializar(s.pts, b.K, 5000, 42)
	res, err := red.Entrenar(s.Nodos, ini, b.MaxIter, 0.01)
	if err != nil {
		errj(w, 503, err.Error())
		return
	}
	cents := make([]map[string]any, len(res.Centroides))
	for i, c := range res.Centroides {
		la, lo := red.Inversa(c)
		cents[i] = map[string]any{"cluster": i, "lat": math.Round(la*1e5) / 1e5, "lon": math.Round(lo*1e5) / 1e5, "puntos": res.Tamanos[i]}
	}
	cj, _ := json.Marshal(cents)
	var id int
	s.DB.QueryRow(`INSERT INTO entrenamientos(k,iteraciones,sse,convergio,duracion_ms,nodos,centroides) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		b.K, res.Iteraciones, res.SSE, res.Convergio, float64(res.Duracion.Microseconds())/1000, len(s.Nodos), cj).Scan(&id)
	js(w, 200, map[string]any{"id": id, "k": b.K, "nodos": len(s.Nodos), "iteraciones": res.Iteraciones, "convergio": res.Convergio,
		"sse_km2": math.Round(res.SSE), "duracion_ms": float64(res.Duracion.Microseconds()) / 1000, "ms_por_iteracion": res.MsPorIter, "centroides": cents})
}

func (s *Servidor) entrenamientos(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`SELECT id,k,iteraciones,sse,convergio,duracion_ms,nodos,creado FROM entrenamientos ORDER BY id DESC LIMIT 20`)
	if err != nil {
		errj(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, k, it, n int
		var sse, ms float64
		var c bool
		var t time.Time
		rows.Scan(&id, &k, &it, &sse, &c, &ms, &n, &t)
		out = append(out, map[string]any{"id": id, "k": k, "iteraciones": it, "sse_km2": math.Round(sse), "convergio": c, "duracion_ms": ms, "nodos": n, "creado": t})
	}
	js(w, 200, out)
}

func (s *Servidor) snapshot() map[string]any {
	m := s.met.Snapshot()
	m["nodos"] = red.Sondear(s.Nodos)
	return m
}

func (s *Servidor) metricas(w http.ResponseWriter, r *http.Request) { js(w, 200, s.snapshot()) }

var up = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

func (s *Servidor) ws(w http.ResponseWriter, r *http.Request) {
	if _, err := ValidarToken(s.Secret, r.URL.Query().Get("token")); err != nil {
		errj(w, 401, "token requerido en ?token=")
		return
	}
	c, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()
	go func() { // lector para detectar cierre del cliente
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				c.Close()
				return
			}
		}
	}()
	for {
		if c.WriteJSON(s.snapshot()) != nil {
			return
		}
		time.Sleep(time.Second)
	}
}

func swaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!doctype html><html><head><meta charset="utf-8"><title>Swagger - API Siniestros K-Means</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css"></head><body><div id="ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url:'/openapi.yaml',dom_id:'#ui'})</script></body></html>`)
}
