package api

import (
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Metricas: contadores atómicos + ventana de las últimas latencias por ruta.
type Metricas struct {
	inicio   time.Time
	Peticion atomic.Int64
	Errores  atomic.Int64
	mu       sync.Mutex
	lat      map[string][]float64 // ruta -> últimas 1000 latencias (ms)
}

func NuevaMetricas() *Metricas { return &Metricas{inicio: time.Now(), lat: map[string][]float64{}} }

func (m *Metricas) Registrar(ruta string, ms float64, status int) {
	m.Peticion.Add(1)
	if status >= 500 {
		m.Errores.Add(1)
	}
	m.mu.Lock()
	l := append(m.lat[ruta], ms)
	if len(l) > 1000 {
		l = l[len(l)-1000:]
	}
	m.lat[ruta] = l
	m.mu.Unlock()
}

func pct(s []float64, p float64) float64 {
	if len(s) == 0 {
		return 0
	}
	c := append([]float64(nil), s...)
	sort.Float64s(c)
	return c[int(float64(len(c)-1)*p)]
}

func (m *Metricas) Snapshot() map[string]any {
	m.mu.Lock()
	rutas := map[string]any{}
	for r, l := range m.lat {
		rutas[r] = map[string]any{"n": len(l), "p50_ms": pct(l, .5), "p95_ms": pct(l, .95), "max_ms": pct(l, 1)}
	}
	m.mu.Unlock()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return map[string]any{
		"uptime_s": int(time.Since(m.inicio).Seconds()), "peticiones": m.Peticion.Load(), "errores_5xx": m.Errores.Load(),
		"goroutines": runtime.NumGoroutine(), "cpus": runtime.NumCPU(), "mem_mb": float64(ms.Alloc) / 1e6, "latencias": rutas,
	}
}
