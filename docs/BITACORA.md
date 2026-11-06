# Bitácora del proyecto — Zonificación y riesgo de accidentes de tránsito (K-Means paralelo)

> Registro de cambios, decisiones, observaciones e incidencias en orden cronológico.
> Sirve como base para redactar el informe final (PC3 / TB2).
> Cada sección indica a qué parte del informe alimenta → **[Informe: sección]**.

---

## 0. Contexto

| Ítem | Detalle |
|---|---|
| Curso / entrega | 1ACC0065 — PC3 (entrega 07/11/2026 23:30) |
| Caso | N.° 3: Zonificación y riesgo de accidentes de tránsito — K-Means paralelo |
| Lenguaje exigido | Go (goroutines, channels; luego TCP entre nodos) |
| Requisito de volumen | > 1,000,000 de registros |
| Equipo de pruebas | Windows 11 Pro, 12 CPUs lógicas · WSL2 Ubuntu 24.04 (mismas 12 CPUs) · Go 1.27.1 |

### Pregunta de impacto social **[Informe: d) Descripción del problema y motivación]**

> **¿En qué zonas y horas se concentran los siniestros de tránsito fatales en el Perú, para priorizar el patrullaje preventivo y la ubicación de ambulancias?**

Se eligió porque responde a una necesidad real de PNP, SAMU y municipalidades: los recursos son limitados y deben ir donde se producen las muertes.

---

## 1. Dataset original — 06/11/2026 **[Informe: Limpieza y análisis de datos]**

- **Fuente:** Observatorio Nacional de Seguridad Vial (ONSV) – MTC. Información registrada por las UPIAT-PNP.
- **Archivo:** `BBDD ONSV - SINIESTROS FATALES 2021-2025 (preliminar).xlsx`, hoja `SINIESTROS`.
- **Volumen:** 9,106 registros × 27 columnas. Periodo 01/01/2021 – 30/12/2025 (2024 y 2025 son preliminares).
- **Columnas clave para K-Means:** `COORDENADAS LATITUD`, `COORDENADAS LONGITUD`, `HORA SINIESTRO`, `FECHA SINIESTRO`, más atributos descriptivos (departamento, clase, causa, clima, fallecidos).

### Observaciones del dataset original
1. **Volumen insuficiente:** 9,106 filas frente al millón exigido (unas 110 veces menos). → Se justifica el *data augmentation*.
2. Los encabezados reales están en la **fila 5** del Excel; las filas 1 a 4 son título y notas.
3. **Nulos reales** (no inyectados):

   | Columna | Nulos | % |
   |---|---:|---:|
   | Clasificación señal vertical N.° 2 | 8,572 | 94 % |
   | Clasificación señal vertical N.° 1 | 7,790 | 86 % |
   | ¿Existe señal vertical? / horizontal | 7,090 | 78 % |
   | Cantidad de vehículos dañados | 2,813 | 31 % |
   | Perfil longitudinal / zonificación / características de vía | ~423 | 5 % |

   → Las columnas de señalización no se usan como variables del modelo por exceso de nulos. En vehículos dañados, el vacío se interpretó como 0.
4. **1 registro sin longitud**, que se descartó.
5. **2 fechas** guardadas por Excel como tipo fecha y no como texto; se unificaron.
6. Las coordenadas reales caen dentro del Perú: latitud −18.31 a −3.48, longitud −81.29 a −68.95.
7. Fallecidos por siniestro: mínimo 1, máximo 33, media 1.19. Todo registro es fatal, así que siempre hay ≥ 1.
8. No había duplicados en el dataset original.
9. Clases más frecuentes: choque (2,875), despiste (2,570), atropello (1,871), atropello con fuga (885), volcadura (313).

---

## 2. Paso 1 — Preparación de la data base

- **Script:** `scripts/01_preparar_base.py` → `data/siniestros_base.csv`
- **Acciones:**
  - Lectura desde la fila 5 y columnas renombradas a *snake_case* sin tildes (`latitud`, `hora`, …), más fáciles de leer desde Go.
  - Textos con `strip()` y en mayúsculas.
  - Fecha unificada a `AAAA-MM-DD` y hora a `HH:MM`.
  - Validación de coordenadas numéricas dentro del *bounding box* del Perú (lat −18.40 a −0.03; lon −81.40 a −68.65).
