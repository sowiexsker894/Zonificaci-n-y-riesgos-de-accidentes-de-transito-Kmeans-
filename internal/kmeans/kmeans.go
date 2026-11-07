// Package kmeans implementa K-Means paralelo con el patrón MapReduce:
//
//	         ┌────────── centroides globales (solo lectura) ──────────┐
//	         ▼                         ▼                              ▼
//	[worker 1: partición 1]   [worker 2: partición 2]  ...   [worker N: partición N]
//	 MAP: asigna cada punto al centroide más cercano y acumula
//	      sumas parciales (Σx, Σy, conteo, SSE) en SU propia estructura
//	         │                         │                              │
//	         └──────────────► chan Parcial ◄──────────────────────────┘
//	                                   ▼
//	              [master] REDUCE: suma parciales en orden fijo,
//	              calcula nuevos centroides = Σ / conteo, revisa convergencia
//
// Sin condiciones de carrera:
//   - cada worker solo escribe en su Parcial y en su rango de `asig`;
//   - los centroides de una iteración no se modifican mientras los workers
//     los leen: el master crea un slice nuevo para la siguiente iteración;
//   - solo el master actualiza los centroides.
//
// Cada worker recibe centroides y responde un Parcial por canales. En la
// versión distribuida, ese mismo intercambio viaja por TCP a los nodos.
package kmeans

import (
	"math"
	"math/rand"
	"time"
)

// Punto en coordenadas planas (km).
type Punto struct{ X, Y float64 }

// Parcial es el resultado de la fase MAP de una partición.
type Parcial struct {
	Sumas   []Punto
	Conteos []int
	SSE     float64
}

// Config de entrenamiento.
type Config struct {
	K       int
	Workers int
	MaxIter int
	Tol     float64 // desplazamiento máximo de centroides (km) para converger
}

// Resultado del entrenamiento.
type Resultado struct {
	Centroides  []Punto
	Asig        []int32
	Tamanos     []int
	SSE         float64
	Iteraciones int
	Convergio   bool
	Duracion    time.Duration
	MsPorIter   float64
}

// Asignar es la fase MAP sobre una partición: escribe en asig el cluster de
// cada punto y devuelve las sumas parciales.
func Asignar(pts []Punto, cents []Punto, asig []int32) Parcial {
	k := len(cents)
	p := Parcial{Sumas: make([]Punto, k), Conteos: make([]int, k)}
	for i, pt := range pts {
		c, d2 := cercano(pt, cents)
		asig[i] = int32(c)
		p.Sumas[c].X += pt.X
		p.Sumas[c].Y += pt.Y
		p.Conteos[c]++
		p.SSE += d2
	}
	return p
}

func cercano(p Punto, cents []Punto) (int, float64) {
	mejor, dmin := 0, math.MaxFloat64
	for j, c := range cents {
		dx, dy := p.X-c.X, p.Y-c.Y
		if d := dx*dx + dy*dy; d < dmin {
			mejor, dmin = j, d
		}
	}
	return mejor, dmin
}

