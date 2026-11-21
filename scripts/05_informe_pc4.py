"""Genera docs/1ACC0065_PC4_202610_Informe.docx a partir del informe de la PC3.

Conserva todo el contenido de la PC3 (con sus correcciones) y agrega las secciones
de la PC4 antes de las Conclusiones. Uso: python scripts/05_informe_pc4.py
"""
import copy
import os

import docx
from docx.shared import Cm
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.patches import FancyBboxPatch

RAIZ = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(RAIZ, "docs", "1ACC0065_PC3_202610_Informe_UXXXXXXXX.docx")
OUT = os.path.join(RAIZ, "docs", "1ACC0065_PC4_202610_Informe_UXXXXXXXX.docx")
IMG = os.path.join(RAIZ, "docs", "graficos", "topologia_cluster.png")


# ---------------------------------------------------------------- diagrama
def diagrama():
    fig, ax = plt.subplots(figsize=(8, 4.2))
    ax.set_xlim(0, 10); ax.set_ylim(0, 6); ax.axis("off")

    def caja(x, y, w, h, txt, color):
        ax.add_patch(FancyBboxPatch((x, y), w, h, boxstyle="round,pad=0.05", fc=color, ec="#333", lw=1.2))
        ax.text(x + w / 2, y + h / 2, txt, ha="center", va="center", fontsize=9)

    caja(3.6, 2.3, 2.8, 1.4, "API coordinadora (Go)\n:8080  JWT · REST · WS\nREDUCE de K-Means", "#cfe3ff")
    for i, x in enumerate((0.3, 3.7, 7.1)):
        caja(x, 0.2, 2.6, 1.1, f"nodo{i+1} (Go)\n:9000  MAP de K-Means\npartición {i+1}/3", "#d8f0d0")
        ax.annotate("", xy=(x + 1.3, 1.35), xytext=(5.0, 2.25),
                    arrowprops=dict(arrowstyle="<->", color="#333", lw=1.4))
    caja(0.3, 4.4, 2.6, 1.1, "PostgreSQL :5432\nzonas · siniestros\nentrenamientos · consultas", "#ffe9b8")
    caja(7.1, 4.4, 2.6, 1.1, "Redis :6379\ncaché TTL 60 s", "#f7c9c9")
    ax.annotate("", xy=(2.9, 4.6), xytext=(3.9, 3.55), arrowprops=dict(arrowstyle="<->", color="#333", lw=1.4))
    ax.annotate("", xy=(7.1, 4.6), xytext=(6.1, 3.55), arrowprops=dict(arrowstyle="<->", color="#333", lw=1.4))
    ax.text(5, 5.5, "Cliente: Postman / Swagger / navegador (HTTP, WebSocket)", ha="center", fontsize=9, style="italic")
    ax.annotate("", xy=(5.0, 3.75), xytext=(5.0, 5.3), arrowprops=dict(arrowstyle="<->", color="#333", lw=1.4))
    ax.text(5.15, 1.75, "TCP + gob\n(broadcast de centroides / parciales)", fontsize=8, color="#333")
    plt.tight_layout()
    plt.savefig(IMG, dpi=160)
    plt.close()


# ---------------------------------------------------------------- helpers
d = docx.Document(SRC)
P = d.paragraphs
T = {k: copy.deepcopy(v._p) for k, v in {  # plantillas (copias antes de modificar)
    "h2": P[216], "h3": P[130], "body": P[217], "bullet": P[223], "code": P[136],
    "cap": P[218], "captitle": P[219], "note": P[62], "empty": P[220], "center": P[27],
}.items()}
TPL = {"2": d.tables[13], "3": d.tables[11], "4": d.tables[6]}
ANCLA = P[222]  # "4. Conclusiones": todo lo nuevo se inserta antes
tabn = [15]
fign = [4]


def clon(tpl, texto):
    p = copy.deepcopy(tpl)
    ANCLA._p.addprevious(p)
    par = docx.text.paragraph.Paragraph(p, ANCLA._parent)
    runs = par.runs
    if runs and texto.startswith("Nota.") and len(runs) >= 2:
        runs[0].text = "Nota."
        runs[1].text = texto[5:]
        for r in runs[2:]:
            r._r.getparent().remove(r._r)
    elif runs:
        runs[0].text = texto
        for r in runs[1:]:
            r._r.getparent().remove(r._r)
    else:
        par.add_run(texto)
    return par


