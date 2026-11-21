# Guion y defensa — PC3: Zonificación de siniestros fatales con K-Means paralelo en Go

> Material de estudio para la exposición y para las preguntas del profesor.
> Presentación: https://claude.ai/artifact/4CyE8PQnnxLX7V8AunriVC (18 diapositivas).
> Todas las cifras salen de `docs/BITACORA.md` y del informe Word.

**Cómo usar este documento:** cada diapositiva tiene tres partes:
1. **Guion:** lo que dices (unos 15 a 25 segundos; en total, unos 5 a 6 minutos).
2. **Teoría y justificación:** lo que tienes que *entender*, aunque no lo digas.
3. **Si el profesor pregunta:** respuestas cortas, listas para decir.

**Reparto sugerido** (según el reporte de participación del informe):
- **Ximena:** diapositivas 1–5 (problema, pregunta, objetivos y dataset) y 16–18 (cierre).
- **Nicole:** diapositivas 6–15 (datos, Go, K-Means y resultados).

---

## Diapositiva 1 — Portada (Ximena)

**Guion:**
> "Buenos días, profesor. Somos Nicole Sihuincha y Ximena Chavarria. Presentamos la zonificación del riesgo de siniestros de tránsito fatales en el Perú, con K-Means paralelo implementado en Go sobre más de 1.2 millones de registros."

**Ojo:** el título dice *1.2 millones*, pero los datos reales son 9,105. Si el profesor lo menciona, responde con la diapositiva 7 (*augmentation*). Nunca digas que el ONSV tiene 1.2 millones de registros.

---

## Diapositiva 2 — Los siniestros viales son un problema de salud pública (Ximena)

**Guion:**
> "Según la OMS, los siniestros de tránsito causan cerca de 1.19 millones de muertes al año en el mundo. En el Perú, la Defensoría del Pueblo reportó más de 83 mil siniestros solo en 2023. Nuestro dataset del ONSV tiene 9,106 siniestros fatales entre 2021 y 2025. Patrulleros y ambulancias son limitados: tienen que estar donde y cuando ocurren las muertes."

**Teoría y justificación:**
- Las tres cifras van de lo global a lo local: mundo (OMS), Perú (Defensoría) y nuestro dataset (ONSV).
- El problema tiene dos partes:
  - **Social:** los recursos son limitados y hay que asignarlos.
  - **Computacional:** procesar muchos registros georreferenciados de forma secuencial no escala.

**Si el profesor pregunta:**
- *"¿De dónde salen las cifras?"* → OMS (2023), hoja informativa "Traumatismos causados por el tránsito", y nota de prensa de la Defensoría del Pueblo (2024). Las dos están en la bibliografía del informe, en formato APA 7.
- *"¿Por qué 9,106 y luego 9,105?"* → El Excel tiene 9,106 filas. Un registro no tenía longitud y se descartó en la preparación.

---

## Diapositiva 3 — Pregunta de impacto social (Ximena)

**Guion:**
> "Esta pregunta guía todo el proyecto: ¿en qué zonas y horas del país hay mayor riesgo de siniestros fatales? La respuesta sirve a la Policía para planificar el patrullaje, al SAMU para ubicar ambulancias y a las municipalidades para priorizar la señalización."

**Teoría y justificación:**
- La pregunta tiene dos dimensiones: **dónde** (espacio) y **cuándo** (hora). Esto explica una decisión de diseño clave: K-Means agrupa por **dónde** y luego cada zona se describe por **cuándo** (ver diapositiva 10).
- **Es un problema no supervisado:** no hay una etiqueta de "zona" que predecir; las zonas las descubre el algoritmo. Por eso se usa clustering y no clasificación.

**Si el profesor pregunta:**
- *"¿Quién usaría esto?"* → La PNP (tránsito y UPIAT), el SAMU, los servicios de emergencia regionales y las municipalidades.

---

## Diapositiva 4 — Objetivos (Ximena)

**Guion:**
> "Nuestro objetivo general es construir en Go un sistema concurrente que cargue, limpie y agrupe más de un millón de registros. Los específicos son cinco: preparar el volumen de datos, implementar el cargador concurrente, implementar K-Means paralelo, medir la aceleración y describir cada zona por su horario y sus causas."

**Teoría y justificación:**
- Cada objetivo específico tiene una diapositiva que lo cumple:

  | Objetivo | Diapositivas que lo cumplen |
  |---|---|
  | 1 | 7 y 8 |
  | 2 | 9 |
  | 3 | 10 y 15 |
  | 4 | 12 |
  | 5 | 13 y 14 |

- Si el profesor pregunta *"¿cumplieron los objetivos?"*, recorre esta tabla.

---

## Diapositiva 5 — Datos reales del ONSV (Ximena)

**Guion:**
> "Trabajamos con datos reales del Observatorio Nacional de Seguridad Vial del MTC; los registra la PNP. Son 9,106 siniestros fatales con 27 variables. Un dato importante: la mitad de los registros tiene la causa 'en proceso de investigación', y eso limita el análisis de causas."

**Teoría y justificación:**
- Las variables del modelo son **latitud y longitud**. Hora, clase, causa y fallecidos se usan para **describir** las zonas.
- Las columnas de señalización tienen entre 78 % y 94 % de nulos. Por eso no se usan: imputar el 94 % de una columna sería inventar datos.
- El 50.6 % de causas "en proceso de investigación" se debe a que 2024 y 2025 son datos **preliminares**.
- Detalles de la preparación:
  - Los encabezados reales están en la fila 5 del Excel.
  - Dos fechas venían como tipo fecha de Excel y se unificaron.
  - No había duplicados en los datos originales.

