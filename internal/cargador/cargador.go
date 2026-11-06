// Package cargador implementa la Parte A: lectura y limpieza concurrente del
// CSV de siniestros.
//
// Patrón usado (pipeline + worker pool + fan-out por hash + fan-in):
//
//	[dividir]  el archivo se parte en bloques de bytes alineados a fin de línea
//	   │ chan bloque
//	   ▼
//	[worker 1..N]  cada goroutine lee SU bloque con ReadAt (seguro en paralelo),
//	   │           limpia sus filas y lleva sus propios contadores (sin mutex)
//	   │ chan []filaLimpia  (repartidas por hash(codigo) % S)
//	   ▼
//	[shard 1..S]   cada shard tiene SU mapa de códigos vistos y elimina
//	   │           duplicados: un código siempre llega al mismo shard
//	   │ chan []string
//	   ▼
//	[escritor]  una sola goroutine escribe el CSV limpio -> sin escrituras
//	            compartidas. Los contadores se suman al final.
package cargador

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"siniestros/internal/limpieza"
)

type bloque struct {
	ini, fin int64
}

type filaLimpia struct {
	codigo string
	linea  string
}

// hashShard: FNV-1a de 32 bits sobre el código.
func hashShard(s string, n int) int {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return int(h % uint32(n))
}

// Metricas resume una ejecución.
type Metricas struct {
	Workers     int     `json:"workers"`
	Bloques     int     `json:"bloques"`
	DuracionMs  float64 `json:"duracion_ms"`
	FilasPorSeg float64 `json:"filas_por_segundo"`
}

// Procesar lee `ruta` con `workers` goroutines y escribe el CSV limpio en w
// (usar io.Discard para solo medir).
func Procesar(ruta string, workers int, tamBloque int64, w io.Writer) (*limpieza.Stats, Metricas, error) {
	inicio := time.Now()
	f, err := os.Open(ruta)
	if err != nil {
		return nil, Metricas{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, Metricas{}, err
	}

	encabezado, err := bufio.NewReader(f).ReadString('\n')
	if err != nil {
		return nil, Metricas{}, fmt.Errorf("leyendo encabezado: %w", err)
	}
	encabezado = strings.TrimRight(encabezado, "\r\n")
	cols, err := limpieza.NuevasColumnas(strings.Split(encabezado, ","))
	if err != nil {
		return nil, Metricas{}, err
	}

	bloques, err := dividir(f, int64(len(encabezado))+1, info.Size(), tamBloque)
	if err != nil {
		return nil, Metricas{}, err
	}
	if encabezadoCRLF(f, int64(len(encabezado))) {
		bloques[0].ini++ // saltar el '\n' de un fin de línea \r\n
	}

	// Etapa 1: productor de bloques
	jobs := make(chan bloque, len(bloques))
	for _, b := range bloques {
		jobs <- b
	}
	close(jobs)

	shards := workers
	statsCh := make(chan *limpieza.Stats, workers+shards)
	shardIn := make([]chan []filaLimpia, shards)
	for s := range shardIn {
		shardIn[s] = make(chan []filaLimpia, 4)
	}
	escritura := make(chan []string, shards*2)

	// Etapa 2: worker pool (leer + limpiar + repartir por hash del código)
	var wgW sync.WaitGroup
	for i := 0; i < workers; i++ {
		wgW.Add(1)
		go func() {
			defer wgW.Done()
			st := limpieza.NuevasStats() // contadores locales: sin mutex
			for b := range jobs {
				filas := procesarBloque(f, b, cols, st)
				porShard := make([][]filaLimpia, shards)
				for _, fl := range filas {
					s := hashShard(fl.codigo, shards)
					porShard[s] = append(porShard[s], fl)
				}
				for s, lote := range porShard {
					if len(lote) > 0 {
						shardIn[s] <- lote
					}
				}
			}
			statsCh <- st
		}()
	}
	go func() {
		wgW.Wait()
		for _, c := range shardIn {
			close(c)
		}
	}()

	// Etapa 3: deduplicación por shards. Un mismo código siempre cae en el
	// mismo shard, así que cada mapa es privado de su goroutine.
	var wgS sync.WaitGroup
	for s := 0; s < shards; s++ {
		wgS.Add(1)
		go func(in <-chan []filaLimpia) {
			defer wgS.Done()
			st := limpieza.NuevasStats()
			vistos := make(map[string]struct{}, 1_300_000/shards)
			for lote := range in {
				lineas := make([]string, 0, len(lote))
				for _, fl := range lote {
					if _, dup := vistos[fl.codigo]; dup {
						st.Descartes[limpieza.DescDuplicado]++
						st.Validas--
						continue
					}
					vistos[fl.codigo] = struct{}{}
					lineas = append(lineas, fl.linea)
				}
				escritura <- lineas
			}
			statsCh <- st
		}(shardIn[s])
	}
	go func() { wgS.Wait(); close(escritura) }()

	// Etapa 4: escritor único (fan-in) -> el archivo nunca se escribe en paralelo
	bw := bufio.NewWriterSize(w, 1<<20)
	bw.WriteString(encabezado + "\n")
	for lineas := range escritura {
		for _, l := range lineas {
			bw.WriteString(l)
			bw.WriteByte('\n')
		}
	}
	if err := bw.Flush(); err != nil {
		return nil, Metricas{}, err
	}
	close(statsCh)
	total := limpieza.NuevasStats()
	for st := range statsCh {
		total.Sumar(st)
	}

	d := time.Since(inicio)
	return total, Metricas{
		Workers:     workers,
		Bloques:     len(bloques),
		DuracionMs:  float64(d.Microseconds()) / 1000,
		FilasPorSeg: float64(total.Leidas) / d.Seconds(),
	}, nil
}

func encabezadoCRLF(f *os.File, n int64) bool {
	b := make([]byte, 1)
	_, err := f.ReadAt(b, n)
	return err == nil && b[0] == '\n'
}

// dividir parte [ini, tam) en bloques de ~tamBloque bytes, moviendo cada
// corte hasta el siguiente '\n' para no partir una fila en dos.
func dividir(f *os.File, ini, tam, tamBloque int64) ([]bloque, error) {
	var bs []bloque
	buf := make([]byte, 4096)
	for ini < tam {
		fin := ini + tamBloque
		if fin >= tam {
			bs = append(bs, bloque{ini, tam})
			break
		}
		for {
			n, err := f.ReadAt(buf, fin)
			if i := bytes.IndexByte(buf[:n], '\n'); i >= 0 {
				fin += int64(i) + 1
				break
			}
			fin += int64(n)
			if err != nil { // EOF
				fin = tam
				break
			}
		}
		bs = append(bs, bloque{ini, fin})
		ini = fin
	}
	return bs, nil
}

func procesarBloque(f *os.File, b bloque, cols limpieza.Columnas, st *limpieza.Stats) []filaLimpia {
	buf := make([]byte, b.fin-b.ini)
	if _, err := f.ReadAt(buf, b.ini); err != nil && err != io.EOF {
		st.Descartes["error_lectura"]++
		return nil
	}
	filas := make([]filaLimpia, 0, len(buf)/200)
	for len(buf) > 0 {
		var linea []byte
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			linea, buf = buf[:i], buf[i+1:]
		} else {
			linea, buf = buf, nil
		}
		linea = bytes.TrimRight(linea, "\r")
		if len(linea) == 0 {
			continue
		}
		campos := strings.Split(string(linea), ",")
		if limpieza.Limpiar(campos, cols, st) == "" {
			filas = append(filas, filaLimpia{campos[cols.Codigo], strings.Join(campos, ",")})
		}
	}
	return filas
}
