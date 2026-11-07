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

## 6. Paso 5 — Parte B: K-Means paralelo en Go (07/11/2026) **[Informe: Diseño del modelo ML distribuido / Paralelización / Evidencias]**

### Archivos
| Archivo | Contenido |
|---|---|
| `internal/datos/datos.go` | Carga paralela del CSV limpio en formato columnar (un slice por columna) |
| `internal/kmeans/kmeans.go` | K-Means paralelo (MapReduce), K-Means++ y método del codo (Kneedle) |
| `internal/kmeans/kmeans_test.go` | Pruebas: recupera centros conocidos, **paralelo = secuencial**, codo |
| `cmd/kmeans/main.go` | Ejecutable: codo, benchmark, entrenamiento final y perfil de cada zona |

### Ejecución
```bash
go test ./...
go run ./cmd/kmeans                          # elige k por el codo, entrena y perfila
go run ./cmd/kmeans -k 20                    # k fijo
go run ./cmd/kmeans -bench 1,2,4,8,12        # benchmark (30 iteraciones fijas, 5 repeticiones)
GOOS=linux go build -o bin/kmeans-linux ./cmd/kmeans   # binario para WSL
```

**Salidas** (en `data/`):
- `kmeans_codo.csv`: SSE por k.
- `kmeans_clusters.json`: perfil de las 15 zonas.
- `kmeans_puntos_reales.csv`: 8,791 siniestros reales con su cluster, para el mapa.
- `metricas_kmeans.json` y `metricas_kmeans_wsl.json`.

### 6.1 Diseño del modelo
- **Variables del clustering:** latitud y longitud proyectadas a km con una proyección equirectangular. La latitud de referencia es la media del dataset; 1° de latitud = 110.574 km y 1° de longitud = 111.320 · cos(lat₀) km. Así la distancia euclidiana se mide en km reales.
- **¿Por qué no se incluye la hora en el clustering?**
  - Mezclaría escalas distintas (km frente a horas).
  - La hora es cíclica: 23:00 está cerca de 00:00.
  - Se clusteriza por **dónde** y luego se perfila cada zona por **cuándo** (hora pico y franja de 3 h). Así se responde la pregunta de impacto: zonas **y** horas.
- **Inicialización: K-Means++** sobre una muestra aleatoria de 50,000 puntos (semilla 42). Es reproducible y evita que los centroides iniciales malos lleven a malos óptimos locales.
- **Convergencia:** cuando ningún centroide se mueve más de 0.01 km (10 m), o al llegar a 100 iteraciones. **Con k = 15 converge en 13 iteraciones.**
- **Cluster vacío:** conserva su centroide anterior.

### 6.2 Paralelización (MapReduce con workers persistentes)
```
            ┌──────── centroides globales (solo lectura) ────────┐
            ▼                        ▼                           ▼
   [worker 1: partición 1]  [worker 2: partición 2]  ...  [worker N: partición N]
    MAP: punto → centroide más cercano; acumula Σx, Σy, conteo, SSE
            └───────────────► chan Parcial ◄─────────────────────┘
                                     ▼
          [master] REDUCE: suma parciales EN ORDEN FIJO (por id de worker)
                   nuevos centroides = Σ / conteo → ¿convergió? → siguiente iteración
```
- **Particionado:** los 1,160,337 puntos se dividen en N rangos contiguos `[id·n/N, (id+1)·n/N)`. Cada worker es dueño de su partición durante todo el entrenamiento, igual que un nodo del clúster.
- **Comunicación:** en cada iteración el master difunde los centroides a cada worker por su canal y recibe un `Parcial` por un canal común. **En la versión distribuida este mismo mensaje viajará por TCP**: la función `Asignar` es exactamente lo que ejecutará cada nodo.
- **Sin condiciones de carrera:**
  1. Cada worker escribe solo en su propio `Parcial` y en su rango del arreglo de asignaciones (rangos disjuntos).
  2. Mientras los workers leen los centroides nadie los modifica. El master crea un slice **nuevo** para la siguiente iteración.
  3. Solo el master actualiza los centroides (un solo escritor).
  4. Ninguna estructura se comparte con escritura, así que no se necesita `sync.Mutex`.
- **Determinismo:** el master reduce los parciales en orden de id de worker, no en orden de llegada. Así el resultado no depende de qué goroutine termina primero.
- **Carga paralela del CSV:** `datos.Cargar` lee los 281 MB y los parte en N trozos alineados a `\n`. Cada goroutine parsea su trozo en un `Dataset` propio, que luego se concatena en orden. Tarda ~0.5 s en Windows. Se reutilizan los strings repetidos (departamentos, clases) para ahorrar memoria.