**Si el profesor pregunta:**
- *"¿Por qué no usaron la señalización como variable?"* → Por exceso de nulos (78–94 %). Imputarla introduciría más ruido que información.
- *"¿Las coordenadas son confiables?"* → Las validamos: todas caen dentro del *bounding box* del Perú (latitud −18.31 a −3.48; longitud −81.29 a −68.95).

---

## Diapositiva 6 — Arquitectura: una cadena de seis etapas (Nicole)

**Guion:**
> "La solución es una cadena de seis etapas. Las tres primeras preparan los datos en Python: limpiamos el Excel, lo aumentamos a 1.2 millones de filas y lo ensuciamos a propósito. Desde ahí, todo el procesamiento masivo es en Go: el cargador concurrente limpia y K-Means paralelo identifica las 15 zonas. La API, MongoDB, Redis y la interfaz web son de las siguientes entregas."

**Teoría y justificación:**
- **¿Por qué Python para preparar los datos y Go para procesarlos?**
  - La preparación se ejecuta una sola vez y no es el objeto del curso. Python con pandas lee Excel con facilidad.
  - Lo que el curso evalúa es la concurrencia, y eso está todo en Go.
- **Etapas desacopladas:** cada etapa deja un archivo, así que se puede volver a ejecutar cualquiera por separado. Los scripts usan semillas fijas (42 y 7), así que son reproducibles.
- **Los CSV no están en GitHub:** pesan unos 290 MB y GitHub no acepta archivos de más de 100 MB. Se regeneran con los scripts.

---

## Diapositiva 7 — Data augmentation: jittering (Nicole)

**Guion:**
> "El laboratorio exige más de un millón de registros y teníamos 9,105. Aplicamos inyección de ruido, o jittering: cada siniestro se replica 131 veces, moviendo sus coordenadas unos 100 metros, la hora unos 15 minutos y la fecha hasta una semana. Así el punto negro sigue en la misma zona. Descartamos SMOTE porque sirve para clasificación. Y somos honestas: el augmentation no aporta información nueva, por eso contrastamos las cifras de impacto con los datos reales."

**Teoría y justificación:**
- **Las cuentas:** 9,105 originales + 9,105 × 131 réplicas = 9,105 × 132 = **1,201,860** filas.
- **El ruido es gaussiano**, centrado en 0: la mayoría de las réplicas quedan muy cerca del original.
  - σ = 0.0009°. Como 1° de latitud ≈ 111 km, 0.0009 × 111 km ≈ **0.1 km = 100 m**.
  - Con una distribución normal, el 95 % de las réplicas cae a menos de 2σ, es decir, a menos de unos 200 m. Un cluster tiene un radio de 15 a 103 km, así que el ruido no cambia ningún punto de zona.
- **Hora:** σ = 15 min y se recorta al rango 00:00–23:59. Conserva la franja horaria del riesgo.
- **Fecha:** desplazamiento de ±7 días, dentro de 2021–2025. Conserva la estacionalidad (por ejemplo, feriados o temporada de lluvias).
- **Columna `es_sintetico`** (0 = real, 1 = generado): permite separar los datos reales de los aumentados.
- **⚠ Qué se calculó con qué datos** (lo verifiqué en `cmd/kmeans/main.go`):
  - Los **conteos absolutos** ("siniestros reales" y "fallecidos reales") usan **solo** `es_sintetico = 0`.
  - El **% de muertes por zona**, la **franja pico**, la **clase** y la **causa** se calcularon sobre los **1.16 M registros** (reales y sintéticos).
  - Al recalcular con solo los reales, casi nada cambia:

    | Cifra | Con 1.16 M | Solo reales |
    |---|---|---|
    | Lima | 19.86 % | 19.75 % |
    | Las 5 primeras zonas | 56.7 % | 56.4 % |
    | Franjas pico | — | 13 de 15 iguales |

  - Las dos franjas que cambian son **Puno (zona 6)**, que pasa de 17:00 a 18:00, y **Cajamarca (zona 12)**, que pasa de 17:00–19:59 a **04:00–06:59**. Con solo los reales serían 12 de 15 zonas con pico en la tarde y noche, en vez de 13.
  - **Si el profesor pregunta, dilo así:** *"Los porcentajes y franjas se calcularon con todo el dataset; como cada réplica solo mueve unos 100 m y 15 min, al contrastarlos con los datos reales cambian menos de 0.3 puntos."*
  - **Ojo, el informe** (sección 3.2, al final del párrafo del augmentation) dice que *"las cifras de impacto se reportan sobre los registros reales"*. Conviene cambiarlo por: *"los conteos de siniestros y fallecidos se reportan sobre los registros reales (es_sintetico = 0); los porcentajes y franjas horarias, calculados sobre el dataset completo, difieren en menos de 0.3 puntos de los calculados solo con los reales"*.
- **¿Por qué no SMOTE?** SMOTE (*Synthetic Minority Over-sampling Technique*) crea ejemplos por interpolación entre vecinos de la **clase minoritaria** para balancear clases en clasificación supervisada. K-Means es no supervisado: no hay clases.
- **¿Por qué no un generador como SDV o una cópula gaussiana?** El borrador lo mencionaba. Un modelo generativo aprende la distribución conjunta y puede crear puntos en lugares donde nunca hubo siniestros, es decir, **zonas falsas**. El jittering solo crea puntos alrededor de siniestros reales.

**Si el profesor pregunta:**
- *"¿No están inflando artificialmente los clusters?"* → Sí, cada punto pesa 132 veces, pero **todos por igual**: la proporción entre zonas no cambia. Los centroides con datos aumentados son prácticamente los mismos que con los datos reales. El volumen sirve para demostrar el rendimiento paralelo, no para sacar conclusiones nuevas.
- *"¿Por qué 131?"* → El script calcula las réplicas para llegar a un objetivo de 1.2 M: ⌈(1,200,000 − 9,105) / 9,105⌉ = ⌈130.8⌉ = 131. Elegimos 1.2 M y no 1 M para tener margen: después de ensuciar y limpiar quedan 1,160,337 filas, todavía por encima del millón.

