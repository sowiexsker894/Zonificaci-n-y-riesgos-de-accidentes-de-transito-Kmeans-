// Package limpieza contiene las reglas de validación y corrección de cada
// fila del dataset de siniestros. Las funciones son puras respecto al estado
// global: cada worker usa su propio *Stats, por lo que no hay condiciones de
// carrera y no se necesitan mutex.
package limpieza

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Límites aproximados del territorio peruano.
const (
	LatMin, LatMax = -18.40, -0.03
	LonMin, LonMax = -81.40, -68.65

	FallecidosMin, FallecidosMax = 1, 100
)

// Motivos de descarte (la fila no se puede usar).
const (
	DescColumnas   = "columnas_incompletas"
	DescCoordTexto = "coordenada_vacia_o_texto"
	DescCoordCero  = "coordenada_cero"
	DescCoordFuera = "coordenada_fuera_de_peru"
	DescHora       = "hora_invalida"
	DescFecha      = "fecha_invalida"
	DescFallecidos = "fallecidos_fuera_de_rango"
	DescDuplicado  = "duplicado"
)

// Tipos de corrección (la fila se arregla y se conserva).
const (
	CorrTexto          = "texto_normalizado"
	CorrCoordInvertida = "coordenadas_invertidas"
	CorrHora           = "hora_reformateada"
	CorrFecha          = "fecha_reformateada"
	CorrFallecidos     = "fallecidos_imputado"
	CorrDepartamento   = "departamento_imputado"
	CorrClase          = "clase_imputada"
)

// Stats acumula contadores. Cada worker tiene el suyo y el colector los suma.
type Stats struct {
	Leidas       int            `json:"leidas"`
	Validas      int            `json:"validas"`
	Descartes    map[string]int `json:"descartes"`
	Correcciones map[string]int `json:"correcciones"`
}

func NuevasStats() *Stats {
	return &Stats{Descartes: map[string]int{}, Correcciones: map[string]int{}}
}

// Sumar agrega los contadores de o en s.
func (s *Stats) Sumar(o *Stats) {
	s.Leidas += o.Leidas
	s.Validas += o.Validas
	for k, v := range o.Descartes {
		s.Descartes[k] += v
	}
	for k, v := range o.Correcciones {
		s.Correcciones[k] += v
	}
}

// Columnas guarda la posición de cada campo según el encabezado del CSV.
type Columnas struct {
	Total                                                   int
	Codigo, Fecha, Hora, Lat, Lon, Fallecidos, Depto, Clase int
	Texto                                                   []int
}

func NuevasColumnas(encabezado []string) (Columnas, error) {
	pos := map[string]int{}
	for i, h := range encabezado {
		pos[strings.TrimSpace(h)] = i
	}
	get := func(n string) (int, error) {
		i, ok := pos[n]
		if !ok {
			return 0, fmt.Errorf("falta la columna %q", n)
		}
		return i, nil
	}
	var c Columnas
	var err error
	c.Total = len(encabezado)
	campos := []struct {
		dst  *int
		name string
	}{
		{&c.Codigo, "codigo"}, {&c.Fecha, "fecha"}, {&c.Hora, "hora"},
		{&c.Lat, "latitud"}, {&c.Lon, "longitud"}, {&c.Fallecidos, "fallecidos"},
		{&c.Depto, "departamento"}, {&c.Clase, "clase"},
	}
	for _, f := range campos {
		if *f.dst, err = get(f.name); err != nil {
			return c, err
		}
	}
	for _, n := range []string{"departamento", "provincia", "distrito", "clase", "clima",
		"zona", "tipo_via", "causa_principal", "causa_especifica"} {
		if i, ok := pos[n]; ok {
			c.Texto = append(c.Texto, i)
		}
	}
	return c, nil
}

func enPeru(lat, lon float64) bool {
	return lat >= LatMin && lat <= LatMax && lon >= LonMin && lon <= LonMax
}

