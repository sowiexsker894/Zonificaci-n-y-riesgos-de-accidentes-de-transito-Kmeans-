// Package red implementa la comunicación TCP interna entre el coordinador (API)
// y los nodos de cómputo ML. Protocolo: gob sobre TCP, una conexión persistente
// por nodo. El coordinador envía los centroides (broadcast) y cada nodo responde
// con su Parcial (fase MAP de K-Means); el coordinador hace el REDUCE.
package red

import (
	"encoding/csv"
	"encoding/gob"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"siniestros/internal/kmeans"
)

const (
	kmLat = 110.574
	lat0  = -10.0 // latitud de referencia fija para que nodos y coordinador proyecten igual
)

var kmLon = 111.320 * math.Cos(lat0*math.Pi/180)

// Proyectar convierte lat/lon a km planos; Inversa hace el camino contrario.
func Proyectar(lat, lon float64) kmeans.Punto   { return kmeans.Punto{X: lon * kmLon, Y: lat * kmLat} }
func Inversa(p kmeans.Punto) (lat, lon float64) { return p.Y / kmLat, p.X / kmLon }

// CargarPuntos lee latitud,longitud de un CSV (con encabezado) y los proyecta.
func CargarPuntos(ruta string) ([]kmeans.Punto, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	if _, err := r.Read(); err != nil {
		return nil, err
	}
	var pts []kmeans.Punto
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		la, e1 := strconv.ParseFloat(rec[0], 64)
		lo, e2 := strconv.ParseFloat(rec[1], 64)
		if e1 == nil && e2 == nil {
			pts = append(pts, Proyectar(la, lo))
		}
	}
	return pts, nil
}

// Mensajes del protocolo.
type Tarea struct {
	Info  bool
	Cents []kmeans.Punto
}
type Respuesta struct {
	NodoID int
	N      int // puntos de la partición
	P      kmeans.Parcial
}

// Servir atiende a un coordinador: cada Tarea{Cents} se responde con el Parcial
// calculado sobre la partición local del nodo.
func Servir(addr string, id int, pts []kmeans.Punto) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		go func(c net.Conn) {
			defer c.Close()
			dec, enc := gob.NewDecoder(c), gob.NewEncoder(c)
			asig := make([]int32, len(pts)) // buffer propio de esta conexión
			for {
				var t Tarea
				if err := dec.Decode(&t); err != nil {
					return
				}
				resp := Respuesta{NodoID: id, N: len(pts)}
				if !t.Info {
					resp.P = kmeans.Asignar(pts, t.Cents, asig)
				}
				if err := enc.Encode(resp); err != nil {
					return
				}
			}
		}(c)
	}
}

// Estado de un nodo visto por el coordinador.
type EstadoNodo struct {
	Addr   string  `json:"addr"`
	Activo bool    `json:"activo"`
	NodoID int     `json:"nodo_id"`
	Puntos int     `json:"puntos"`
	RTTms  float64 `json:"rtt_ms"`
}

// Sondear consulta a cada nodo (ping + tamaño de partición).
func Sondear(addrs []string) []EstadoNodo {
	out := make([]EstadoNodo, len(addrs))
	var wg sync.WaitGroup
	for i, a := range addrs {
		wg.Add(1)
		go func(i int, a string) {
			defer wg.Done()
			out[i] = EstadoNodo{Addr: a}
			t0 := time.Now()
			c, err := net.DialTimeout("tcp", a, 800*time.Millisecond)
			if err != nil {
				return
			}
			defer c.Close()
			c.SetDeadline(time.Now().Add(time.Second))
			if gob.NewEncoder(c).Encode(Tarea{Info: true}) != nil {
				return
			}
			var r Respuesta
			if gob.NewDecoder(c).Decode(&r) != nil {
				return
			}
			out[i] = EstadoNodo{Addr: a, Activo: true, NodoID: r.NodoID, Puntos: r.N,
				RTTms: float64(time.Since(t0).Microseconds()) / 1000}
		}(i, a)
	}
	wg.Wait()
	return out
}

// Entrenar ejecuta K-Means distribuido: cada iteración difunde los centroides a
// todos los nodos en paralelo (una goroutine por nodo + channel de resultados)
// y reduce los parciales en orden de nodo para que el resultado sea determinista.
func Entrenar(addrs []string, ini []kmeans.Punto, maxIter int, tol float64) (kmeans.Resultado, error) {
	inicio := time.Now()
	type nodo struct {
		enc *gob.Encoder
		dec *gob.Decoder
		c   net.Conn
	}
	nodos := make([]nodo, len(addrs))
	for i, a := range addrs {
		c, err := net.DialTimeout("tcp", a, 2*time.Second)
		if err != nil {
			return kmeans.Resultado{}, fmt.Errorf("nodo %s no disponible: %w", a, err)
		}
		defer c.Close()
		nodos[i] = nodo{gob.NewEncoder(c), gob.NewDecoder(c), c}
	}
	type msg struct {
		i   int
		r   Respuesta
		err error
	}
	cents := append([]kmeans.Punto(nil), ini...)
	k := len(cents)
	res := kmeans.Resultado{}
	for it := 1; it <= maxIter; it++ {
		ch := make(chan msg, len(nodos))
		for i := range nodos {
			go func(i int) {
				n := nodos[i]
				n.c.SetDeadline(time.Now().Add(10 * time.Second))
				if err := n.enc.Encode(Tarea{Cents: cents}); err != nil {
					ch <- msg{i: i, err: err}
					return
				}
				var r Respuesta
				err := n.dec.Decode(&r)
				ch <- msg{i, r, err}
			}(i)
		}
		parc := make([]kmeans.Parcial, len(nodos))
		for range nodos {
			m := <-ch
			if m.err != nil {
				return res, fmt.Errorf("nodo %s: %w", addrs[m.i], m.err)
			}
			parc[m.i] = m.r.P
		}
		sumas, conteos, sse := make([]kmeans.Punto, k), make([]int, k), 0.0
		for _, p := range parc {
			for j := 0; j < k; j++ {
				sumas[j].X += p.Sumas[j].X
				sumas[j].Y += p.Sumas[j].Y
				conteos[j] += p.Conteos[j]
			}
			sse += p.SSE
		}
		nuevos, desplaz := make([]kmeans.Punto, k), 0.0
		for j := 0; j < k; j++ {
			if conteos[j] == 0 {
				nuevos[j] = cents[j]
				continue
			}
			nuevos[j] = kmeans.Punto{X: sumas[j].X / float64(conteos[j]), Y: sumas[j].Y / float64(conteos[j])}
			desplaz = math.Max(desplaz, math.Hypot(nuevos[j].X-cents[j].X, nuevos[j].Y-cents[j].Y))
		}
		res.Iteraciones, res.SSE, res.Tamanos, cents = it, sse, conteos, nuevos
		if desplaz < tol {
			res.Convergio = true
			break
		}
	}
	res.Centroides = cents
	res.Duracion = time.Since(inicio)
	res.MsPorIter = float64(res.Duracion.Microseconds()) / 1000 / float64(res.Iteraciones)
	return res, nil
}