def h2(t): clon(T["h2"], t)
def h3(t): clon(T["h3"], t)
def body(t): clon(T["body"], t)
def bullet(t): clon(T["bullet"], t)
def vacio(): clon(T["empty"], "")


def codigo(txt):
    for ln in txt.strip("\n").split("\n"):
        clon(T["code"], ln)
    vacio()


def tabla(titulo, cab, filas, plantilla="2", nota=None):
    clon(T["cap"], f"Tabla {tabn[0]}")
    clon(T["captitle"], titulo)
    tabn[0] += 1
    t = copy.deepcopy(TPL[plantilla]._tbl)
    ANCLA._p.addprevious(t)
    tb = docx.table.Table(t, ANCLA._parent)
    rows = tb.rows
    molde = copy.deepcopy(rows[1]._tr)
    for r in list(rows)[1:]:
        t.remove(r._tr)

    def poner(tr_row, vals):
        for c, v in zip(tr_row.cells, vals):
            pars = c.paragraphs
            if pars[0].runs:
                pars[0].runs[0].text = v
                for r in pars[0].runs[1:]:
                    r._r.getparent().remove(r._r)
            else:
                pars[0].add_run(v)
            for extra in pars[1:]:
                extra._p.getparent().remove(extra._p)

    poner(tb.rows[0], cab)
    for f in filas:
        tr = copy.deepcopy(molde)
        t.append(tr)
        poner(tb.rows[-1], f)
    if nota:
        clon(T["note"], nota)
    vacio()


def figura(titulo, ruta=None, placeholder=None):
    if ruta:
        p = clon(T["center"], "")
        p.add_run().add_picture(ruta, width=Cm(15))
    else:
        clon(T["body"], f"[Insertar captura: {placeholder}]")
    clon(T["cap"], f"Figura {fign[0]}")
    clon(T["captitle"], titulo)
    fign[0] += 1
    vacio()


# ---------------------------------------------------------------- portada y resumen
def reemplazar(par, texto):
    runs = par.runs
    if texto.startswith("Nota.") and len(runs) >= 2:
        runs[0].text, runs[1].text = "Nota.", texto[5:]
        runs = runs[1:]
    else:
        runs[0].text = texto
    for r in runs[1:]:
        r._r.getparent().remove(r._r)


reemplazar(P[11], "Entregable: PC4")
reemplazar(P[27], "Lima, 21 de noviembre de 2026")

resumen_pc4 = (
    "En la PC4 el modelo se llevó a un sistema distribuido. K-Means se ejecuta ahora en un clúster de tres nodos de "
    "cómputo en Go que se comunican por TCP con una API coordinadora (topología en estrella), todo orquestado con Docker. "
    "La API expone endpoints REST documentados con Swagger, autenticación JWT y métricas por WebSocket, y responde la "
    "predicción de riesgo en 0.17 ms de cálculo (p99 de 27 ms medido de extremo a extremo, por debajo de los 100 ms exigidos). "
    "Los resultados se persisten en PostgreSQL y las respuestas frecuentes se cachean en Redis. El informe incluye además "
    "las correcciones de la PC3 (mejora continua)."
)
nuevo = copy.deepcopy(P[33]._p)
P[33]._p.addnext(nuevo)
npar = docx.text.paragraph.Paragraph(nuevo, P[33]._parent)
reemplazar(npar, resumen_pc4)

# ---------------------------------------------------------------- mejoras continuas en la PC3
reemplazar(
    P[89],
    "Resultado: 9,105 registros reales + 1,192,755 sintéticos = 1,201,860 registros (≈ 292 MB). Se descartó SMOTE porque es una "
    "técnica de sobremuestreo para balancear clases en problemas supervisados, y K-Means no tiene clases. Es importante declarar "
    "una limitación: el augmentation amplifica la distribución espacial real pero no aporta información nueva; su función es "
    "demostrar el rendimiento paralelo. Corrección (mejora continua): los conteos marcados como «reales» filtran es_sintetico = 0, "
    "pero el porcentaje de fallecidos, la franja pico, la clase y la causa principal de cada zona se calculan sobre los 1,160,337 "
    "registros limpios (reales y sintéticos). Al recalcular solo con los 8,791 registros reales, Lima Metropolitana pasa de 19.86 % "
    "a 19.75 %, las cinco primeras zonas de 56.7 % a 56.4 %, 13 de las 15 franjas pico no cambian y solo varían Puno y Cajamarca. "
    "La conclusión no se altera.",
)
reemplazar(
    P[206],
    "Nota. Elaboración propia con datos del ONSV-MTC (2021–2025, preliminar). Los conteos «reales» filtran es_sintetico = 0; "
    "los porcentajes, franjas, clases y causas se calculan sobre los 1,160,337 registros limpios.",
)
reemplazar(
    P[62],
    "Nota. En la PC4 se añadieron los nodos TCP, la API REST/WebSocket, PostgreSQL, Redis y Docker (secciones 3.7 a 3.10). "
    "La interfaz web (SPA) queda para la entrega final.",
)