- **Resultado:** 9,106 → **9,105** filas (1 descartada por no tener longitud).
- **Bug encontrado y corregido:** el contador de filas descartadas mostraba 0. El valor nulo de longitud producía `<NA>` en la máscara booleana y pandas lo excluía de ambos lados. Se corrigió con `fillna(False)`.

---

## 3. Paso 2 — Data augmentation **[Informe: Limpieza y análisis de datos]**

- **Técnica:** **inyección de ruido / *jittering*** (data augmentation para datos tabulares y geoespaciales).
- **Script:** `scripts/02_augmentation.py` (semilla 42, reproducible) → `data/siniestros_aug.csv`
- **Cómo funciona:** cada siniestro real se replica **131 veces**, y en cada réplica:

  | Variable | Perturbación | Justificación |
  |---|---|---|
  | Latitud / longitud | Ruido gaussiano, σ = 0.0009° (~100 m) | El punto negro se mantiene en la misma zona; no se crean zonas falsas |
  | Hora | Ruido gaussiano, σ = 15 min, acotada a 00:00–23:59 | Conserva la franja horaria del riesgo |
  | Fecha | Desplazamiento uniforme de −7 a +7 días, dentro de 2021–2025 | Conserva la estacionalidad |
  | Resto de atributos | Sin cambios | Clase, causa, clima, etc. siguen siendo coherentes |

- Se añadió la columna **`es_sintetico`** (0 = real, 1 = generado) para separar la data real de la aumentada en el análisis.
- El código de cada réplica es `<código original>-S<n>`, por ejemplo `A-2022-03-159-S21`.
- Las filas se mezclan al final, así el archivo no queda ordenado por original.
- **Resultado:** 9,105 reales + 1,192,755 sintéticas = **1,201,860 filas** (~292 MB). Tarda unos 25 s.
- **Técnica descartada: SMOTE.** Sirve para balancear clases en clasificación supervisada y K-Means no tiene clases, así que no aplica.
- **Limitación que hay que declarar en el informe:** la data aumentada amplifica la distribución espacial real, pero **no aporta información nueva**. El volumen es para demostrar el rendimiento paralelo; las conclusiones sobre zonas deben validarse con la data real (`es_sintetico = 0`).

---

## 4. Paso 3 — Ensuciado intencional de la data **[Informe: Limpieza y análisis de datos]**

- **Técnica:** **inyección de errores / *dirty data injection*** (también llamada *data corruption*). Se meten errores controlados y realistas para probar que el cargador detecta y corrige problemas de calidad.
- **Script:** `scripts/03_ensuciar.py` (semilla 7) → `data/siniestros_sucio.csv` + `data/reporte_ensuciado.md`
- **Resultado:** 1,201,860 → **1,213,878 filas** (por los duplicados agregados).

| # | Tipo de error inyectado | Filas | % | Cómo se inyectó |
|---|---|---:|---:|---|
| 1 | Nulos (celdas vacías) | 36,055 | 3.00 % | En latitud, longitud, hora, departamento, clase o fallecidos (~6,000 c/u) |
| 2 | Coordenadas inválidas | 12,018 | 1.00 % | ¼ en (0,0) · ¼ con latitud fuera del Perú (10° a 60°) · ¼ con lat/lon invertidas · ¼ con el texto "NO REGISTRA" |
| 3 | Hora mal formateada | 12,018 | 1.00 % | `25:70`, `99:99`, `SIN DATO` (irrecuperables) · `4.40`, `4:5` (recuperables) |
| 4 | Fecha mal formateada | 6,009 | 0.50 % | `dd/mm/aaaa`, `aaaa/mm/dd` (recuperables) · `00/00/0000` (irrecuperable) |
| 5 | Texto inconsistente | 24,037 | 2.00 % | Minúsculas o espacios extra en departamento, provincia, distrito, clase, clima |
| 6 | Fallecidos imposibles | 6,009 | 0.50 % | `-1`, `-3`, `150`, `999` |
| 7 | Filas duplicadas | 12,018 | 1.00 % | Copia exacta de filas existentes |
| | **Total** | **108,164** | **9.00 %** | |

