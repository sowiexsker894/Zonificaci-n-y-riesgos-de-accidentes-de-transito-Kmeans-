// Comando kmeans: Parte B - K-Means paralelo para detectar "puntos negros".
//
// Ejemplos:
//
//	go run ./cmd/kmeans                          # elige k por el método del codo
//	go run ./cmd/kmeans -k 20                    # k fijo
//	go run ./cmd/kmeans -bench 1,2,4,8,12        # benchmark secuencial vs paralelo
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"siniestros/internal/datos"
	"siniestros/internal/kmeans"
)

const kmPorGradoLat = 110.574

func main() {
	entrada := flag.String("entrada", "data/siniestros_limpio.csv", "CSV limpio (salida de cmd/loader)")
	dirSalida := flag.String("salida", "data", "carpeta de resultados")
	k := flag.Int("k", 0, "número de clusters (0 = elegir por el método del codo)")
	kmin := flag.Int("kmin", 5, "k mínimo para el codo")
	kmax := flag.Int("kmax", 60, "k máximo para el codo")
	kpaso := flag.Int("kpaso", 5, "paso de k para el codo")
	// K-Means es cálculo puro (sin E/S ni asignaciones en la fase MAP): escala
	// hasta todos los hilos lógicos. Ver docs/BITACORA.md §6.4.
	workers := flag.Int("workers", runtime.NumCPU(), "goroutines worker (particiones)")
	maxIter := flag.Int("iter", 100, "iteraciones máximas")
	tol := flag.Float64("tol", 0.01, "convergencia: desplazamiento máximo de centroides en km")
	semilla := flag.Int64("semilla", 42, "semilla de K-Means++")
	bench := flag.String("bench", "", "lista de workers a comparar, ej: 1,2,4,8,12")
	rep := flag.Int("rep", 5, "repeticiones por configuración en modo bench")
	benchIter := flag.Int("bench-iter", 30, "iteraciones fijas por corrida en modo bench (sin parar al converger)")
	flag.Parse()
	debug.SetGCPercent(400)

	fmt.Printf("SO: %s/%s | CPUs: %d | workers: %d\n\n", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), *workers)
	reporte := map[string]any{"so": runtime.GOOS, "cpus": runtime.NumCPU(), "workers": *workers}

	// 1. Carga paralela
	t0 := time.Now()
	ds, err := datos.Cargar(*entrada, *workers)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Carga: %d filas en %v\n", ds.Len(), time.Since(t0).Round(time.Millisecond))
	reporte["filas"] = ds.Len()
	reporte["carga_ms"] = time.Since(t0).Milliseconds()

	// 2. Proyección equirectangular a km (lat de referencia = media)
	lat0 := 0.0
	for _, v := range ds.Lat {
		lat0 += v
	}
	lat0 /= float64(ds.Len())
	kmPorGradoLon := 111.320 * math.Cos(lat0*math.Pi/180)
	pts := make([]kmeans.Punto, ds.Len())
	for i := range pts {
		pts[i] = kmeans.Punto{X: ds.Lon[i] * kmPorGradoLon, Y: ds.Lat[i] * kmPorGradoLat}
	}
	aLatLon := func(p kmeans.Punto) (float64, float64) { return p.Y / kmPorGradoLat, p.X / kmPorGradoLon }
	cfg := kmeans.Config{Workers: *workers, MaxIter: *maxIter, Tol: *tol}

	// 3. Método del codo
	if *k == 0 {
		var ks []int
		var sses []float64
		fmt.Println("\n## Método del codo\n\n| k | SSE (km²) | Iteraciones | Tiempo (ms) |\n|---:|---:|---:|---:|")
		codoCSV := "k,sse,iteraciones,ms\n"
		for kk := *kmin; kk <= *kmax; kk += *kpaso {
			cfg.K = kk
			r := kmeans.Entrenar(pts, kmeans.Inicializar(pts, kk, 50000, *semilla), cfg)
			ks, sses = append(ks, kk), append(sses, r.SSE)
			ms := float64(r.Duracion.Microseconds()) / 1000
			fmt.Printf("| %d | %.4g | %d | %.0f |\n", kk, r.SSE, r.Iteraciones, ms)
			codoCSV += fmt.Sprintf("%d,%.6g,%d,%.1f\n", kk, r.SSE, r.Iteraciones, ms)
		}
		*k = kmeans.Codo(ks, sses)
		fmt.Printf("\nk elegido por el codo (Kneedle): %d\n", *k)
		escribir(filepath.Join(*dirSalida, "kmeans_codo.csv"), []byte(codoCSV))
		reporte["codo"] = map[string]any{"k": ks, "sse": sses, "k_elegido": *k}
	}
	cfg.K = *k
	ini := kmeans.Inicializar(pts, *k, 50000, *semilla)

	// 4. Benchmark de escalabilidad (misma inicialización => mismas iteraciones)
	if *bench != "" {
		cb := cfg
		cb.MaxIter, cb.Tol = *benchIter, -1 // misma carga de trabajo para todos
		reporte["benchmark"] = benchmark(pts, ini, cb, *bench, *rep)
	}

	// 5. Entrenamiento final
	r := kmeans.Entrenar(pts, ini, cfg)
	fmt.Printf("\nEntrenamiento final: k=%d, %d iteraciones (convergió=%v), %.0f ms, %.1f ms/iteración, SSE=%.4g\n",
		*k, r.Iteraciones, r.Convergio, float64(r.Duracion.Microseconds())/1000, r.MsPorIter, r.SSE)
	reporte["entrenamiento"] = map[string]any{
		"k": *k, "iteraciones": r.Iteraciones, "convergio": r.Convergio,
		"ms": r.Duracion.Milliseconds(), "ms_por_iteracion": r.MsPorIter, "sse": r.SSE,
	}

	// 6. Perfil de cada cluster
	perfiles := perfilar(ds, pts, r, aLatLon)
	imprimirTop(perfiles, 15)
	if b, err := json.MarshalIndent(perfiles, "", "  "); err == nil {
		escribir(filepath.Join(*dirSalida, "kmeans_clusters.json"), b)
	}
	exportarReales(filepath.Join(*dirSalida, "kmeans_puntos_reales.csv"), ds, r.Asig)
	if b, err := json.MarshalIndent(reporte, "", "  "); err == nil {
		escribir(filepath.Join(*dirSalida, "metricas_kmeans.json"), b)
	}
}