# ---------------------------------------------------------------- contenido PC4
diagrama()

h2("3.7. Distribución: clúster de nodos ML")
h3("3.7.1. Arquitectura del clúster")
body(
    "El clúster se compone de seis contenedores Docker conectados en una red interna llamada cluster (docker-compose.yml). "
    "Tres nodos de cómputo ejecutan la fase Map de K-Means; la API actúa como coordinador, ejecuta la fase Reduce y atiende a los clientes. "
    "PostgreSQL y Redis completan los dos niveles de almacenamiento. Solo la API, PostgreSQL y Redis publican puertos hacia el equipo anfitrión; "
    "los nodos solo son alcanzables desde la red interna."
)
figura("Topología en estrella del clúster de la PC4", ruta=IMG)
tabla(
    "Componentes del clúster",
    ["Componente", "Rol", "Tecnología / puerto"],
    [
        ["nodo1, nodo2, nodo3", "Fase Map de K-Means sobre su partición (2,930 / 2,930 / 2,931 puntos)", "Go, TCP 9000 (interno)"],
        ["api", "Coordinador: Reduce, REST, JWT, WebSocket, Swagger", "Go net/http, 8080"],
        ["postgres", "Almacenamiento persistente", "PostgreSQL 16, 5432"],
        ["redis", "Caché de respuestas", "Redis 7, 6379"],
    ],
    plantilla="3",
    nota="Nota. Elaboración propia. Los nodos reciben los 8,791 puntos reales del ONSV; el entrenamiento sobre los 1.16 M de registros se hizo en la PC3 en una sola máquina.",
)

h3("3.7.2. Justificación de la topología en estrella")
body(
    "Se eligió una topología en estrella (coordinador al centro, nodos como hojas) porque el patrón de comunicación de K-Means ya tiene esa forma:"
)
bullet("Los nodos nunca necesitan datos de otros nodos. En cada iteración el coordinador difunde los centroides y cada nodo devuelve solo un parcial (sumas, conteos y SSE). Enlaces nodo a nodo no aportarían nada.")
bullet("Latencia por iteración = el mayor RTT de un nodo, no la suma. Las consultas a los tres nodos salen en paralelo; el RTT medido por /health fue de 2 a 4 ms. En un anillo, la información pasaría por saltos secuenciales.")
bullet("Escala lineal en enlaces. Agregar un nodo implica un enlace nuevo y una dirección más en la variable NODOS. Una malla completa crece como n(n−1)/2.")
bullet("Reducción determinista en un único lugar. El coordinador suma los parciales en orden de nodo, lo que evita condiciones de carrera y hace el resultado reproducible.")
body(
    "Costo asumido: el coordinador es un punto único de falla y un posible cuello de botella. Se mitigó con nodos sin estado (se reinician con docker compose start), "
    "con un endpoint /health que informa los nodos activos y con una respuesta 503 explícita cuando falta un nodo, en lugar de devolver un resultado parcial incorrecto. "
    "No se implementó replicación del coordinador; se declara como limitación."
)
tabla(
    "Comparación de topologías para K-Means distribuido (coordinador + 3 nodos)",
    ["Topología", "Enlaces / saltos por iteración", "Evaluación"],
    [
        ["Estrella (adoptada)", "3 enlaces; 1 ida y vuelta en paralelo", "Calza con broadcast + reduce; simple de operar; coordinador único"],
        ["Anillo", "4 enlaces; saltos secuenciales entre nodos", "Suma latencias y complica la reducción sin ganar nada"],
        ["Malla completa", "6 enlaces (n(n−1)/2); varias rutas", "Complejidad innecesaria: los nodos no intercambian datos"],
    ],
    plantilla="3",
    nota="Nota. Elaboración propia.",
)