---

## Diapositiva 8 — Ensuciamos la data a propósito (Nicole)

**Guion:**
> "Para demostrar la limpieza, metimos errores a propósito en el 9 % de las filas: nulos, coordenadas en cero o invertidas, horas como 25:70, fechas en otro formato, texto en minúsculas, fallecidos negativos y duplicados. El cargador en Go detectó los siete tipos y, cuando se podía, corrigió en lugar de descartar; por ejemplo, intercambió 3,001 coordenadas invertidas. Conservamos el 95.6 % de la data."

**Teoría y justificación:**
- **Técnica:** *dirty data injection* (inyección de errores). Como sabes exactamente qué errores metiste, puedes **medir** cuántos detecta el cargador. Es como un test con respuesta conocida.
- **Hay dos tipos de reglas:**

  | Tipo | Errores | Qué hace el cargador |
  |---|---|---|
  | **Corrección** (recuperables) | Lat/lon invertidas, hora `4.40` o `4:5`, fecha `dd/mm/aaaa`, texto en minúsculas, fallecidos vacío | Se arreglan |
  | **Descarte** (irrecuperables) | Coordenada (0,0) o fuera del Perú, hora `25:70`, fecha `00/00/0000`, fallecidos −1 o 999, duplicados | Se descartan |

- **Coordenadas invertidas:** se intercambian solo si, al intercambiarlas, caen dentro del Perú. Si no, se descartan.
- **Fallecidos vacío → se imputa 1:** el dataset es de siniestros **fatales**, así que hay al menos un fallecido. Es una imputación con conocimiento del dominio, no un promedio.
- **¿Por qué los números no cuadran exactamente?**
  - Se inyectaron 12,018 duplicados y se eliminaron 11,616. El resto ya se había descartado por **otro** error antes de llegar a la deduplicación. Una fila puede tener dos errores y se cuenta una sola vez.
  - Se inyectaron 24,037 textos sucios y se normalizaron 24,112. Hay más normalizados porque algunos duplicados copiaron filas que ya tenían texto sucio.

**Si el profesor pregunta:**
- *"¿Por qué no descartar todo lo sucio?"* → Porque perderías datos válidos. Una hora `4:5` es claramente las 04:05. Recuperar en vez de descartar es lo que permite conservar el 95.6 %.
- *"¿Por qué 1 a 100 como rango válido de fallecidos?"* → El máximo real es 33 (un accidente de bus). 100 da margen para un caso extremo y rechaza valores como 150 o 999.

---

## Diapositiva 9 — Cargador concurrente: pipeline sin estado compartido (Nicole)

**Guion:**
> "El cargador es un pipeline de cuatro etapas conectadas por channels. Una goroutine parte el archivo en 71 bloques; ocho workers los leen en paralelo y limpian las filas; ocho shards eliminan duplicados, y un mismo código siempre llega al mismo shard porque se reparte por hash; al final, una sola goroutine escribe el archivo. Nadie comparte datos que se escriban, por eso no necesitamos mutex. La primera versión, con un solo colector, se estancaba en 2.6x por la ley de Amdahl; con los shards bajamos de 5.1 a 1.6 segundos."

**Teoría y justificación (lo más importante del curso):**

**Goroutine:**
- Es un hilo ligero administrado por el runtime de Go, no por el sistema operativo. Empieza con una pila de unos pocos KB, mientras que un hilo del sistema operativo usa del orden de 1 MB.
- El planificador de Go es **M:N** (modelo G-M-P): reparte muchas goroutines (G) sobre pocos hilos del sistema (M), con P procesadores lógicos. P = `GOMAXPROCS`, que por defecto es igual al número de CPUs.

**Channel:**
- Es una cola tipada y sincronizada entre goroutines. Se basa en CSP (*Communicating Sequential Processes*, Hoare, 1978).
- El lema de Go: *"No te comuniques compartiendo memoria; comparte memoria comunicándote."*

**Condición de carrera (*data race*):**
- Ocurre cuando dos o más goroutines acceden a la misma memoria al mismo tiempo, al menos una escribe y no hay sincronización.
- El resultado depende del orden de ejecución, así que es no determinista.

**Cómo la evitamos sin mutex (memoriza los cuatro puntos):**
1. **`ReadAt`:** cada worker lee *su* rango de bytes con `ReadAt`, que recibe el desplazamiento como parámetro (internamente usa `pread`). No comparte el puntero de lectura del archivo, así que es seguro en paralelo.
2. **Contadores propios:** cada worker y cada shard tiene sus *propias* estadísticas. Al final se envían por un channel y se suman.
3. **Sharding por hash:** el shard de una fila es `FNV-1a(código) % 8`. El mismo código siempre va al mismo shard, así que cada mapa de "códigos vistos" lo toca **una sola goroutine**. No hace falta `sync.Mutex` ni `sync.Map`.
4. **Un solo escritor** para el archivo de salida.

**Patrones de concurrencia:**
- **Pipeline:** etapas en cadena unidas por channels.
- **Worker pool:** N workers toman tareas de un mismo channel.
- **Fan-out:** repartir el trabajo entre varios destinos (aquí, por hash hacia los shards).
- **Fan-in:** juntar varios resultados en uno (aquí, hacia el escritor).

**Sincronización:**
- `sync.WaitGroup` espera a que terminen todos los workers.
- Después de esperar, se cierra el channel. Así el siguiente consumidor sabe que no llegan más datos: `range` sobre un channel cerrado termina el ciclo.