// Limpiar valida y corrige la fila f en el mismo slice. Devuelve "" si la
// fila es válida, o el motivo de descarte.
func Limpiar(f []string, c Columnas, st *Stats) string {
	st.Leidas++
	if len(f) != c.Total {
		return descartar(st, DescColumnas)
	}

	// 1. Texto: espacios extra y minúsculas
	corrTexto := false
	for _, i := range c.Texto {
		n := strings.ToUpper(strings.TrimSpace(f[i]))
		if n != f[i] {
			f[i] = n
			corrTexto = true
		}
	}
	if corrTexto {
		st.Correcciones[CorrTexto]++
	}

	// 2. Coordenadas
	lat, e1 := strconv.ParseFloat(strings.TrimSpace(f[c.Lat]), 64)
	lon, e2 := strconv.ParseFloat(strings.TrimSpace(f[c.Lon]), 64)
	switch {
	case e1 != nil || e2 != nil:
		return descartar(st, DescCoordTexto)
	case lat == 0 && lon == 0:
		return descartar(st, DescCoordCero)
	case enPeru(lat, lon):
	case enPeru(lon, lat):
		lat, lon = lon, lat
		st.Correcciones[CorrCoordInvertida]++
	default:
		return descartar(st, DescCoordFuera)
	}
	f[c.Lat] = strconv.FormatFloat(lat, 'f', 6, 64)
	f[c.Lon] = strconv.FormatFloat(lon, 'f', 6, 64)

	// 3. Hora -> HH:MM
	h, ok := NormalizarHora(f[c.Hora])
	if !ok {
		return descartar(st, DescHora)
	}
	if h != f[c.Hora] {
		f[c.Hora] = h
		st.Correcciones[CorrHora]++
	}

	// 4. Fecha -> AAAA-MM-DD
	fe, ok := NormalizarFecha(f[c.Fecha])
	if !ok {
		return descartar(st, DescFecha)
	}
	if fe != f[c.Fecha] {
		f[c.Fecha] = fe
		st.Correcciones[CorrFecha]++
	}

	// 5. Fallecidos: vacío -> 1 (todo registro es un siniestro fatal)
	if strings.TrimSpace(f[c.Fallecidos]) == "" {
		f[c.Fallecidos] = "1"
		st.Correcciones[CorrFallecidos]++
	} else if n, err := strconv.Atoi(strings.TrimSpace(f[c.Fallecidos])); err != nil ||
		n < FallecidosMin || n > FallecidosMax {
		return descartar(st, DescFallecidos)
	}

	// 6. Categóricas vacías
	if f[c.Depto] == "" {
		f[c.Depto] = "DESCONOCIDO"
		st.Correcciones[CorrDepartamento]++
	}
	if f[c.Clase] == "" {
		f[c.Clase] = "NO ESPECIFICADO"
		st.Correcciones[CorrClase]++
	}

	st.Validas++
	return ""
}

func descartar(st *Stats, motivo string) string {
	st.Descartes[motivo]++
	return motivo
}

// NormalizarHora acepta "H:M", "HH:MM" o "H.MM" y devuelve "HH:MM".
func NormalizarHora(s string) (string, bool) {
	s = strings.TrimSpace(s)
	sep := strings.IndexAny(s, ":.")
	if sep < 1 {
		return "", false
	}
	h, e1 := strconv.Atoi(s[:sep])
	m, e2 := strconv.Atoi(s[sep+1:])
	if e1 != nil || e2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return "", false
	}
	return fmt.Sprintf("%02d:%02d", h, m), true
}

var formatosFecha = []string{"2006-01-02", "02/01/2006", "2006/01/02"}

// NormalizarFecha acepta AAAA-MM-DD, DD/MM/AAAA o AAAA/MM/DD (años 2021-2025).
func NormalizarFecha(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for _, l := range formatosFecha {
		if t, err := time.Parse(l, s); err == nil {
			if t.Year() < 2021 || t.Year() > 2025 {
				return "", false
			}
			return t.Format("2006-01-02"), true
		}
	}
	return "", false
}