h3("3.7.3. Funcionamiento en Go")
body(
    "Nodo (cmd/nodo, internal/red). Cada nodo calcula su partición contigua [id·n/total, (id+1)·n/total) de los puntos proyectados a kilómetros y escucha en TCP. "
    "Por cada conexión lanza una goroutine que decodifica una Tarea (con gob), ejecuta kmeans.Asignar sobre sus puntos y responde un Parcial. "
    "La conexión es persistente: durante todo el entrenamiento se reutiliza, sin costo de reconexión por iteración."
)
codigo("""
// internal/red/red.go - nodo: una goroutine por conexión
go func(c net.Conn) {
    defer c.Close()
    dec, enc := gob.NewDecoder(c), gob.NewEncoder(c)
    asig := make([]int32, len(pts)) // buffer propio de la conexión
    for {
        var t Tarea
        if err := dec.Decode(&t); err != nil { return }
        resp := Respuesta{NodoID: id, N: len(pts)}
        if !t.Info { resp.P = kmeans.Asignar(pts, t.Cents, asig) }
        if err := enc.Encode(resp); err != nil { return }
    }
}(c)
""")
body(
    "Coordinador (red.Entrenar). En cada iteración se lanza una goroutine por nodo que envía los centroides y espera el parcial; los resultados vuelven por un channel con búfer. "
    "El coordinador guarda cada parcial en la posición de su nodo y recién entonces los suma, de modo que el orden de reducción es fijo."
)
codigo("""
// internal/red/red.go - coordinador: difusión en paralelo y reducción ordenada
ch := make(chan msg, len(nodos))
for i := range nodos {
    go func(i int) {
        n := nodos[i]
        n.c.SetDeadline(time.Now().Add(10 * time.Second))
        if err := n.enc.Encode(Tarea{Cents: cents}); err != nil { ch <- msg{i: i, err: err}; return }
        var r Respuesta
        err := n.dec.Decode(&r)
        ch <- msg{i, r, err}
    }(i)
}
parc := make([]kmeans.Parcial, len(nodos))
for range nodos { m := <-ch; parc[m.i] = m.r.P }   // REDUCE después, en orden de nodo
""")
body(
    "Sin condiciones de carrera: los nodos solo leen los centroides recibidos y escriben en su propio parcial; cada goroutine del coordinador escribe solo en su posición del slice; "
    "los centroides nuevos los calcula únicamente el coordinador; y un sync.Mutex (TryLock) impide dos entrenamientos simultáneos. "
    "Se agregó un plazo (SetDeadline) por iteración para que un nodo colgado no bloquee la API."
)
body(
    "Resultado medido con el clúster en Docker: k = 15 converge en 17 iteraciones y 15 ms. Con un nodo detenido, /health reporta 2 de 3 nodos activos y el entrenamiento responde 503."
)

h2("3.8. Desarrollo de la API")
h3("3.8.1. Endpoints y Swagger")
body(
    "La API está escrita en Go con el paquete estándar net/http. Se usan los patrones de ruta de Go 1.22 (por ejemplo GET /api/zonas/{ranking}). "
    "La documentación sigue OpenAPI 3.0 en el archivo internal/api/openapi.yaml, incrustado en el binario con go:embed y servido en /openapi.yaml; "
    "la interfaz Swagger UI está disponible en /swagger y permite probar cada endpoint con el botón Authorize para el token."
)
tabla(
    "Endpoints de la API",
    ["Método y ruta", "Autenticación", "Función"],
    [
        ["GET /health", "No", "Estado de API, PostgreSQL, Redis y nodos"],
        ["POST /api/login", "No", "Entrega un JWT (usuario y clave)"],
        ["GET /api/zonas · /api/zonas/{ranking}", "No", "Zonas de riesgo (caché Redis, cabecera X-Cache)"],
        ["POST /api/predict", "No", "Zona más cercana y nivel de riesgo para lat, lon y hora"],
        ["GET /api/siniestros", "No", "Lista siniestros con filtro por departamento"],
        ["POST /api/siniestros · DELETE /api/siniestros/{id}", "JWT", "Registra o elimina un siniestro"],
        ["POST /api/entrenar", "JWT", "Entrena K-Means distribuido en los nodos y guarda el resultado"],
        ["GET /api/entrenamientos", "JWT", "Historial de entrenamientos"],
        ["GET /api/admin/metricas · WS /ws/metricas", "JWT", "Métricas del clúster (goroutines, memoria, p50/p95, nodos)"],
    ],
    plantilla="3",
    nota="Nota. Elaboración propia. Usuarios de prueba: admin y analista.",
)