**Bloques de 4 MB:**
- Se alinean a `\n` para no cortar una fila por la mitad.
- Son 71 bloques para 8 workers, unos 9 por worker. Esto equilibra la carga: un worker que termina antes toma otro bloque. Es importante porque los núcleos P y E van a velocidades distintas.

**Ley de Amdahl:**
- Fórmula: S(N) = 1 / ((1 − p) + p/N), donde p es la fracción paralelizable.
- Con un solo colector secuencial que deduplicaba 1.2 M códigos, (1 − p) era grande y el speedup se estancaba en unos 2.6x.
- Al paralelizar la deduplicación con shards, p creció y el speedup subió a 4.51x en WSL.

**GOGC = 400:**
- El recolector de basura (GC) se activa cuando el heap crece un GOGC % sobre la memoria viva. El valor por defecto es 100.
- Con 400 se ejecuta menos veces. Parsear texto genera muchas asignaciones de memoria, así que se ganó cerca de 1.4x.
- El costo es usar más RAM.

**Si el profesor pregunta:**
- *"¿Por qué no usaron un mutex sobre un mapa global?"* → Funcionaría, pero los 8 workers competirían por el mismo lock (contención) y esa sección crítica se volvería secuencial. Por Amdahl, limitaría el speedup. El sharding elimina la contención.
- *"¿Corrieron el detector de carreras?"* → **Sé honesta:** `go test -race` necesita CGO y gcc en Windows, y no lo teníamos instalado. Está en las recomendaciones del informe. La ausencia de carreras está garantizada por el diseño: ninguna estructura se escribe desde dos goroutines. Además, las pruebas dan resultados idénticos con 1 y con N workers.
- *"¿Por qué FNV-1a?"* → Es un hash no criptográfico, muy rápido y con buena dispersión para strings cortos. Viene en la librería estándar de Go (`hash/fnv`). No necesitamos seguridad, solo repartir de forma uniforme.

---

## Diapositiva 10 — K-Means paralelo con MapReduce (Nicole)

**Guion:**
> "K-Means se paraleliza de forma natural. En cada iteración, el master envía los centroides a los workers; cada worker, dueño de su partición, asigna sus puntos al centroide más cercano y acumula sumas parciales; luego el master suma esos parciales y calcula los nuevos centroides. Solo el master actualiza los centroides, y suma en orden fijo, por eso el resultado es determinista. Convertimos las coordenadas a kilómetros en vez de usar Z-Score, para que la distancia sea real, e inicializamos con K-Means++."

**Teoría de K-Means (algoritmo de Lloyd, 1982):**
- **Objetivo:** minimizar la **SSE** (suma de errores cuadráticos, también llamada *inercia*): Σ ‖xᵢ − μ_c(i)‖², la suma de las distancias al cuadrado de cada punto a su centroide.
- **Se repite hasta converger:**
  1. **Asignación:** cada punto va al centroide más cercano.
  2. **Actualización:** cada centroide pasa a ser la media de sus puntos.
- **Siempre converge**, porque la SSE nunca sube y hay un número finito de particiones. Pero converge a un **mínimo local**, no necesariamente al global. Por eso importa la inicialización.
- **Complejidad:** O(n · k · d · iteraciones). Aquí n = 1,160,337, k = 15, d = 2 e iteraciones = 13.
- **Supuestos y limitaciones:**
  - Supone clusters más o menos esféricos y de tamaño parecido.
  - Es sensible a los outliers.
  - Hay que elegir k antes de ejecutarlo.

**¿Por qué se puede paralelizar? (la clave de MapReduce):**
- El nuevo centroide es la media: Σx / conteo.
- **La suma es asociativa:** puedes sumar por partes y luego sumar los resultados parciales.
- Cada worker devuelve solo `(Σx, Σy, conteo)` por cluster, que son 15 × 3 números, no sus 100 mil puntos.
- **Map:** asignar puntos y acumular sumas parciales. **Reduce:** sumar los parciales y dividir.
- Es la misma idea que Dean y Ghemawat (2008). La suma parcial dentro de cada worker hace el papel de un *combiner*.

**¿Por qué es importante para la versión distribuida (TCP)?**
- Lo que viaja por la red en cada iteración es muy poco: los 15 centroides de ida y 15 sumas de vuelta.
- Los puntos nunca se mueven: cada nodo es dueño de su partición.
- Por eso K-Means escala bien a un clúster: hay poca comunicación y mucho cálculo local.

**Determinismo:**
- La suma de números de punto flotante **no es exactamente asociativa**: (a+b)+c puede diferir de a+(b+c) en el último bit.
- Si el master sumara los parciales en orden de llegada, el resultado cambiaría según qué goroutine termine primero.
- Por eso suma **por id de worker**, en orden fijo.

**No hay carreras:**
- Los centroides son de solo lectura mientras los workers trabajan.
- El master crea un slice **nuevo** para la siguiente iteración.
- Cada worker escribe solo en su rango del arreglo de asignaciones (rangos disjuntos).