### 6.3 Elección de k (método del codo)
| k | 5 | 10 | **15** | 20 | 25 | 30 | 40 | 50 | 60 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| SSE (×10⁹ km²) | 29.9 | 11.6 | **7.03** | 4.59 | 3.40 | 2.28 | 1.68 | 1.22 | 1.01 |

- Se probó k = 5…60 de 5 en 5. El método **Kneedle** (el punto más alejado de la recta entre los extremos de la curva normalizada) eligió **k = 15**.
- De k = 5 a k = 15 (10 clusters más) el SSE baja **22,900 millones** de km². De k = 15 a k = 60 (45 clusters más) baja solo **6,000 millones**. A partir de 15, cada cluster adicional aporta mucho menos.
- **Observación:** a escala nacional, k = 15 da **macrozonas regionales** con radio medio de 15 a 103 km. Para "puntos negros" a nivel de calles se recomienda un **segundo nivel**: volver a aplicar K-Means dentro de la zona 1 (Lima Metropolitana). Queda como mejora futura.

### 6.4 Cantidad de goroutines en K-Means y su justificación
**Decisión: 12 workers** (`runtime.NumCPU()`, flag `-workers`). Es distinta a la del cargador (8), y la diferencia es intencional:

| | Cargador (Parte A) | K-Means (Parte B) |
|---|---|---|
| Tipo de trabajo | Parseo de texto: muchas asignaciones de memoria (GC), lectura de disco, shards y escritor que compiten por CPU | Cálculo puro: distancias y sumas sobre floats, sin asignaciones ni E/S en la fase MAP |
| Óptimo medido | 8 | **12** |
| Por qué | Pasar de 8 satura los hilos que necesitan el GC, los shards y el escritor | Cada hilo extra, aunque sea un E-core o hyper-threading, aporta cálculo útil; el master solo reduce 15 × N números por iteración |

**Respuesta corta para el profesor:** *"En K-Means usamos los 12 hilos porque la fase MAP es cálculo puro, sin E/S ni memoria nueva, y el benchmark lo confirma: 12 workers fue el más rápido en Windows y en Linux. En el cargador usamos 8 porque ahí hay GC, shards y escritura que necesitan CPU libre. La cantidad de goroutines depende del tipo de carga, no es un número fijo."*

### 6.5 Benchmark (k = 15, **30 iteraciones fijas**, mediana de 5 repeticiones, con calentamiento)
| Workers | Windows (ms) | ms/iter | Speedup | Eficiencia | WSL Ubuntu (ms) | ms/iter | Speedup | Eficiencia |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | 1,744 | 58.1 | 1.00x | 100 % | 1,880 | 62.7 | 1.00x | 100 % |
| 2 | 892 | 29.7 | 1.95x | 98 % | 1,000 | 33.3 | 1.88x | 94 % |
| 4 | 613 | 20.4 | 2.84x | 71 % | 638 | 21.3 | 2.95x | 74 % |
| 8 | 452 | 15.1 | 3.86x | 48 % | 646 | 21.5 | 2.91x | 36 % |
| 12 | **436** | **14.5** | **4.00x** | 33 % | **449** | **15.0** | **4.19x** | 35 % |

**Observaciones del benchmark:**
1. **Primera versión del benchmark (descartada):** medía el entrenamiento hasta converger, unos 13 iteraciones y menos de 1 s. Los resultados eran inestables: 12 workers dio 3.9x en una corrida y **10x** en otra, lo cual no es creíble. **Causa:** el portátil estaba **funcionando con batería** (Windows reporta `BatteryStatus = 1`) y cambia la frecuencia de la CPU constantemente. En mediciones de milisegundos eso distorsiona todo.
2. **Corrección:** benchmark con 30 iteraciones fijas (`Tol = -1`), para que cada configuración haga exactamente el mismo trabajo, con una corrida de calentamiento y la mediana de 5 repeticiones.
3. **Recomendación para las evidencias del informe:** correr el benchmark **conectado a la corriente** y con el plan de energía en "Máximo rendimiento".
4. La eficiencia baja de 2 a 12 workers por la **arquitectura híbrida** (los E-cores son más lentos que los P-cores y los hilos de hyper-threading comparten núcleo) y por el **límite de potencia** del i7-1265U (15 W).
5. La partición estática (rangos iguales) hace que un worker en un E-core termine después que uno en un P-core, y el master espera al más lento en cada iteración. **Mejora posible:** partición dinámica con bloques más pequeños.