h3("3.8.2. Funcionamiento en Go")
bullet("Middleware (mw). Agrega CORS, mide cada petición y escribe la cabecera X-Response-Time justo antes de enviar la respuesta (con un http.ResponseWriter envuelto). Registra la latencia por ruta en un contenedor protegido con sync.Mutex y contadores sync/atomic.")
bullet("JWT. Se implementó HS256 con crypto/hmac y crypto/sha256: se firma cabecera y carga con un secreto, se valida la firma con hmac.Equal y se revisa la expiración (2 horas). Las rutas protegidas se envuelven con s.auth.")
bullet("Predicción en memoria. Al arrancar, la API carga las zonas de PostgreSQL. POST /api/predict no consulta la base: busca la zona más cercana (distancia euclidiana en km) y combina el peso de la zona con el perfil horario: riesgo = 0.6·(% fallecidos de la zona / máximo) + 0.4·(siniestros de esa hora / pico de la zona).")
bullet("Registro asíncrono. Cada predicción se guarda en la tabla consultas mediante un channel con búfer atendido por una goroutine; si el búfer se llena, se descarta el registro y nunca se retrasa la respuesta.")
bullet("WebSocket. /ws/metricas (gorilla/websocket) empuja las métricas cada segundo a quien se conecte con un token válido en ?token=.")
codigo("""
// internal/api/server.go - registro asíncrono fuera de la ruta crítica
select {
case s.logCons <- consulta{*b.Lat, *b.Lon, hora, z.Ranking, riesgo, ms}:
default: // búfer lleno: se prefiere perder el registro antes que sumar latencia
}
""")

h3("3.8.3. Latencia menor a 100 ms")
body(
    "La medición se hizo con scripts/latencia.sh: 500 peticiones consecutivas a POST /api/predict contra el clúster en Docker, midiendo el tiempo total que reporta curl "
    "(incluye el arranque del proceso curl, por lo que sobrestima la latencia real del servidor). Postman reporta el mismo tiempo en cada petición y la colección valida que sea menor a 100 ms."
)
tabla(
    "Latencia de POST /api/predict (500 peticiones)",
    ["Métrica", "Valor"],
    [
        ["Cálculo dentro del servidor (calculo_ms / X-Response-Time)", "≈ 0.17 – 0.19 ms"],
        ["Tiempo total p50", "5.6 ms"],
        ["Tiempo total p95", "24.0 ms"],
        ["Tiempo total p99", "27.4 ms"],
        ["Tiempo total máximo", "29.4 ms"],
        ["Requisito", "< 100 ms: se cumple"],
    ],
    nota="Nota. Elaboración propia. Equipo local con Docker Desktop; la red es el bucle local.",
)

h2("3.9. Implementación de bases de datos")
h3("3.9.1. Modelo en PostgreSQL")
body(
    "PostgreSQL guarda los datos persistentes. El esquema se crea y se carga automáticamente al iniciar la API (store.go), de forma idempotente: si las tablas ya tienen datos no se vuelven a sembrar. "
    "Los datos viven en el volumen de Docker pgdata y sobreviven a reinicios. La tabla siniestros referencia a zonas mediante una clave foránea (zona_ranking)."
)
tabla(
    "Tablas de la base de datos",
    ["Tabla", "Registros", "Contenido"],
    [
        ["usuarios", "2", "Usuario, hash SHA-256 de la clave y rol (admin, lector)"],
        ["zonas", "15", "Zonas de riesgo del K-Means: centro, siniestros, fallecidos, franja pico, clase, causa y perfil por hora"],
        ["siniestros", "8,791 (+ altas)", "Siniestros reales con coordenadas, hora, fallecidos, departamento y zona asignada"],
        ["entrenamientos", "1 por ejecución", "k, iteraciones, SSE, convergencia, duración, nodos y centroides (JSONB)"],
        ["consultas", "1 por predicción", "Historial de /api/predict con su latencia"],
    ],
    plantilla="3",
    nota="Nota. Elaboración propia.",
)
h3("3.9.2. Caché en Redis")
body(
    "Redis actúa como caché lateral (cache-aside) para GET /api/zonas y GET /api/zonas/{ranking}, con vigencia de 60 segundos. La primera consulta devuelve X-Cache: MISS y las siguientes X-Cache: HIT. "
    "Si Redis no responde, la API sigue funcionando y devuelve X-Cache: BYPASS."
)
h3("3.9.3. Decisión: PostgreSQL y Redis en lugar de MongoDB")
body(
    "El documento del curso sugiere MongoDB como almacenamiento persistente y Redis como caché. El equipo eligió PostgreSQL porque los datos son tabulares y relacionales "
    "(siniestros pertenecen a una zona), se consultan con filtros y agregaciones, y se necesita integridad referencial. Redis se mantuvo como segundo nivel."
)