**Proyección equirectangular, en vez de Z-Score:**
- x = (lon − lon₀) · 111.320 · cos(lat₀) km
- y = (lat − lat₀) · 110.574 km
- lat₀ es la latitud media del dataset.
- **¿Por qué?** Un grado de longitud mide menos km que uno de latitud, porque los meridianos se juntan hacia los polos. K-Means usa distancia **euclidiana**, así que las coordenadas tienen que estar en las mismas unidades reales (km).
- **Z-Score** divide cada eje por *su propia* desviación estándar. En el Perú, la latitud y la longitud tienen dispersiones distintas, así que Z-Score **estiraría un eje más que el otro** y deformaría las distancias geográficas.
- **¿Por qué no Haversine?** Haversine es la distancia exacta sobre la esfera, pero K-Means necesita que la **media** sea el punto que minimiza la suma de distancias al cuadrado, y eso solo se cumple con la distancia euclidiana. Con otra distancia el paso de "media" ya no sería correcto; habría que usar k-medoids, que es mucho más costoso.
- **Error de la proyección:** el Perú está cerca del ecuador (latitud 0° a −18°) y cos(lat) varía poco, entre 0.95 y 1.00. El error es de unos pocos %, aceptable para zonas de decenas de km.

**K-Means++ (Arthur y Vassilvitskii, 2007):**
- El primer centroide se elige al azar. Cada siguiente se elige con probabilidad **proporcional a D(x)²**, la distancia al cuadrado al centroide más cercano ya elegido.
- Así los centroides iniciales quedan separados.
- Garantiza en esperanza una solución O(log k)-competitiva con la óptima y suele converger en menos iteraciones.
- Se ejecuta sobre una muestra de 50,000 puntos (semilla 42) para que sea rápido y reproducible.

**Convergencia:**
- Se detiene cuando ningún centroide se mueve más de 0.01 km (10 m), o a las 100 iteraciones.
- Con k = 15 convergió en 13 iteraciones.
- **Cluster vacío:** conserva su centroide anterior, para evitar una división por cero.

**¿Por qué la hora no entra en el clustering?**
1. Mezcla escalas: km frente a horas. ¿Cuántos km equivalen a una hora? No hay una respuesta natural.
2. **La hora es cíclica:** 23:00 está a una hora de 00:00, pero numéricamente están a 23 de distancia.
3. Se agrupa por **dónde** y después se describe cada zona por **cuándo**. Así se responde la pregunta de impacto (zonas **y** horas).
- *Alternativa si insiste:* codificar la hora como (sin(2πh/24), cos(2πh/24)) y ponderarla. Lo dejamos como mejora.

**Si el profesor pregunta:**
- *"¿Por qué K-Means y no DBSCAN?"* →
  - El caso del curso pide K-Means.
  - K-Means se paraleliza muy bien con MapReduce, mientras que DBSCAN necesita consultas de vecindario, que son costosas (O(n²) sin un índice espacial) y difíciles de distribuir.
  - DBSCAN sí encontraría zonas de forma irregular, como las carreteras, y no necesita k. Es una alternativa válida para el análisis urbano de segundo nivel.
- *"¿Cómo saben que no cayeron en un mínimo local malo?"* → K-Means++ reduce ese riesgo. Además, la semilla fija hace que el resultado sea reproducible, y Windows y WSL dan el mismo SSE.

---

## Diapositiva 11 — Elección de k: método del codo (Nicole)

**Guion:**
> "Para elegir cuántas zonas, ejecutamos K-Means con k de 5 a 60, de 5 en 5, y graficamos el error SSE. En lugar de elegir el codo a ojo, usamos el algoritmo Kneedle, que lo detecta de forma objetiva: k igual a 15. El modelo convergió en 13 iteraciones."

**Teoría y justificación:**
- **La SSE siempre baja al aumentar k.** Con k = n, la SSE es 0. Por eso no se puede elegir "el k con menor SSE".
- **El codo** es el punto donde agregar clusters deja de reducir mucho el error: los rendimientos decrecientes.
- **Kneedle (Satopää et al., 2011):**
  1. Se normaliza la curva SSE(k) al rango [0,1] en los dos ejes.
  2. Se traza la recta entre el primer y el último punto.
  3. Se elige el k cuyo punto está **más lejos** de esa recta.
- **Las cifras:**
  - De k = 5 a k = 15 (10 clusters más), la SSE baja 22,900 millones de km².
  - De k = 15 a k = 60 (45 clusters más), baja solo 6,000 millones.
- **Valores:** SSE(5) = 29.9 × 10⁹; SSE(15) = 7.03 × 10⁹; SSE(60) = 1.01 × 10⁹ km².

**Si el profesor pregunta:**
- *"¿Por qué no el coeficiente de silueta?"* → La silueta exacta es O(n²): con 1.16 M puntos son unos 1.3 billones de distancias. Se podría calcular sobre una muestra; queda como validación adicional.
- *"¿La SSE es tan grande, 7 × 10⁹ km²?"* → Es la suma sobre 1.16 millones de puntos. Por punto son unos 6,000 km², es decir, unos 78 km de distancia típica al centroide. Es coherente con zonas regionales de 15 a 103 km de radio.
- *"¿15 zonas no es poco para todo el Perú?"* → A escala nacional da **macrozonas regionales**, útiles para decidir dónde poner bases de ambulancias. Para puntos negros a nivel de calles proponemos un segundo K-Means dentro de Lima (diapositiva 17).

---

## Diapositiva 12 — Rendimiento y elección de goroutines (Nicole)

**Guion:**
> "Medimos con 1, 2, 4, 8 y 12 goroutines, en Windows y en Linux con WSL. En el cargador elegimos 8 workers: con 12 el tiempo empeora, porque los shards, el escritor y el recolector de basura también necesitan CPU. En K-Means usamos 12, uno por hilo, porque es cálculo puro y sigue escalando. La aceleración no es lineal porque el procesador es híbrido —solo dos núcleos son de alto rendimiento— y la laptop estaba en batería, a 15 watts."

**Métricas:**
- **Speedup:** S(N) = T(1) / T(N).
- **Eficiencia:** E(N) = S(N) / N. Mide qué tan bien se aprovecha cada hilo.

**Benchmarks:**