type filaBench struct {
	Workers    int     `json:"workers"`
	MedianaMs  float64 `json:"mediana_ms"`
	MsPorIter  float64 `json:"ms_por_iteracion"`
	Speedup    float64 `json:"speedup"`
	Eficiencia float64 `json:"eficiencia"`
}

func benchmark(pts []kmeans.Punto, ini []kmeans.Punto, cfg kmeans.Config, lista string, rep int) []filaBench {
	fmt.Printf("\n## Benchmark K-Means (k=%d, %d iteraciones fijas)\n\n| Workers | Tiempo mediana (ms) | ms/iteración | Speedup | Eficiencia |\n|---:|---:|---:|---:|---:|\n", cfg.K, cfg.MaxIter)
	kmeans.Entrenar(pts, ini, cfg) // calentamiento (caché y frecuencia de CPU)
	var res []filaBench
	var base float64
	for _, s := range strings.Split(lista, ",") {
		w, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || w < 1 {
			log.Fatalf("valor de -bench inválido: %q", s)
		}
		c := cfg
		c.Workers = w
		var ts, its []float64
		for i := 0; i < rep; i++ {
			r := kmeans.Entrenar(pts, ini, c)
			ts = append(ts, float64(r.Duracion.Microseconds())/1000)
			its = append(its, r.MsPorIter)
		}
		sort.Float64s(ts)
		sort.Float64s(its)
		med := ts[len(ts)/2]
		if base == 0 {
			base = med * float64(w)
		}
		fb := filaBench{w, med, its[len(its)/2], base / med, base / med / float64(w)}
		res = append(res, fb)
		fmt.Printf("| %d | %.0f | %.1f | %.2fx | %.0f%% |\n", w, med, fb.MsPorIter, fb.Speedup, fb.Eficiencia*100)
	}
	return res
}

// Causas que no aportan información para el perfil de la zona.
var noInformativa = map[string]bool{
	"": true, "NO CUENTA CON CAUSA ESPECIFICA": true, "EN PROCESO DE INVESTIGACIÓN": true,
	"EN INVESTIGACIÓN": true, "DESCONOCIDO": true,
}

// Perfil describe un cluster (zona) para responder "dónde y a qué hora".
type Perfil struct {
	Ranking          int     `json:"ranking"`
	Cluster          int     `json:"cluster"`
	Lat              float64 `json:"lat"`
	Lon              float64 `json:"lon"`
	Siniestros       int     `json:"siniestros"`
	SiniestrosReales int     `json:"siniestros_reales"`
	Fallecidos       int     `json:"fallecidos"`
	FallecidosReales int     `json:"fallecidos_reales"`
	PctFallecidos    float64 `json:"pct_fallecidos_nacional"`
	RadioMedioKm     float64 `json:"radio_medio_km"`
	Departamento     string  `json:"departamento"`
	Provincia        string  `json:"provincia"`
	Distrito         string  `json:"distrito"`
	HoraPico         int     `json:"hora_pico"`
	FranjaPico       string  `json:"franja_pico_3h"`
	PctFranjaPico    float64 `json:"pct_franja_pico"`
	ClasePrincipal   string  `json:"clase_principal"`
	CausaPrincipal   string  `json:"causa_principal"`
	Horas            [24]int `json:"siniestros_por_hora"`
}