---

## 5. Paso 4 — Parte A: cargador concurrente en Go **[Informe: Paralelización del cálculo / Evidencias de implementación]**

### Archivos
| Archivo | Contenido |
|---|---|
| `go.mod` | Módulo `siniestros` |
| `internal/limpieza/limpieza.go` | Reglas de validación y corrección de una fila (funciones puras) |
| `internal/limpieza/limpieza_test.go` | 16 casos de prueba, uno por cada tipo de error, más la normalización de hora |
| `internal/cargador/cargador.go` | Pipeline concurrente |
| `cmd/loader/main.go` | Ejecutable con modo benchmark |

### Ejecución
```bash
go test ./...                                   # pruebas unitarias
go run ./cmd/loader                             # limpia con todos los núcleos
go run ./cmd/loader -bench 1,2,4,8,12 -rep 3    # benchmark secuencial vs paralelo
# WSL: compilar en Windows y ejecutar en Ubuntu
GOOS=linux GOARCH=amd64 go build -o bin/loader-linux ./cmd/loader
wsl ./bin/loader-linux -bench 1,2,4,8,12 -rep 3 -salida /dev/null
```

### Diseño del paralelismo (patrones)
```
[dividir]       archivo → bloques de 4 MB alineados a '\n'  (71 bloques)
   │ chan bloque
   ▼
[worker 1..N]   ReadAt de SU bloque (seguro en paralelo) → limpiar filas
   │            contadores locales por worker (sin mutex)
   │ chan []filaLimpia   repartidas por hash FNV-1a(código) % S
   ▼
[shard 1..S]    cada shard tiene SU mapa de códigos vistos → elimina duplicados
   │ chan []string
   ▼
[escritor]      única goroutine que escribe el CSV limpio
```
- **Patrones:** *pipeline*, *worker pool*, *fan-out* por hash (*sharding*), *fan-in*. Sincronización con `sync.WaitGroup` y cierre ordenado de canales.
- **Cómo se evitan las condiciones de carrera** (punto clave del curso):
  1. Cada worker lee un rango de bytes distinto con `ReadAt`, que es seguro de usar en paralelo sobre el mismo archivo.
  2. Cada worker y cada shard tienen sus **propios** contadores (`Stats`), que se suman al final por un canal. Ningún mapa se comparte.
  3. La eliminación de duplicados usa *sharding*: un mismo código siempre va al mismo shard, así que cada mapa lo usa una sola goroutine. No hace falta `sync.Mutex` ni `sync.Map`.
  4. Una sola goroutine escribe el archivo de salida.

### Reglas de limpieza
| Problema | Acción | Tipo |
|---|---|---|
| Columnas incompletas | Descartar | Descarte |
| Coordenada vacía o texto | Descartar | Descarte |
| Coordenada (0,0) | Descartar | Descarte |
| Coordenada fuera del Perú | Descartar | Descarte |
| Lat/lon invertidas | Intercambiar si así caen dentro del Perú | Corrección |
| Hora `4.40`, `4:5` | Reformatear a `HH:MM` | Corrección |
| Hora `25:70`, `99:99`, `SIN DATO`, vacía | Descartar | Descarte |
| Fecha `dd/mm/aaaa`, `aaaa/mm/dd` | Reformatear a `AAAA-MM-DD` | Corrección |
| Fecha `00/00/0000` o fuera de 2021–2025 | Descartar | Descarte |
| Texto en minúsculas o con espacios | `TrimSpace` + `ToUpper` | Corrección |
| Fallecidos vacío | Imputar **1** (todo registro es fatal) | Corrección |
| Fallecidos < 1 o > 100 | Descartar | Descarte |
| Departamento vacío | `DESCONOCIDO` | Corrección |
| Clase vacía | `NO ESPECIFICADO` | Corrección |
| Código repetido | Descartar (sharding) | Descarte |