| | Cargador (Windows / WSL) | K-Means (Windows / WSL) |
|---|---|---|
| 1 worker | 5,062 ms / 7,839 ms | 1,744 ms / 1,880 ms |
| 8 workers | **1,641 ms (3.09x) / 1,740 ms (4.51x)** | 452 ms / 646 ms |
| 12 workers | 2,048 ms (empeora) / 2,208 ms | **436 ms (4.00x) / 449 ms (4.19x)** |

**Hardware: Intel i7-1265U** (12.ª generación, serie U de 15 W):
- **10 núcleos físicos y 12 hilos:**
  - 2 P-cores (rendimiento) con hyper-threading → 4 hilos;
  - 8 E-cores (eficiencia) → 8 hilos.
- `runtime.NumCPU()` devuelve 12, pero esos 12 hilos no rinden lo mismo.

**¿Por qué 8 en el cargador?**
- Con 12 workers más 12 shards hay 24 goroutines activas sobre 12 hilos.
- Los shards, el escritor y el GC compiten con los workers por la CPU.
- Además, el parseo genera muchas asignaciones de memoria (GC) y hay lectura de disco.
- Con 8 workers quedan hilos libres para el resto.

**¿Por qué 12 en K-Means?**
- La fase Map es **cálculo puro**: solo floats, sin asignaciones de memoria y sin E/S.
- Cada hilo extra, aunque sea un E-core, aporta cálculo útil.
- El master solo suma 15 × N números por iteración.

**Mensaje clave:** *el número de goroutines depende del tipo de carga y se decide midiendo, no es un número fijo.*

**¿Por qué el speedup no es lineal? (lo vas a necesitar)**
1. **Ley de Amdahl:** siempre queda una parte secuencial (escribir el archivo, el reduce del master).
   - *Estimación:* con S = 4.51 a N = 8, despejando p de Amdahl, sale p ≈ 0.89. Es decir, alrededor del 11 % del trabajo se comporta como secuencial, y el speedup máximo teórico sería alrededor de 9x.
   - Es solo una aproximación, porque los núcleos no son iguales.
2. **CPU heterogénea:**
   - Los E-cores son más lentos.
   - Dos hilos de hyper-threading comparten las unidades de cálculo del mismo núcleo.
   - Con **partición estática**, el master espera en cada iteración al worker más lento (efecto *straggler*).
3. **Límite de potencia y temperatura:** con todos los núcleos al 100 %, la frecuencia turbo baja.
4. **Ancho de banda de memoria:** todos los hilos leen de la misma RAM.

**¿Por qué WSL es más lento con 1 worker pero escala mejor?**
- Lee el archivo desde `/mnt/c`, el sistema de archivos de Windows montado en Linux. Ese acceso tiene más latencia.
- Al ser más lento en secuencial, hay más que ganar paralelizando.

**El benchmark que descartamos (cuéntalo si preguntan por la confiabilidad):**
- El primer benchmark medía hasta converger (menos de 1 s) y daba resultados absurdos, como **10x en una corrida**.
- **Causa:** la laptop estaba en batería y la frecuencia de la CPU variaba.
- **Corrección:**
  - 30 iteraciones fijas: todas las configuraciones hacen el mismo trabajo.
  - Una corrida de calentamiento.
  - La **mediana** de 5 repeticiones. La mediana resiste valores extremos mejor que el promedio.

**Si el profesor pregunta:**
- *"¿Por qué con 8 workers K-Means en WSL da 646 ms, casi igual que con 4?"* → Es ruido de medición: la laptop estaba en batería y WSL corre en una máquina virtual. Por eso recomendamos repetir los benchmarks conectados a la corriente. La tendencia general (12 es el mejor) se mantiene en los dos sistemas.

---

## Diapositiva 13 — 15 zonas de riesgo en el Perú (Nicole)

**Guion:**
> "Este es el resultado principal: los 8,791 siniestros reales en gris y las 15 zonas como burbujas, de tamaño proporcional a los fallecidos. Lima concentra casi el 20 % de las muertes, seguida de La Libertad, Cusco, Ica y Arequipa. Estas cinco zonas suman el 56.7 % de las muertes del país: ahí debería priorizarse el patrullaje y las ambulancias."

**Teoría y justificación:**
- **¿Por qué 8,791 y no 9,105?** Los 314 siniestros reales que faltan recibieron errores irrecuperables durante el ensuciado y se descartaron en la limpieza.
- **El nombre de cada zona** es la combinación departamento / provincia / distrito más frecuente, tomada **como tripleta**.
  - Hubo un bug: al calcular la moda de cada columna por separado salían combinaciones imposibles, como "PUNO / TACNA / ILAVE". Se corrigió.
- **Diseño del mapa:**
  - No se usaron 15 colores: serían indistinguibles, sobre todo para personas daltónicas. La identidad de la zona la da el número y la magnitud la da el tamaño de la burbuja.
  - La paleta azul/naranja es apta para daltonismo.
- **Horas pico exactas** (franjas de 3 h):

  | Zona | Franja pico |
  |---|---|
  | Lima | 05:00–07:59 |
  | La Libertad | 18:00–20:59 |
  | Cusco | 17:00–19:59 |
  | Ica | 04:00–06:59 |
  | Arequipa | 18:00–20:59 |

- **Radios:** van de 15 km (Loreto) a 103 km (Ica). Lima tiene 26 km.

**Si el profesor pregunta:**
- *"¿Cómo calcularon la franja pico?"* → Es la ventana de 3 horas consecutivas con más siniestros en la zona. Es **circular**: 23:00–01:59 también cuenta como franja válida. Se calculó sobre todo el dataset y se contrastó con los reales (ver la diapositiva 7).
- *"¿Lima es una sola zona?"* → Sí, a esta escala. Es la limitación que proponemos resolver con el segundo nivel de K-Means.

