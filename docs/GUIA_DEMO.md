# Guía de la demo en vivo (PC4)

Orden pensado para ~5 min. Cada paso muestra **qué ejecutar**, **qué debe verse** y **qué cambia**.
Alternativa a curl: importar `docs/postman/PC4_API_Siniestros.postman_collection.json` en Postman y pulsar *Run collection* (cada request valida latencia < 100 ms).

## 0. Antes de exponer (hacerlo 10 min antes)
```powershell
docker compose up -d --build        # levanta 3 nodos ML + API + Postgres + Redis
docker compose ps                   # los 6 contenedores deben estar Up
```
Abrir en el navegador: **http://localhost:8080/swagger** (necesita internet para cargar Swagger UI).
Usuarios de prueba: `admin / admin123` (escribe), `analista / analista123`.

## 1. Clúster vivo — `GET /health`
```powershell
curl.exe -s http://localhost:8080/health
```
Debe verse: `"nodos_activos":3`, `"postgres":"ok"`, `"redis":"ok"`, y los 3 nodos con ~2 930 puntos cada uno (partición de 8 791 puntos reales) y `rtt_ms` de 2–4 ms.
**Demo de tolerancia:** `docker compose stop nodo3` → repetir: `nodos_activos":2`. Luego `docker compose start nodo3`.

## 2. Autenticación JWT — `POST /api/login`
```powershell
curl.exe -s -X POST http://localhost:8080/api/login -H "Content-Type: application/json" -d '{\"usuario\":\"admin\",\"clave\":\"admin123\"}'
```
Devuelve `{"token":"eyJ...","tipo":"Bearer","expira_en_s":7200,"rol":"admin"}`. Guardar el token:
```powershell
$T = (curl.exe -s -X POST http://localhost:8080/api/login -H "Content-Type: application/json" -d '{\"usuario\":\"admin\",\"clave\":\"admin123\"}' | ConvertFrom-Json).token
```
Con clave mala → **401** `{"error":"credenciales inválidas"}`.

## 3. Caché Redis — `GET /api/zonas`
```powershell
curl.exe -s -i http://localhost:8080/api/zonas | Select-String "X-Cache|X-Response-Time"
```
Primera vez: `X-Cache: MISS`; repetir: `X-Cache: HIT`. Dura 60 s. `GET /api/zonas/1` da el detalle de la zona más crítica (SAN MARTIN DE PORRES, LIMA, 19.86 % de fallecidos); `/api/zonas/99` → **404**.

## 4. La predicción (< 100 ms) — `POST /api/predict`
```powershell
curl.exe -s -i -X POST http://localhost:8080/api/predict -d '{\"lat\":-12.0464,\"lon\":-77.0428,\"hora\":18}'
```
Respuesta real obtenida: `zona_ranking:1`, `"SAN MARTIN DE PORRES, LIMA"`, `distancia_km:9.8`, `riesgo:0.865`, `nivel:"ALTO"`, `calculo_ms:0.17`. La cabecera `X-Response-Time` muestra ~0.2 ms (servidor).
**Cambios para mostrar que responde a la entrada:** cambiar `hora` a 3 (baja el riesgo), o la ubicación a Cusco (`-13.5319,-71.9675`), o coordenadas de España (`40.4,-3.7`) → **400** "coordenadas fuera del Perú".
Medición de latencia total (500 peticiones, incluye arranque de curl): `bash scripts/latencia.sh 500` → p50 ≈ 5.6 ms, p99 ≈ 27 ms → **CUMPLE < 100 ms**.

## 5. Base de datos: leer, crear, borrar — `/api/siniestros`
```powershell
curl.exe -s "http://localhost:8080/api/siniestros?departamento=LIMA&limit=3"            # total real LIMA
curl.exe -s -X POST http://localhost:8080/api/siniestros                                 # sin token → 401
curl.exe -s -X POST http://localhost:8080/api/siniestros -H "Authorization: Bearer $T" -d '{\"lat\":-12.07,\"lon\":-77.05,\"hora\":19,\"fallecidos\":1,\"departamento\":\"lima\"}'
```
El POST con token devuelve **201** `{"id":8792,"zona_ranking_asignada":1,...}`. Repetir el GET: `total` sube en 1 y el nuevo id aparece primero.
Borrar: `curl.exe -s -X DELETE http://localhost:8080/api/siniestros/8792 -H "Authorization: Bearer $T"` → `{"eliminado":"8792"}`; el total vuelve a bajar. Verificación directa en Postgres:
```powershell
docker compose exec postgres psql -U siniestros -c "SELECT count(*) FROM siniestros;" -c "SELECT id,k,iteraciones,duracion_ms FROM entrenamientos;" -c "SELECT count(*), round(avg(latencia_ms)::numeric,3) AS ms_prom FROM consultas;"
```

## 6. Lo distribuido: entrenar en los 3 nodos — `POST /api/entrenar`
```powershell
curl.exe -s -X POST http://localhost:8080/api/entrenar -H "Authorization: Bearer $T" -d '{\"k\":15,\"max_iter\":50}'
```
Debe verse `"nodos":3`, `iteraciones`, `convergio:true`, `duracion_ms` y los 15 centroides con sus puntos. Cada iteración: la API difunde los centroides por TCP, cada nodo responde su parcial (MAP) y la API suma (REDUCE). Con `{"k":5}` salen 5 centroides. Sin token → **401**. Historial: `GET /api/entrenamientos` (queda guardado en Postgres).
**Con un nodo caído** (`docker compose stop nodo2`) → **503** `nodo nodo2:9000 no disponible` (se explica que no se devuelven resultados parciales falsos).

## 7. Panel admin y tiempo real
```powershell
curl.exe -s http://localhost:8080/api/admin/metricas -H "Authorization: Bearer $T"
```
Muestra uptime, peticiones, goroutines, memoria, p50/p95 por ruta y estado de nodos. WebSocket (empuja cada 1 s): en Postman → *New → WebSocket* → `ws://localhost:8080/ws/metricas?token=<TOKEN>`.

## Si algo falla
| Síntoma | Causa / arreglo |
|---|---|
| `docker compose` no conecta | Abrir Docker Desktop y esperar a que diga *Running* |
| Swagger en blanco | Sin internet (carga Swagger UI desde unpkg); usar Postman |
| `/health` con nodo `activo:false` | `docker compose start nodoN` |
| Puerto 8080/5432/6379 ocupado | Cerrar el programa que lo usa o cambiar el puerto en `docker-compose.yml` |
| Reiniciar datos desde cero | `docker compose down -v && docker compose up -d --build` |