func perfilar(ds *datos.Dataset, pts []kmeans.Punto, r kmeans.Resultado, aLatLon func(kmeans.Punto) (float64, float64)) []Perfil {
	k := len(r.Centroides)
	ps := make([]Perfil, k)
	type conteo map[string]int
	nuevo := func() []conteo {
		c := make([]conteo, k)
		for i := range c {
			c[i] = conteo{}
		}
		return c
	}
	zona, clase, causa := nuevo(), nuevo(), nuevo()
	sumDist := make([]float64, k)
	totalFall := 0
	for i, c32 := range r.Asig {
		c := int(c32)
		p := &ps[c]
		f := int(ds.Fallecidos[i])
		p.Siniestros++
		p.Fallecidos += f
		totalFall += f
		if !ds.Sintetico[i] {
			p.SiniestrosReales++
			p.FallecidosReales += f
		}
		p.Horas[ds.Hora[i]]++
		zona[c][ds.Depto[i]+"|"+ds.Prov[i]+"|"+ds.Dist[i]]++ // moda conjunta: zona coherente
		clase[c][ds.Clase[i]]++
		if !noInformativa[ds.Causa[i]] {
			causa[c][ds.Causa[i]]++
		}
		sumDist[c] += math.Hypot(pts[i].X-r.Centroides[c].X, pts[i].Y-r.Centroides[c].Y)
	}
	moda := func(m conteo) string {
		mejor, n := "", -1
		for s, v := range m {
			if v > n || (v == n && s < mejor) {
				mejor, n = s, v
			}
		}
		return mejor
	}
	for c := range ps {
		p := &ps[c]
		p.Cluster = c
		p.Lat, p.Lon = aLatLon(r.Centroides[c])
		p.Lat, p.Lon = math.Round(p.Lat*1e5)/1e5, math.Round(p.Lon*1e5)/1e5
		if p.Siniestros == 0 {
			continue
		}
		p.PctFallecidos = math.Round(10000*float64(p.Fallecidos)/float64(totalFall)) / 100
		p.RadioMedioKm = math.Round(10*sumDist[c]/float64(p.Siniestros)) / 10
		if z := strings.SplitN(moda(zona[c]), "|", 3); len(z) == 3 {
			p.Departamento, p.Provincia, p.Distrito = z[0], z[1], z[2]
		}
		p.ClasePrincipal, p.CausaPrincipal = moda(clase[c]), moda(causa[c])
		// hora pico y mejor ventana circular de 3 horas
		mejorV := -1
		for h := 0; h < 24; h++ {
			if p.Horas[h] > p.Horas[p.HoraPico] {
				p.HoraPico = h
			}
			v := p.Horas[h] + p.Horas[(h+1)%24] + p.Horas[(h+2)%24]
			if v > mejorV {
				mejorV = v
				p.FranjaPico = fmt.Sprintf("%02d:00-%02d:59", h, (h+2)%24)
			}
		}
		p.PctFranjaPico = math.Round(1000*float64(mejorV)/float64(p.Siniestros)) / 10
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].Fallecidos > ps[j].Fallecidos })
	for i := range ps {
		ps[i].Ranking = i + 1
	}
	return ps
}

func imprimirTop(ps []Perfil, n int) {
	fmt.Printf("\n## Puntos negros (top %d por fallecidos)\n\n", n)
	fmt.Println("| # | Zona (moda) | Centro (lat, lon) | Radio km | Siniestros reales | Fallecidos reales | % fallecidos | Franja pico | Clase | Causa |")
	fmt.Println("|---:|---|---|---:|---:|---:|---:|---|---|---|")
	for _, p := range ps[:min(n, len(ps))] {
		fmt.Printf("| %d | %s / %s / %s | %.4f, %.4f | %.1f | %d | %d | %.2f%% | %s (%.0f%%) | %s | %s |\n",
			p.Ranking, p.Departamento, p.Provincia, p.Distrito, p.Lat, p.Lon, p.RadioMedioKm,
			p.SiniestrosReales, p.FallecidosReales, p.PctFallecidos, p.FranjaPico, p.PctFranjaPico,
			p.ClasePrincipal, p.CausaPrincipal)
	}
}

// exportarReales guarda solo los siniestros reales con su cluster (para mapas).
func exportarReales(ruta string, ds *datos.Dataset, asig []int32) {
	var sb strings.Builder
	sb.WriteString("latitud,longitud,cluster,hora,fallecidos,departamento\n")
	for i := range asig {
		if !ds.Sintetico[i] {
			fmt.Fprintf(&sb, "%.6f,%.6f,%d,%d,%d,%s\n", ds.Lat[i], ds.Lon[i], asig[i], ds.Hora[i], ds.Fallecidos[i], ds.Depto[i])
		}
	}
	escribir(ruta, []byte(sb.String()))
}

func escribir(ruta string, b []byte) {
	if err := os.WriteFile(ruta, b, 0o644); err != nil {
		log.Printf("no se pudo escribir %s: %v", ruta, err)
		return
	}
	fmt.Printf("Guardado: %s\n", ruta)
}