---

## Diapositiva 14 — Cuándo y por qué ocurren las muertes (Nicole)

**Guion:**
> "En 13 de las 15 zonas, la franja de mayor riesgo es entre las 5 y las 10 de la noche. En 7 zonas la causa más frecuente es el exceso de velocidad. Lima e Ica son la excepción: su pico es de madrugada, y en Lima predominan los atropellos. La recomendación concreta: patrullaje de tarde y noche en casi todo el país, y de madrugada en Lima e Ica."

**Teoría y justificación:**
- **Tarde y noche (17:00–21:59):** atardecer, poca luz, regreso a casa y cansancio.
- **Lima, 05:00–07:59, con atropellos:** es la hora de salida al trabajo, con muchos peatones y poca luz.
- **Ica, 04:00–06:59, con despistes en la Panamericana Sur:** es solo una **hipótesis por validar**; podría ser fatiga de conductores del transporte interprovincial nocturno. Dilo así, como hipótesis.
- **Exceso de velocidad:** es la causa más frecuente en la sierra (Cusco, Puno, Cajamarca) y en las carreteras de la costa (Ica, Piura, Casma).
- **Al calcular la causa más frecuente se excluyeron** "NO CUENTA CON CAUSA ESPECÍFICA" y "EN PROCESO DE INVESTIGACIÓN", porque no aportan información. Si no se excluían, casi todas las zonas salían con "sin causa".

**Si el profesor pregunta:**
- *"¿Correlación o causalidad?"* → Es **descriptivo**: muestra dónde y cuándo se concentran las muertes, no por qué. Las causas vienen del registro policial y la mitad está en investigación.

---

## Diapositiva 15 — Validación: paralelo es igual a secuencial (Nicole)

**Guion:**
> "Lo más importante en programación concurrente es que el resultado sea correcto. Comprobamos que K-Means con varias goroutines da exactamente los mismos centroides, asignaciones y error que con una sola, y que Windows y Linux dan el mismo resultado. Tenemos pruebas unitarias para la limpieza y para el paralelismo, y no hay condiciones de carrera porque cada goroutine escribe solo en su partición."

**Teoría y justificación:**
- **Prueba de equivalencia:** con 2, 3, 8 y 16 workers se obtienen **exactamente** las mismas asignaciones, iteraciones y SSE que con 1 worker. Lo permite el reduce en orden fijo (diapositiva 10).
- **Prueba de corrección:** con 3 grupos sintéticos de centro conocido, los centroides encontrados quedan a menos de 0.1 km de los reales.
- **Prueba del codo:** Kneedle encuentra el codo correcto en una curva conocida.
- **Limpieza:** 16 casos de prueba, uno por cada tipo de error, más la normalización de la hora.
- **Entre sistemas:** Windows y WSL dan la misma SSE (7.026 × 10⁹), 13 iteraciones y 15 zonas.
- **Gestión:**
  - Git Flow: `main` / `develop`, integración por pull request.
  - Commits por etapa.
  - Kanban en GitHub Projects.

**Si el profesor pregunta:**
- *"¿Cómo ejecuto las pruebas?"* → `go test ./...` desde la raíz del repositorio.

---

## Diapositiva 16 — Conclusiones (Ximena)

**Guion:**
> "Concluimos cuatro cosas. Primero, Go procesó 1.2 millones de registros en segundos: la limpieza en 1.6 segundos y K-Means hasta 4.2 veces más rápido. Segundo, diseñar sin estado compartido dio resultados correctos y reproducibles sin usar mutex. Tercero, respondemos la pregunta: cinco zonas concentran más de la mitad de las muertes y la tarde y noche es la franja de mayor riesgo. Cuarto, el número de goroutines se decide midiendo: más hilos no siempre es más rápido."

---

## Diapositiva 17 — Limitaciones y siguientes pasos (Ximena)

**Guion:**
> "Reconocemos limitaciones: el augmentation no agrega información, la mitad de las causas siguen en investigación, Lima queda como una sola zona muy grande y medimos con la laptop en batería. Como siguientes pasos: un segundo K-Means dentro de Lima y, en la siguiente entrega, distribuir la fase Map en nodos TCP, con API, MongoDB, Redis y Docker."

**Teoría y justificación (por si preguntan cómo será la versión distribuida):**
- **Cada nodo TCP** tendrá su partición de puntos y ejecutará la misma función `Asignar`.
- **El master**, en cada iteración:
  1. envía los centroides a cada nodo;
  2. recibe las sumas parciales (Σx, Σy, conteo, SSE);
  3. hace el reduce.
- **Hoy los mensajes viajan por channels; mañana viajarán por sockets.** El algoritmo no cambia.
- **Partición dinámica:** dividir en bloques pequeños asignados bajo demanda, para que los núcleos rápidos no esperen a los lentos.

---

## Diapositiva 18 — Cierre (Ximena)

> "Muchas gracias por su atención. Quedamos atentas a sus preguntas."

---

# Preguntas difíciles: respuestas cortas para memorizar