### Resultado de la limpieza (idéntico en Windows y WSL)
- Filas leídas: **1,213,878** → filas válidas: **1,160,337 (95.59 %)** → `data/siniestros_limpio.csv`

| Descarte | Filas | | Corrección | Filas |
|---|---:|---|---|---:|
| coordenada_vacia_o_texto | 14,965 | | texto_normalizado | 24,112 |
| hora_invalida | 13,076 | | clase_imputada | 6,057 |
| duplicado | 11,616 | | fallecidos_imputado | 6,041 |
| fallecidos_fuera_de_rango | 5,888 | | departamento_imputado | 5,846 |
| coordenada_cero | 3,035 | | hora_reformateada | 4,807 |
| coordenada_fuera_de_peru | 3,025 | | fecha_reformateada | 3,933 |
| fecha_invalida | 1,936 | | coordenadas_invertidas | 3,001 |

### Contraste: lo inyectado frente a lo detectado
- **Invertidas:** se inyectaron ~3,004 y se corrigieron 3,001. La diferencia son filas que también tenían otro error.
- **Duplicados:** se inyectaron 12,018 y se detectaron 11,616. El resto ya se había descartado por otro motivo antes de llegar a la deduplicación.
- **Fechas:** de 6,009 fechas mal formateadas, 3,933 se recuperaron y 1,936 se descartaron (`00/00/0000`).
- **Horas:** de 12,018 horas mal formateadas, 4,807 se recuperaron (`4.40`, `4:5`). El resto se descartó junto con las horas nulas.
- **Texto:** se corrigieron 24,112 filas, un poco más de las 24,037 inyectadas, porque algunas filas duplicadas también tenían texto sucio.
- **Conclusión:** el cargador detecta el 100 % de los tipos de error inyectados. Recupera los corregibles en vez de perderlos, y así conserva el 95.6 % de la data.

### Benchmark (mediana de 3 repeticiones, archivo ya en caché, GOGC = 400)
| Workers | Windows (ms) | Speedup | Eficiencia | WSL Ubuntu (ms) | Speedup | Eficiencia |
|---:|---:|---:|---:|---:|---:|---:|
| 1 | 5,062 | 1.00x | 100 % | 7,839 | 1.00x | 100 % |
| 2 | 2,618 | 1.93x | 97 % | 4,241 | 1.85x | 92 % |
| 4 | 1,761 | 2.87x | 72 % | 2,373 | 3.30x | 83 % |
| 8 | 1,641 | 3.09x | 39 % | 1,740 | **4.51x** | 56 % |
| 12 | 2,048 | 2.47x | 21 % | 2,208 | 3.55x | 30 % |

Métricas completas: `data/metricas_loader.json` (Windows) y `data/metricas_loader_wsl.json` (WSL).

### Observaciones del rendimiento
1. **Primera versión (colector único):** el speedup se estancaba en ~2.6x. Un solo hilo deduplicaba 1.2 M códigos en un mapa, y esa parte secuencial limitaba la aceleración (ley de Amdahl).
2. **Ajuste del GC:** con `GOGC=400` el recolector de basura corre menos veces y se ganó cerca de 1.4x en 12 workers. Hay que tenerlo en cuenta porque el parseo de texto genera muchas asignaciones de memoria.
3. **Segunda versión (deduplicación por shards):** se paralelizó también la deduplicación. En WSL el speedup llegó a 4.51x con 8 workers.
4. **Con 12 workers el rendimiento baja.** Las 12 CPUs son lógicas (hyper-threading), así que a partir de ~8 los hilos compiten por los mismos núcleos físicos y por el ancho de banda de memoria y de disco.
5. **WSL es más lento con 1 worker** porque lee el archivo desde `/mnt/c`, el sistema de archivos de Windows montado en Linux, que tiene más latencia. Aun así **escala mejor** que en Windows.
6. Throughput máximo: ~760,000 filas/s en Windows y ~700,000 filas/s en WSL.

### 5.1 Cantidad de goroutines elegida y su justificación **[Informe: Paralelización del cálculo]**