h2("3.10. Evidencias de la implementación (PC4)")
h3("3.10.1. Datos de prueba")
body(
    "Usuarios: admin / admin123 (puede escribir) y analista / analista123. Predicción de ejemplo: POST /api/predict con {\"lat\": -12.0464, \"lon\": -77.0428, \"hora\": 18} devuelve la zona 1 "
    "(San Martín de Porres, Lima), distancia 9.8 km, riesgo 0.865 y nivel ALTO. Siniestro de prueba: {\"lat\": -12.07, \"lon\": -77.05, \"hora\": 19, \"fallecidos\": 1, \"departamento\": \"LIMA\"}, "
    "que PostgreSQL registra con id 8792 y zona 1. Los casos de error comprobados son 401 (sin token o clave errónea), 404 (zona 99) y 400 (coordenadas fuera del Perú)."
)
tabla(
    "Archivos de la PC4",
    ["Archivo", "Contenido"],
    [
        ["cmd/nodo/main.go", "Nodo de cómputo ML"],
        ["cmd/api/main.go", "Arranque de la API (Postgres, Redis, nodos)"],
        ["internal/red/red.go", "Protocolo TCP y K-Means distribuido"],
        ["internal/api/server.go · store.go", "Rutas, caché, CRUD, entrenamiento · esquema y siembra"],
        ["internal/api/auth.go · metricas.go", "JWT HS256 · métricas por ruta"],
        ["internal/api/openapi.yaml", "Documentación Swagger / OpenAPI"],
        ["Dockerfile · docker-compose.yml", "Imagen y orquestación de los 6 contenedores"],
        ["docs/postman/ · scripts/latencia.sh", "Colección de Postman (16 pruebas) y medición de latencia"],
        ["docs/GUIA_DEMO.md", "Guía de la demostración en vivo"],
    ],
    nota="Nota. Elaboración propia.",
)
h3("3.10.2. Pruebas")
body(
    "Se agregó la prueba unitaria TestJWT (internal/api/auth_test.go), que verifica que un token válido se acepta, que una firma distinta se rechaza y que un token expirado se rechaza. "
    "La colección de Postman contiene 16 peticiones con pruebas automáticas (código de estado y latencia menor a 100 ms)."
)
h3("3.10.3. Capturas de pantalla")
body("Comando para reproducir el entorno: docker compose up -d --build. Las capturas siguientes corresponden a la ejecución en vivo.")
figura("Contenedores del clúster en ejecución (docker compose ps)", placeholder="terminal con docker compose ps, 6 contenedores Up")
figura("Swagger UI con los endpoints documentados (/swagger)", placeholder="navegador en http://localhost:8080/swagger")
figura("Postman: POST /api/login y token JWT", placeholder="Postman, login con status 200 y el token")
figura("Postman: POST /api/predict con tiempo de respuesta menor a 100 ms", placeholder="Postman, predict con Status 200 y Time en ms")
figura("Postman: colección completa ejecutada (Run collection)", placeholder="Postman Runner con las 16 pruebas en verde")
figura("Postman: GET /api/zonas con X-Cache MISS y HIT", placeholder="Postman, pestaña Headers de dos llamadas consecutivas")
figura("Postman: POST /api/entrenar distribuido en 3 nodos", placeholder="Postman, respuesta con nodos: 3 y convergio: true")
figura("Respuesta de /health con los tres nodos activos", placeholder="Postman o navegador en /health")
figura("Consultas en PostgreSQL (siniestros, entrenamientos, consultas)", placeholder="psql o DBeaver con los SELECT de la guía")

