// Comando loader: Parte A - cargador concurrente.
//
// Ejemplos:
//
//	go run ./cmd/loader                                  # limpia con todos los núcleos
//	go run ./cmd/loader -bench 1,2,4,8,12 -rep 3         # compara secuencial vs paralelo
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"

	"siniestros/internal/cargador"
	"siniestros/internal/limpieza"
)

func main() {
	entrada := flag.String("entrada", "data/siniestros_sucio.csv", "CSV de entrada")
	salida := flag.String("salida", "data/siniestros_limpio.csv", "CSV limpio de salida")
	metricas := flag.String("metricas", "data/metricas_loader.json", "archivo JSON con métricas")
	// 8 workers: mejor tiempo medido en el benchmark (ver docs/BITACORA.md §5.1).
	// En una CPU con menos de 8 hilos se usa runtime.NumCPU().
	workers := flag.Int("workers", min(8, runtime.NumCPU()), "número de goroutines worker")
	bloqueMB := flag.Int("bloque", 4, "tamaño de bloque en MB")
	bench := flag.String("bench", "", "lista de workers a comparar, ej: 1,2,4,8")
	rep := flag.Int("rep", 3, "repeticiones por configuración en modo bench")
	gogc := flag.Int("gogc", 400, "porcentaje del recolector de basura (100 = defecto de Go)")
	flag.Parse()
	debug.SetGCPercent(*gogc)
	tamBloque := int64(*bloqueMB) << 20

	fmt.Printf("SO: %s/%s | CPUs: %d | GOGC: %d | entrada: %s\n\n", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), *gogc, *entrada)

	reporte := map[string]any{"so": runtime.GOOS, "cpus": runtime.NumCPU()}

	if *bench != "" {
		reporte["benchmark"] = benchmark(*entrada, *bench, *rep, tamBloque)
	}

	out, err := os.Create(*salida)
	if err != nil {
		log.Fatal(err)
	}
	st, m, err := cargador.Procesar(*entrada, *workers, tamBloque, out)
	out.Close()
	if err != nil {
		log.Fatal(err)
	}
	imprimirStats(st)
	fmt.Printf("\nTiempo con %d workers: %.0f ms (%.0f filas/s, %d bloques)\nCSV limpio: %s\n",
		m.Workers, m.DuracionMs, m.FilasPorSeg, m.Bloques, *salida)

	reporte["ejecucion"] = m
	reporte["limpieza"] = st
	if b, err := json.MarshalIndent(reporte, "", "  "); err == nil {
		os.WriteFile(*metricas, b, 0o644)
		fmt.Printf("Métricas: %s\n", *metricas)
	}
}

type filaBench struct {
	Workers    int     `json:"workers"`
	MedianaMs  float64 `json:"mediana_ms"`
	Speedup    float64 `json:"speedup"`
	Eficiencia float64 `json:"eficiencia"`
}

func benchmark(entrada, lista string, rep int, tamBloque int64) []filaBench {
	// Calentamiento: deja el archivo en la caché del SO para no medir el disco.
	if _, _, err := cargador.Procesar(entrada, runtime.NumCPU(), tamBloque, io.Discard); err != nil {
		log.Fatal(err)
	}
	var res []filaBench
	var base float64
	fmt.Println("| Workers | Tiempo mediana (ms) | Speedup | Eficiencia |")
	fmt.Println("|---:|---:|---:|---:|")
	for _, s := range strings.Split(lista, ",") {
		w, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || w < 1 {
			log.Fatalf("valor de -bench inválido: %q", s)
		}
		var tiempos []float64
		for i := 0; i < rep; i++ {
			_, m, err := cargador.Procesar(entrada, w, tamBloque, io.Discard)
			if err != nil {
				log.Fatal(err)
			}
			tiempos = append(tiempos, m.DuracionMs)
		}
		sort.Float64s(tiempos)
		med := tiempos[len(tiempos)/2]
		if base == 0 {
			base = med * float64(w) // si la lista no empieza en 1, aproxima
			if w == 1 {
				base = med
			}
		}
		fb := filaBench{Workers: w, MedianaMs: med, Speedup: base / med, Eficiencia: base / med / float64(w)}
		res = append(res, fb)
		fmt.Printf("| %d | %.0f | %.2fx | %.0f%% |\n", fb.Workers, fb.MedianaMs, fb.Speedup, fb.Eficiencia*100)
	}
	fmt.Println()
	return res
}

func imprimirStats(st *limpieza.Stats) {
	fmt.Printf("Filas leídas : %d\nFilas válidas: %d (%.2f%%)\n\n", st.Leidas, st.Validas,
		100*float64(st.Validas)/float64(st.Leidas))
	tabla := func(titulo string, m map[string]int) {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })
		fmt.Printf("| %s | Filas |\n|---|---:|\n", titulo)
		for _, k := range keys {
			fmt.Printf("| %s | %d |\n", k, m[k])
		}
		fmt.Println()
	}
	tabla("Descarte", st.Descartes)
	tabla("Corrección", st.Correcciones)
}
