package kmeans

import (
	"math"
	"math/rand"
	"sort"
	"testing"
)

// Tres grupos bien separados: el algoritmo debe encontrar sus centros.
func grupos(n int) ([]Punto, []Punto) {
	rng := rand.New(rand.NewSource(1))
	centros := []Punto{{0, 0}, {100, 0}, {50, 80}}
	var pts []Punto
	for i := 0; i < n; i++ {
		c := centros[i%3]
		pts = append(pts, Punto{c.X + rng.NormFloat64(), c.Y + rng.NormFloat64()})
	}
	return pts, centros
}

func ordenar(ps []Punto) {
	sort.Slice(ps, func(i, j int) bool { return ps[i].X*1000+ps[i].Y < ps[j].X*1000+ps[j].Y })
}

func TestRecuperaCentros(t *testing.T) {
	pts, centros := grupos(30000)
	r := Entrenar(pts, Inicializar(pts, 3, 5000, 7), Config{K: 3, Workers: 4, MaxIter: 100, Tol: 1e-6})
	if !r.Convergio {
		t.Fatal("no convergió")
	}
	ordenar(r.Centroides)
	ordenar(centros)
	for i := range centros {
		if d := math.Hypot(r.Centroides[i].X-centros[i].X, r.Centroides[i].Y-centros[i].Y); d > 0.1 {
			t.Errorf("centroide %d a %.3f del real", i, d)
		}
	}
}

// El resultado paralelo debe ser igual al secuencial (1 worker).
func TestParaleloIgualASecuencial(t *testing.T) {
	pts, _ := grupos(20000)
	ini := Inicializar(pts, 5, 5000, 3)
	seq := Entrenar(pts, ini, Config{K: 5, Workers: 1, MaxIter: 50, Tol: 1e-9})
	for _, w := range []int{2, 3, 8, 16} {
		par := Entrenar(pts, ini, Config{K: 5, Workers: w, MaxIter: 50, Tol: 1e-9})
		if par.Iteraciones != seq.Iteraciones {
			t.Errorf("w=%d iteraciones %d vs %d", w, par.Iteraciones, seq.Iteraciones)
		}
		if math.Abs(par.SSE-seq.SSE)/seq.SSE > 1e-9 {
			t.Errorf("w=%d SSE %.6f vs %.6f", w, par.SSE, seq.SSE)
		}
		for i := range par.Asig {
			if par.Asig[i] != seq.Asig[i] {
				t.Fatalf("w=%d asignación distinta en el punto %d", w, i)
			}
		}
	}
}

func TestCodo(t *testing.T) {
	ks := []int{1, 2, 3, 4, 5, 6, 7, 8}
	sse := []float64{1000, 500, 120, 100, 90, 85, 80, 78}
	if k := Codo(ks, sse); k != 3 {
		t.Errorf("codo = %d, esperado 3", k)
	}
}