| Pregunta | Respuesta |
|---|---|
| ¿Qué es concurrencia y qué es paralelismo? | **Concurrencia:** estructurar el programa en tareas independientes que pueden avanzar intercaladas. **Paralelismo:** ejecutarlas al mismo tiempo en varios núcleos. Go da concurrencia con goroutines y channels; hay paralelismo cuando `GOMAXPROCS` > 1 y hay núcleos libres. |
| ¿Por qué Go? | El curso lo exige. Además, las goroutines son baratas y los channels evitan memoria compartida. Compila a un binario nativo que se puede compilar para otro sistema operativo (`GOOS=linux`), por eso pudimos ejecutar en WSL sin instalar Go. |
| ¿Qué pasa si un channel no se cierra? | El consumidor que hace `range` sobre él se bloquea para siempre. Si todas las goroutines quedan bloqueadas, Go detecta un *deadlock*. Por eso cerramos cada channel después del `WaitGroup.Wait()`. |
| ¿Channel con buffer o sin buffer? | Sin buffer, el envío espera al receptor (sincronización). Con buffer, el emisor puede adelantarse hasta llenar el buffer, lo que suaviza diferencias de velocidad entre etapas del pipeline. |
| ¿Dónde está la sección crítica? | No hay: diseñamos para no tenerla. Cada dato modificable tiene un solo dueño (worker, shard, escritor o master). |
| ¿Qué es *deadlock* y cómo lo evitaron? | Es una espera circular: cada goroutine espera algo que otra retiene. Lo evitamos con un flujo en una sola dirección (pipeline sin ciclos) y cerrando los channels en orden. |
| ¿Speedup superlineal? | No lo tuvimos. El 10x del primer benchmark era ruido de la batería y se descartó. |
| ¿Por qué no usar todos los datos sin augmentation? | Porque 9,105 filas no cumplen el requisito de más de 1 M y no permiten medir el paralelismo: se procesarían en milisegundos. |
| ¿El modelo es "distribuido"? | Hoy es **paralelo en memoria compartida**, con workers que imitan nodos. El diseño ya es MapReduce: la siguiente entrega reemplaza los channels por TCP. |
| ¿Cuánto tarda todo? | Cargar el CSV limpio, unos 0.5 s; limpiar, unos 1.6 s; K-Means con k = 15, 13 iteraciones de unos 15 ms cada una con 12 workers. |
| ¿Por qué la mediana y no el promedio? | La mediana no se deja arrastrar por una corrida atípica (por ejemplo, un cambio de frecuencia de la CPU). |

---

# El informe, sección por sección (lo que dice y por qué)

**Archivo:** `docs/1ACC0065_PC3_202610_Informe_UXXXXXXXX.docx` (19 páginas, APA 7).

| Sección | Qué contiene | Diapositivas |
|---|---|---|
| **Resumen** | Problema, método (jittering, cargador, K-Means MapReduce), cifras clave (1.6 s, 95.6 %, 4.5x y 4.0–4.2x, k = 15) y hallazgos (19.9 %, 56.7 %, 17–22 h) | Todas |
| **1.1 Descripción del problema** | OMS y Defensoría: los siniestros como problema de salud pública | 2 |
| **1.2 Motivación** | Optimizar la asignación de recursos del Estado y demostrar el valor de un sistema concurrente | 2 |
| **1.3 Pregunta de impacto** | La pregunta y sus beneficiarios | 3 |
| **2. Objetivos** | El general y los 5 específicos | 4 |
| **3.1 Arquitectura** | Tabla de etapas Python → Go | 6 |
| **3.2 Limpieza y análisis** | Dataset original, nulos, augmentation, ensuciado, reglas de limpieza, resultados e inyectado frente a detectado | 5, 7, 8 |
| **3.3 Diseño del modelo** | Variables, proyección a km, K-Means++, convergencia y codo (Figura 1) | 10, 11 |
| **3.4 Paralelización** | Pipeline del cargador, MapReduce de K-Means, cómo se evitan las carreras, benchmarks y justificación de 8 y 12 goroutines | 9, 10, 12 |
| **3.5 Evidencias** | Archivos de código, pruebas, comandos de ejecución, mapa y tabla de las 15 zonas | 13–15 |
| **3.6 Gestión en GitHub** | Git Flow, tabla de commits (incluye el merge del PR #2) y Kanban | 15 |
| **4. Conclusiones** | 5 conclusiones. Además de las de la diapositiva 16, una sobre Amdahl y la CPU híbrida | 16 |
| **5. Recomendaciones** | Segundo nivel en Lima, validar con un dataset real más grande (por ejemplo, SUTRAN), benchmarks conectados, partición dinámica, TCP + MongoDB/Redis + API, ramas `feature/*`, `go test -race` y pedir menos causas "en investigación" | 17 |
| **Bibliografía** | Amdahl (1967), Arthur y Vassilvitskii (2007, K-Means++), Dean y Ghemawat (2008, MapReduce), Defensoría (2024), Lloyd (1982, K-Means), ONSV (2025), OMS (2023), Satopää et al. (2011, Kneedle), Effective Go | — |
| **Anexos** | A: enlaces (repositorio y video); B: Kanban; C: reporte de participación; D: bitácora | — |

**Correcciones que hicimos al borrador original** (por si Ximena o el profesor lo comparan):

| El borrador decía | Lo implementado | Por qué |
|---|---|---|
| Dataset "Personas involucradas…" | "BBDD ONSV – Siniestros fatales 2021-2025" | Es el archivo que realmente usamos |
| SDV GaussianCopula | Jittering | La cópula puede crear zonas falsas |
| Z-Score de latitud y longitud | Proyección a km | Z-Score deforma las distancias |
| Clima y vía con One-Hot dentro del modelo | Solo latitud y longitud | Las variables categóricas sirven para describir las zonas, no para agruparlas |
| "más de 1.2 M de registros históricos" | 9,105 reales y 1.2 M aumentados | Honestidad sobre los datos |

**Pendientes del informe antes de entregarlo (hoy, 23:30):**
- [ ] Poner el código de alumno en el nombre del archivo (`UXXXXXXXX`).
- [ ] Agregar el enlace del video (Anexo A).
- [ ] Insertar la captura del Kanban (Anexo B).
- [ ] Confirmar el reporte de participación con Ximena (Anexo C).