// Entrenar ejecuta K-Means paralelo desde los centroides iniciales `ini`.
func Entrenar(pts []Punto, ini []Punto, cfg Config) Resultado {
	inicio := time.Now()
	n, w := len(pts), cfg.Workers
	if w < 1 {
		w = 1
	}
	if w > n {
		w = n
	}
	asig := make([]int32, n)

	type respuesta struct {
		id int
		p  Parcial
	}
	entradas := make([]chan []Punto, w)
	salida := make(chan respuesta, w)

	// Workers persistentes: cada uno es dueño de la partición [a, b)
	for id := 0; id < w; id++ {
		a, b := id*n/w, (id+1)*n/w
		entradas[id] = make(chan []Punto)
		go func(id int, in <-chan []Punto) {
			for cents := range in {
				salida <- respuesta{id, Asignar(pts[a:b], cents, asig[a:b])}
			}
		}(id, entradas[id])
	}
	defer func() {
		for _, c := range entradas {
			close(c)
		}
	}()

	cents := append([]Punto(nil), ini...)
	k := len(cents)
	res := Resultado{}
	parciales := make([]Parcial, w)
	for it := 1; it <= cfg.MaxIter; it++ {
		// Difundir centroides (broadcast)
		for _, c := range entradas {
			c <- cents
		}
		// Recolectar parciales; se guardan por id para reducir en orden fijo
		for i := 0; i < w; i++ {
			r := <-salida
			parciales[r.id] = r.p
		}
		// REDUCE
		sumas := make([]Punto, k)
		conteos := make([]int, k)
		sse := 0.0
		for _, p := range parciales {
			for j := 0; j < k; j++ {
				sumas[j].X += p.Sumas[j].X
				sumas[j].Y += p.Sumas[j].Y
				conteos[j] += p.Conteos[j]
			}
			sse += p.SSE
		}
		nuevos := make([]Punto, k)
		desplaz := 0.0
		for j := 0; j < k; j++ {
			if conteos[j] == 0 { // cluster vacío: conserva su centroide
				nuevos[j] = cents[j]
				continue
			}
			nuevos[j] = Punto{sumas[j].X / float64(conteos[j]), sumas[j].Y / float64(conteos[j])}
			desplaz = math.Max(desplaz, math.Hypot(nuevos[j].X-cents[j].X, nuevos[j].Y-cents[j].Y))
		}
		res.Iteraciones, res.SSE, res.Tamanos = it, sse, conteos
		cents = nuevos
		if desplaz < cfg.Tol {
			res.Convergio = true
			break
		}
	}
	res.Centroides, res.Asig = cents, asig
	res.Duracion = time.Since(inicio)
	res.MsPorIter = float64(res.Duracion.Microseconds()) / 1000 / float64(res.Iteraciones)
	return res
}

// Inicializar elige k centroides con K-Means++ sobre una muestra aleatoria
// de hasta `muestra` puntos (reproducible con la semilla).
func Inicializar(pts []Punto, k int, muestra int, semilla int64) []Punto {
	rng := rand.New(rand.NewSource(semilla))
	m := pts
	if len(pts) > muestra {
		m = make([]Punto, muestra)
		for i := range m {
			m[i] = pts[rng.Intn(len(pts))]
		}
	}
	cents := []Punto{m[rng.Intn(len(m))]}
	d2 := make([]float64, len(m))
	for i := range d2 {
		d2[i] = math.MaxFloat64
	}
	for len(cents) < k {
		ultimo := cents[len(cents)-1]
		total := 0.0
		for i, p := range m {
			dx, dy := p.X-ultimo.X, p.Y-ultimo.Y
			if d := dx*dx + dy*dy; d < d2[i] {
				d2[i] = d
			}
			total += d2[i]
		}
		r := rng.Float64() * total
		elegido := len(m) - 1
		for i, d := range d2 {
			if r -= d; r <= 0 {
				elegido = i
				break
			}
		}
		cents = append(cents, m[elegido])
	}
	return cents
}

// Codo devuelve el k del "codo" de la curva SSE(k) con el método Kneedle:
// el punto más alejado de la recta que une el primer y el último punto.
func Codo(ks []int, sse []float64) int {
	if len(ks) < 3 {
		return ks[0]
	}
	x0, y0 := float64(ks[0]), sse[0]
	x1, y1 := float64(ks[len(ks)-1]), sse[len(sse)-1]
	mejor, dmax := ks[0], -1.0
	for i := range ks {
		// normalizar a [0,1]
		x := (float64(ks[i]) - x0) / (x1 - x0)
		y := (sse[i] - y1) / (y0 - y1)
		if d := (1 - x) - y; d > dmax { // distancia vertical a la recta y = 1 - x
			mejor, dmax = ks[i], d
		}
	}
	return mejor
}