h2("3.11. Mejoras continuas respecto de la PC3")
tabla(
    "Observaciones de la PC3 y su corrección",
    ["Pendiente o error en la PC3", "Cómo se atendió en la PC4"],
    [
        ["La frase de la sección 3.2.5 indicaba que las cifras de impacto se calculaban solo con registros reales, pero el porcentaje de fallecidos, franja pico, clase y causa usan los 1.16 M de registros", "Se corrigió el texto y la nota de la Tabla 12; se recalculó solo con reales (Lima 19.75 %, cinco primeras zonas 56.4 %, 13 de 15 franjas iguales)"],
        ["La presentación decía «k de 2 a 30», franjas pico generalizadas y «mediana de 5» para todos los benchmarks", "Se corrigió: k de 5 a 60 en pasos de 5, franjas exactas por zona y mediana de 3 en el cargador (docs/GUION_DEFENSA.md)"],
        ["Los workers debían reemplazarse por nodos TCP", "Implementado: tres nodos Go en contenedores que se comunican por TCP con el coordinador"],
        ["Faltaban API, base de datos, caché y Docker", "Implementados: API con Swagger y JWT, PostgreSQL, Redis y docker-compose"],
        ["Ejecutar go test -race", "Se ejecutó en un contenedor Linux (golang:1.26, con gcc): las pruebas de api, kmeans y limpieza pasan sin condiciones de carrera"],
        ["Repetir los benchmarks con el equipo conectado a la corriente", "No se repitieron; se mantiene como limitación declarada (los benchmarks de la PC3 se hicieron con batería)"],
    ],
    nota="Nota. Elaboración propia.",
)

# ---------------------------------------------------------------- conclusiones / recomendaciones / anexos
ult = P[227]
for txt in (
    "Se distribuyó K-Means en tres nodos Go conectados por TCP a un coordinador. La topología en estrella calza con el patrón broadcast, map y reduce, y el entrenamiento converge en 17 iteraciones y 15 ms.",
    "La API en Go responde la predicción en 0.17 ms de cálculo y con un p99 de 27 ms de extremo a extremo, cumpliendo el requisito de 100 ms. Se documentó con Swagger y se protegió con JWT.",
    "PostgreSQL aporta persistencia relacional y Redis reduce la lectura repetida de zonas; la solución es reproducible con un solo comando de Docker.",
):
    n = copy.deepcopy(ult._p)
    ult._p.addnext(n)
    ult = docx.text.paragraph.Paragraph(n, ult._parent)
    reemplazar(ult, txt)

reemplazar(
    P[233],
    "Para la entrega final, desarrollar la interfaz SPA (inicio de sesión, panel y métricas del clúster por WebSocket), replicar el coordinador para eliminar el punto único de falla "
    "y entrenar los nodos con el dataset completo de 1.16 M de registros en lugar de los 8,791 reales.",
)

# Anexo C: reporte de participación (PC3 + PC4)
tb = d.tables[14]
reemplazar(P[252], "Tabla " + str(tabn[0]))  # renumera
tb.rows[1].cells[1].paragraphs[0].runs[0].text = (
    "PC3: preparación de datos, data augmentation y ensuciado intencional (Python); cargador concurrente y K-Means paralelo en Go; pruebas unitarias; benchmarks en Windows y WSL; gráficos; gestión del repositorio; bitácora. "
    "PC4: nodos TCP y K-Means distribuido; API en Go (JWT, Swagger, WebSocket); esquema de PostgreSQL y caché Redis; Docker y docker-compose; colección de Postman y medición de latencia; guía de demostración; pruebas; bitácora; redacción de las secciones 3.7 a 3.11."
)
for r in tb.rows[1].cells[1].paragraphs[0].runs[1:]:
    r._r.getparent().remove(r._r)
tb.rows[2].cells[1].paragraphs[0].runs[0].text = (
    "PC3: presentación del caso, motivación y pregunta de impacto social; investigación bibliográfica; borrador de limpieza y análisis de datos; redacción del informe. "
    "PC4: despliegue del clúster en su equipo con Docker; capturas de Postman, Swagger y PostgreSQL; verificación de la guía de demostración; revisión del informe; actualización del tablero Kanban."
)
for r in tb.rows[2].cells[1].paragraphs[0].runs[1:]:
    r._r.getparent().remove(r._r)

d.save(OUT)
print("OK", OUT, "tablas hasta", tabn[0], "figuras hasta", fign[0])