**Decisión: 8 workers** (flag `-workers`, por defecto `min(8, NumCPU)`). Además se lanzan 8 shards de deduplicación (uno por worker), 1 escritor (la goroutine principal) y 2 goroutines auxiliares que cierran canales: **19 goroutines en total**.

**Causa de la decisión: evidencia experimental más la arquitectura del procesador.**

1. **El benchmark lo muestra:** 8 workers dio el menor tiempo en Windows (1.64 s) y en WSL (1.74 s, speedup 4.51x). Con 12 workers el tiempo **empeoró** (2.05 s y 2.21 s).
2. **El procesador es híbrido:** Intel Core i7-1265U (12.ª gen., serie U de 15 W) con **10 núcleos físicos y 12 hilos**:
   - 2 núcleos de rendimiento (P-cores) con hyper-threading → 4 hilos;
   - 8 núcleos de eficiencia (E-cores) → 8 hilos.

   `runtime.NumCPU()` devuelve 12, pero esos 12 hilos no rinden igual: dos pares comparten un P-core y los E-cores son más lentos.
3. **Por qué 12 rinde peor que 8:**
   - Con 12 workers más 12 shards hay 24 goroutines activas sobre 12 hilos. Los shards, el escritor y el recolector de basura de Go compiten con los workers por CPU.
   - Los hilos de hyper-threading comparten las unidades de cálculo del mismo núcleo.
   - En un portátil serie U, al cargar todos los núcleos baja la frecuencia turbo por límite de potencia y temperatura.
   - Con 8 workers quedan hilos libres para los shards, el escritor y el GC.
4. **De 4 a 8 la ganancia es pequeña** (en Windows, 1.76 s → 1.64 s): la parte secuencial (escritura, ley de Amdahl) y el ancho de banda de memoria empiezan a limitar.
5. **Tamaño de bloque:** 4 MB producen 71 bloques, unas 9 tareas por worker. Así un worker que termina antes toma otro bloque y la carga se equilibra entre P-cores y E-cores, que van a velocidades distintas.

**Respuesta corta para el profesor:** *"No usamos el número de hilos lógicos (12) a ciegas: medimos 1, 2, 4, 8 y 12 workers en Windows y Linux, y 8 fue el óptimo en ambos. La razón es que el i7-1265U es híbrido (2 P-cores con HT + 8 E-cores). Pasar de 8 satura los hilos, porque los shards, el escritor y el GC también necesitan CPU, y el hyper-threading y el límite térmico del portátil no aportan más rendimiento real."*

---

## 6. Incidencias técnicas
| Fecha | Incidencia | Solución |
|---|---|---|
| 06/11 | GitHub no acepta archivos > 100 MB y los CSV pesan ~290 MB | `.gitignore` para los CSV grandes y `bin/`. Los CSV se regeneran con los scripts (semillas fijas) |
| 06/11 | Go no está instalado en WSL | Compilación cruzada desde Windows (`GOOS=linux`); el binario corre en Ubuntu sin instalar Go |
| 06/11 | El CSV generado en Windows podía terminar líneas en `\r\n` | El cargador elimina `\r` al final de cada línea y maneja el encabezado con CRLF |
| 06/11 | **Disco C: lleno (10 MB libres de 465 GB)**: falló la compilación | Se borró `siniestros_aug.csv`, que es intermedio y se regenera con el script 02. **Pendiente: liberar espacio en el disco** |
| 06/11 | No se pudo correr `go test -race` | En Windows requiere CGO y gcc. Queda pendiente (opcional) instalar gcc en WSL para correrlo |

---

## 7. Pendientes
- [ ] Parte B: K-Means paralelo en Go sobre `data/siniestros_limpio.csv` (lat, lon, hora).
- [ ] Elegir k (método del codo) y listar los puntos negros por zona y hora.
- [ ] Gráficos: clusters en el mapa y curva de speedup.
- [ ] GitHub: rama `develop` y ramas `feature/*` (Git Flow), commits y tablero Kanban.
- [ ] Informe Word: `1ACC0065_PC3_202610_Informe_UXXXXXXXX`.
- [ ] Liberar espacio en el disco C:.