### 6.6 Validación de la corrección
- **Prueba unitaria:** con 2, 3, 8 y 16 workers se obtienen **exactamente** las mismas asignaciones, iteraciones y SSE que con 1 worker. La paralelización no altera el resultado.
- **Prueba unitaria:** con 3 grupos sintéticos de centro conocido, los centroides encontrados quedan a menos de 0.1 km de los reales.
- **Entre sistemas operativos:** Windows y WSL producen el mismo SSE (7.026 × 10⁹), las mismas 13 iteraciones y las mismas 15 zonas.

### 6.7 Resultados: zonas de mayor riesgo **[Informe: Resultados / impacto social]**
Ranking por fallecidos. Las cifras "reales" cuentan solo `es_sintetico = 0`.

| # | Zona (moda depto / prov / distrito) | Centro (lat, lon) | Radio | Siniestros reales | Fallecidos reales | % nacional | Franja pico (3 h) | Clase | Causa principal* |
|---:|---|---|---:|---:|---:|---:|---|---|---|
| 1 | LIMA / LIMA / SAN MARTÍN DE PORRES | −11.959, −77.035 | 26 km | 1,931 | 2,072 | **19.9 %** | **05:00–07:59** | Atropello | Imprudencia del conductor |
| 2 | LA LIBERTAD / VIRÚ / VIRÚ | −7.873, −78.631 | 64 km | 839 | 1,053 | 10.1 % | 18:00–20:59 | Despiste | Imprudencia del conductor |
| 3 | CUSCO / ANTA / ANTA | −13.623, −72.137 | 76 km | 828 | 994 | 9.5 % | 17:00–19:59 | Despiste | Exceso de velocidad |
| 4 | ICA / ICA / SALAS | −13.795, −75.227 | 103 km | 718 | 953 | 9.1 % | **04:00–06:59** | Despiste | Exceso de velocidad |
| 5 | AREQUIPA / AREQUIPA / CERRO COLORADO | −16.417, −71.792 | 52 km | 670 | 846 | 8.1 % | 18:00–20:59 | Choque | Imprudencia del conductor |
| 6 | PUNO / SAN ROMÁN / JULIACA | −14.812, −70.313 | 99 km | 584 | 706 | 6.6 % | 17:00–19:59 | Choque | Exceso de velocidad |
| 7 | PIURA / PIURA / CASTILLA | −4.795, −80.546 | 71 km | 541 | 601 | 5.7 % | 19:00–21:59 | Choque | Exceso de velocidad |
| 8 | JUNÍN / YAULI / LA OROYA | −11.753, −75.261 | 67 km | 448 | 593 | 5.7 % | 17:00–19:59 | Despiste | Imprudencia del conductor |
| 9 | LA LIBERTAD / PACASMAYO / GUADALUPE | −6.864, −79.648 | 45 km | 492 | 551 | 5.3 % | 19:00–21:59 | Choque | Ebriedad y/o drogas |
| 10 | PUNO / EL COLLAO / ILAVE | −16.730, −69.812 | 99 km | 485 | 547 | 5.2 % | 18:00–20:59 | Choque | Imprudencia del conductor |
| 11 | ÁNCASH / CASMA / CASMA | −9.489, −77.811 | 81 km | 354 | 521 | 4.9 % | 18:00–20:59 | Despiste | Exceso de velocidad |
| 12 | CAJAMARCA / JAÉN / JAÉN | −6.063, −78.798 | 62 km | 354 | 433 | 4.1 % | 17:00–19:59 | Despiste | Exceso de velocidad |
| 13 | SAN MARTÍN / RIOJA / NUEVA CAJAMARCA | −6.139, −77.084 | 44 km | 285 | 317 | 3.0 % | 18:00–20:59 | Choque | Imprudencia del conductor |
| 14 | UCAYALI / CORONEL PORTILLO / YARINACOCHA | −9.573, −75.851 | 97 km | 199 | 240 | 2.3 % | 19:00–21:59 | Choque | Exceso de velocidad |
| 15 | LORETO / MAYNAS / SAN JUAN BAUTISTA | −3.841, −73.312 | 15 km | 63 | 65 | 0.6 % | 18:00–20:59 | Choque | Ebriedad y/o drogas |

\* Moda calculada **excluyendo** "NO CUENTA CON CAUSA ESPECIFICA" y "EN PROCESO DE INVESTIGACIÓN".

