// Package datos carga el CSV limpio en memoria en formato columnar
// (un slice por columna), leyendo y parseando en paralelo.
package datos

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Dataset guarda solo las columnas que usa K-Means y el perfil de clusters.
type Dataset struct {
	Lat, Lon   []float64
	Hora       []uint8 // 0-23
	Fallecidos []uint16
	Sintetico  []bool
	Depto      []string
	Prov       []string
	Dist       []string
	Clase      []string
	Causa      []string
}

func (d *Dataset) Len() int { return len(d.Lat) }

type indices struct{ lat, lon, hora, fall, sint, depto, prov, dist, clase, causa, total int }

func leerIndices(enc string) (indices, error) {
	pos := map[string]int{}
	cols := strings.Split(enc, ",")
	for i, c := range cols {
		pos[strings.TrimSpace(c)] = i
	}
	var ix indices
	ix.total = len(cols)
	for _, p := range []struct {
		dst  *int
		name string
	}{
		{&ix.lat, "latitud"}, {&ix.lon, "longitud"}, {&ix.hora, "hora"},
		{&ix.fall, "fallecidos"}, {&ix.sint, "es_sintetico"}, {&ix.depto, "departamento"},
		{&ix.prov, "provincia"}, {&ix.dist, "distrito"}, {&ix.clase, "clase"},
		{&ix.causa, "causa_especifica"},
	} {
		i, ok := pos[p.name]
		if !ok {
			return ix, fmt.Errorf("falta la columna %q", p.name)
		}
		*p.dst = i
	}
	return ix, nil
}

// Cargar lee el archivo completo y lo parsea con `workers` goroutines. Cada
// goroutine llena su propio Dataset parcial; al final se concatenan en orden.
func Cargar(ruta string, workers int) (*Dataset, error) {
	buf, err := os.ReadFile(ruta)
	if err != nil {
		return nil, err
	}
	fin := bytes.IndexByte(buf, '\n')
	if fin < 0 {
		return nil, fmt.Errorf("archivo vacío")
	}
	ix, err := leerIndices(strings.TrimRight(string(buf[:fin]), "\r"))
	if err != nil {
		return nil, err
	}
	cuerpo := buf[fin+1:]

	// Partir en `workers` trozos alineados a '\n'
	var trozos [][]byte
	paso := len(cuerpo)/workers + 1
	for len(cuerpo) > 0 {
		corte := paso
		if corte >= len(cuerpo) {
			corte = len(cuerpo)
		} else if j := bytes.IndexByte(cuerpo[corte:], '\n'); j >= 0 {
			corte += j + 1
		} else {
			corte = len(cuerpo)
		}
		trozos = append(trozos, cuerpo[:corte])
		cuerpo = cuerpo[corte:]
	}

	parciales := make([]*Dataset, len(trozos))
	errs := make([]error, len(trozos))
	var wg sync.WaitGroup
	for i, t := range trozos {
		wg.Add(1)
		go func(i int, t []byte) {
			defer wg.Done()
			parciales[i], errs[i] = parsear(t, ix) // cada goroutine escribe SOLO su índice i
		}(i, t)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return nil, e
		}
	}
	return concatenar(parciales), nil
}

func parsear(b []byte, ix indices) (*Dataset, error) {
	d := &Dataset{}
	cache := map[string]string{} // reutiliza strings repetidos (departamentos, clases...)
	intern := func(s string) string {
		if v, ok := cache[s]; ok {
			return v
		}
		cache[s] = s
		return s
	}
	campos := make([]string, ix.total)
	for len(b) > 0 {
		var linea []byte
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			linea, b = b[:i], b[i+1:]
		} else {
			linea, b = b, nil
		}
		linea = bytes.TrimRight(linea, "\r")
		if len(linea) == 0 {
			continue
		}
		s := string(linea)
		n := 0
		for n < ix.total-1 {
			j := strings.IndexByte(s, ',')
			if j < 0 {
				break
			}
			campos[n], s = s[:j], s[j+1:]
			n++
		}
		campos[n] = s
		if n != ix.total-1 {
			return nil, fmt.Errorf("fila con %d columnas", n+1)
		}
		lat, e1 := strconv.ParseFloat(campos[ix.lat], 64)
		lon, e2 := strconv.ParseFloat(campos[ix.lon], 64)
		fall, e3 := strconv.Atoi(campos[ix.fall])
		h, e4 := strconv.Atoi(campos[ix.hora][:2])
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
			return nil, fmt.Errorf("fila inválida en CSV limpio: %q", linea)
		}
		d.Lat = append(d.Lat, lat)
		d.Lon = append(d.Lon, lon)
		d.Hora = append(d.Hora, uint8(h))
		d.Fallecidos = append(d.Fallecidos, uint16(fall))
		d.Sintetico = append(d.Sintetico, campos[ix.sint] == "1")
		d.Depto = append(d.Depto, intern(campos[ix.depto]))
		d.Prov = append(d.Prov, intern(campos[ix.prov]))
		d.Dist = append(d.Dist, intern(campos[ix.dist]))
		d.Clase = append(d.Clase, intern(campos[ix.clase]))
		d.Causa = append(d.Causa, intern(campos[ix.causa]))
	}
	return d, nil
}

func concatenar(ps []*Dataset) *Dataset {
	n := 0
	for _, p := range ps {
		n += p.Len()
	}
	d := &Dataset{
		Lat: make([]float64, 0, n), Lon: make([]float64, 0, n), Hora: make([]uint8, 0, n),
		Fallecidos: make([]uint16, 0, n), Sintetico: make([]bool, 0, n),
		Depto: make([]string, 0, n), Prov: make([]string, 0, n), Dist: make([]string, 0, n),
		Clase: make([]string, 0, n), Causa: make([]string, 0, n),
	}
	for _, p := range ps {
		d.Lat = append(d.Lat, p.Lat...)
		d.Lon = append(d.Lon, p.Lon...)
		d.Hora = append(d.Hora, p.Hora...)
		d.Fallecidos = append(d.Fallecidos, p.Fallecidos...)
		d.Sintetico = append(d.Sintetico, p.Sintetico...)
		d.Depto = append(d.Depto, p.Depto...)
		d.Prov = append(d.Prov, p.Prov...)
		d.Dist = append(d.Dist, p.Dist...)
		d.Clase = append(d.Clase, p.Clase...)
		d.Causa = append(d.Causa, p.Causa...)
	}
	return d
}