**Hallazgos (respuesta a la pregunta de impacto):**
1. **Lima Metropolitana concentra casi 1 de cada 5 muertes** (19.9 %). Su franja pico es **05:00–07:59**, la hora de salida al trabajo, y la clase principal es el **atropello**. → Patrullaje y control peatonal al amanecer.
2. **En 13 de las 15 zonas la franja pico está entre las 17:00 y las 21:59**: atardecer, poca luz, regreso a casa. → Es la franja prioritaria para patrullaje en las regiones.
3. **Ica tiene su pico de madrugada (04:00–06:59)** con despistes en la Panamericana Sur. *Hipótesis por validar:* fatiga o sueño de conductores de transporte interprovincial.
4. Las 5 primeras zonas (Lima, La Libertad, Cusco, Ica, Arequipa) suman el **56.7 %** de las muertes. → Ahí deberían ubicarse primero las ambulancias y las bases de fiscalización.
5. **Exceso de velocidad** es la causa identificada más frecuente en 7 zonas, sobre todo en la sierra (Cusco, Puno, Cajamarca) y en las carreteras de la costa (Ica, Piura, Casma).

### 6.8 Observaciones de la Parte B
- **Calidad de la causa:** el **50.6 %** de los siniestros reales tiene la causa "EN PROCESO DE INVESTIGACIÓN", lo que limita el análisis de causas. Es una limitación de la fuente (data preliminar 2024–2025).
- **Siniestros reales que llegaron al mapa:** 8,791 de 9,105 (96.6 %). Los 314 restantes recibieron errores irrecuperables durante el ensuciado intencional (paso 3) y se descartaron en la limpieza.
- **Bug corregido 1:** la zona se calculaba con la moda de departamento, provincia y distrito **por separado**, lo que daba combinaciones imposibles como "PUNO / TACNA / ILAVE". Se cambió a la moda de la tripleta completa.
- **Bug corregido 2:** la causa más frecuente era casi siempre "NO CUENTA CON CAUSA ESPECIFICA", que no informa nada. Se excluyeron los valores no informativos.
- **Bug corregido 3:** el benchmark era inestable (ver 6.5).

---

## 7. Incidencias técnicas
| Fecha | Incidencia | Solución |
|---|---|---|
| 06/11 | GitHub no acepta archivos > 100 MB y los CSV pesan ~290 MB | `.gitignore` para los CSV grandes y `bin/`. Los CSV se regeneran con los scripts (semillas fijas) |
| 06/11 | Go no está instalado en WSL | Compilación cruzada desde Windows (`GOOS=linux`); el binario corre en Ubuntu sin instalar Go |
| 06/11 | El CSV generado en Windows podía terminar líneas en `\r\n` | El cargador elimina `\r` al final de cada línea y maneja el encabezado con CRLF |
| 06/11 | **Disco C: lleno (10 MB libres de 465 GB)**: falló la compilación | Se borró `siniestros_aug.csv`, que es intermedio y se regenera con el script 02. **Pendiente: liberar espacio en el disco** |
| 06/11 | No se pudo correr `go test -race` | En Windows requiere CGO y gcc. Queda pendiente (opcional) instalar gcc en WSL para correrlo |
| 06/11 | Se liberó espacio en el disco: de 0.01 GB a **70 GB libres** | La usuaria vació Descargas |
| 06/11 | Primer commit en la rama `develop` (`f06da81`) y push a GitHub | Pull request `develop → main` a cargo de la usuaria |
| 07/11 | Las pruebas de Go "se colgaron" más de 2 minutos | No era un bloqueo del código: el equipo se suspendió o bloqueó (`LogonUI.exe` activo). Al volver a correr, las pruebas pasaron en 1.6 s |
| 07/11 | Benchmark inestable por funcionar con batería | Benchmark con 30 iteraciones fijas, calentamiento y mediana de 5 (ver 6.5) |
| 07/11 | El `/tmp` de WSL se borró entre ejecuciones porque la instancia se apagó | Guardar los resultados de WSL directamente en `/mnt/c/...` |
| 07/11 | El cargador y K-Means tienen óptimos de workers distintos (8 y 12) | Documentado como hallazgo (6.4), no es un error |

---

## 8. Pendientes
- [x] Parte A: cargador concurrente en Go.
- [x] Parte B: K-Means paralelo en Go.
- [x] Elegir k (codo → 15) y listar los puntos negros por zona y hora.
- [ ] Gráficos para el informe: mapa de clusters (`kmeans_puntos_reales.csv`), curva del codo (`kmeans_codo.csv`) y curvas de speedup.
- [ ] (Opcional) Segundo nivel de K-Means dentro de Lima Metropolitana para puntos negros a escala urbana.
- [ ] Repetir los benchmarks con el portátil conectado a la corriente.
- [ ] GitHub: commit de la Parte B, pull request y tablero Kanban.
- [ ] Informe Word: `1ACC0065_PC3_202610_Informe_UXXXXXXXX`.
- [x] Liberar espacio en el disco C:.
