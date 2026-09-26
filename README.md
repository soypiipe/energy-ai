# Voltix · AI Energy Management Platform

MVP que convierte lecturas de medidores eléctricos en decisiones operativas: detecta anomalías,
las explica con evidencia, las prioriza y recomienda una acción.

No es un CRUD: es una herramienta de **triage**. El operador entiende en segundos **qué medidor
atender primero, por qué y qué hacer**.

```
DATOS → ANÁLISIS → ANOMALÍA → EXPLICACIÓN → PRIORIZACIÓN → ACCIÓN
```

## Arranque rápido

Requisitos: Docker con Docker Compose (y conexión a internet la primera vez, para bajar las imágenes).

```bash
cp .env.example .env                 # 1. crea tu configuración
# 2. edita .env (ver "Configurar .env" abajo: 3 valores)
docker compose up --build            # 3. levanta todo (la primera vez tarda unos minutos)
```

Luego abre **http://localhost:5173** e inicia sesión con:

| Usuario | Contraseña |
|---|---|
| `demo` | la que hayas elegido en el paso 2 de "Configurar .env" |

| Servicio | URL |
|---|---|
| Web (frontend) | http://localhost:5173 |
| API | http://localhost:8080 |

Al primer arranque la API aplica las migraciones y carga `data/*.csv` (solo si la base está vacía).

### Configurar .env

Abre `.env` y cambia estos tres valores (el resto puede quedar igual):

1. **`POSTGRES_PASSWORD`**: cualquier texto, es la clave de la base de datos.
2. **`AUTH_PASSWORD_HASH`**: el hash de la contraseña con la que vas a entrar (el sistema no guarda la
   contraseña en claro, solo su hash bcrypt). Para crearlo:
   1. Elige una contraseña de 8 o más caracteres, por ejemplo `Voltix-demo-2026`.
   2. Genera su hash (reemplaza el texto entre comillas por tu contraseña):
      ```bash
      echo -n 'Voltix-demo-2026' | docker run --rm -i -v "$PWD/backend":/src \
        -v energy-ai-gomod:/go/pkg/mod -w /src golang:1.24-alpine go run ./cmd/hashpw
      ```
      Imprime una línea que empieza por `$2a$10$...` (la primera vez tarda porque descarga Go).
   3. Pega esa línea en `.env` **entre comillas simples**, porque el hash lleva `$`:
      ```
      AUTH_PASSWORD_HASH='$2a$10$...lo-que-imprimió-el-comando...'
      ```
   4. Esa contraseña (`Voltix-demo-2026` en el ejemplo) es la que escribes en el login.
3. **`JWT_SECRET`**: texto aleatorio de 32 o más caracteres. Con `openssl rand -hex 32`, o cualquier
   cadena larga que inventes.

Opcional: `LLM_API_KEY` y `LLM_MODEL` para explicaciones redactadas por un LLM (ver "LLM opcional"). Sin ellos
el análisis funciona igual con una plantilla.

### Si algo falla

| Síntoma | Causa y solución |
|---|---|
| `define AUTH_PASSWORD_HASH en .env` / `define JWT_SECRET...` / `define POSTGRES_PASSWORD...` | Falta el `.env` o un valor: ejecuta `cp .env.example .env` y completa los tres valores de arriba |
| `port is already allocated` / `address already in use` | El 5173 o el 8080 están ocupados. Pon en `.env` otros puertos, por ejemplo `WEB_HOST_PORT=5174` y `API_HOST_PORT=8081` (con el web en 5174 añade también `ALLOWED_ORIGINS=http://localhost:5174`) |
| La API se reinicia o el login da "credenciales inválidas" | El hash está mal pegado: debe ir entre comillas simples y completo (`$2a$10$...`, 60 caracteres). Regénéralo y reinicia con `docker compose up --build` |
| Quiero empezar de cero | `docker compose down -v && docker compose up --build` (el `-v` borra la base) |

### Recorrido de la demo

Login → **Dashboard** → **Medidores** → detalle de **M-109** → **Run AI Analysis** → anomalía →
explicación y acción recomendada.

Resultado esperado del análisis sobre los datos incluidos:

| Medidor | Tipo | Severidad | Por qué |
|---|---|---|---|
| M-109 | Anomalía real | Alta (1.º) | +110% de consumo desde el 12-sep 14:00, corriente ×2, FP 0,94→0,74, sin evento que lo explique |
| M-112 | Calidad de datos | Alta | El consumo no cuadra con V·I·FP: sensor incoherente desde el 13-sep |
| M-104 | Explicable | Media | +47% desde el 11-sep; coincide con "nueva línea de producción" |
| M-106 | Falso positivo | Baja | Caída de 12 h el 8-sep; coincide con mantenimiento programado |
| Los otros 8 | — | — | Cero anomalías |

## Arquitectura

```
Vue SPA ──/api (nginx)──► API Go (net/http)
                            ├── meter      lectura de medidores, lecturas y eventos
                            ├── analysis   API + cola + worker + motor puro (engine/)
                            ├── explain    Explainer: plantilla | LLM OpenAI-compatible
                            ├── dashboard  KPIs
                            ├── auth       login (bcrypt) + JWT + middleware
                            └── db         pool pgx, migraciones embebidas, seed
                                     │
                                     ▼
                                PostgreSQL (datos + cola analysis_runs)
```

Flujo de **Run AI Analysis**: `POST /ai/analyze` (202) encola una fila en `analysis_runs`; un worker la
toma con `FOR UPDATE SKIP LOCKED`, carga datos, corre el motor, redacta las explicaciones y guarda las
anomalías con su evidencia (JSONB). La web consulta `GET /ai/analysis/{id}` y muestra el progreso paso a paso.

### El motor de análisis (`backend/internal/analysis/engine`)

Funciones puras, sin base de datos ni HTTP, 100% testeables y deterministas:

1. **Baseline** por medidor y hora del día: mediana + MAD de los primeros 7 días.
2. **Detectores**: cambio persistente (≥24 h, ±25%), desviación transitoria (z robusto, vuelve a lo normal),
   cambios eléctricos (FP, corriente, voltaje) y calidad de datos (razón física kWh ≈ V·I·FP).
3. **Correlación con eventos** a ±2 h del inicio. `OPERATIONAL_CHANGE` y `SCHEDULED_OUTAGE` explican;
   `UNKNOWN` no.
4. **Clasificación** por reglas en orden: DATA_QUALITY → FALSE_POSITIVE → EXPLAINABLE → REAL.
5. **Confianza y prioridad** con fórmulas documentadas en `score.go`
   (prioridad = severidad × magnitud × confianza).

### Decisiones de diseño

| Decisión | Por qué |
|---|---|
| Monolito modular en Go con `net/http` y `pgx` (SQL plano) | Volumen mínimo: un despliegue simple, sin magia ni ORM |
| Cola de trabajos en PostgreSQL (`SKIP LOCKED`) | Transaccional, sobrevive reinicios y soporta varios workers sin infraestructura extra |
| Estadística robusta + reglas explícitas, sin ML supervisado | 12 medidores y sin etiquetas: un modelo sobreajustaría, y aquí se premia la explicabilidad |
| El LLM solo **redacta**, no decide | Evita alucinaciones: tipo, severidad y cifras salen del motor y son auditables |
| Plantilla determinista de respaldo | La demo nunca depende de la red ni de una clave: el análisis funciona igual con y sin LLM |
| Timestamps sin zona horaria | Los CSV traen hora local de planta; mostrarlos como UTC sería falso |
| Vue 3 + TypeScript + PrimeVue + ECharts | Tablas con filtro/orden y gráficas listas; tema oscuro propio (Voltix) como preset |

### Seguridad

Secretos solo por variables de entorno · contraseña con bcrypt y JWT HS256 con expiración (solo se acepta
HS256) · todas las rutas exigen token salvo `/health` y `/auth/login` (lista blanca) · límite de intentos
de login por IP · SQL siempre parametrizado · validación de parámetros de ruta y de filtros · errores
internos nunca llegan al cliente · CORS con lista blanca y límite de tamaño del body · timeouts del
servidor · contenedores sin privilegios (distroless y nginx-unprivileged) con CSP · la clave del LLM
nunca sale del backend ni se registra en logs.

## LLM opcional

Con `LLM_API_KEY` y `LLM_MODEL` (y `LLM_BASE_URL`, por defecto OpenRouter), las explicaciones las redacta un
modelo compatible con la API de OpenAI a partir de la evidencia del motor. Si falla (red, timeout,
respuesta inválida) se usa la plantilla automáticamente. La pantalla de investigación indica quién
redactó cada explicación.

## Probar con otro dataset

Los CSV se montan en la API como volumen de solo lectura (`./data:/app/data:ro`), así que no hace falta
reconstruir la imagen para cambiar de datos. (El `COPY data/` del Dockerfile queda como respaldo para quien
ejecute la imagen sin Compose.)

1. Reemplaza `data/readings.csv` y `data/events.csv` conservando las mismas columnas y el mismo formato de fecha
   que los originales (`2026-09-01 00:00:00` en lecturas; `2026-09-11 00:00` en eventos).
2. Recarga con una base limpia: `docker compose down -v && docker compose up --build`.
   El `-v` borra el volumen de Postgres: el seed solo carga datos si la base está vacía, así que sin él se
   seguirían viendo los datos anteriores.

Notas:

- **CSV mal formado:** el seed valida el encabezado de forma estricta. Si no coincide, la API **no arranca** y el log dice
  qué se esperaba y qué llegó, en lugar de cargar datos corruptos. Ejemplo:
  `readings.csv: encabezado inesperado: [... consumo ...] (se esperaba [... consumption_kwh ...])`.
- **Umbrales del motor:** los umbrales de detección (z robusto, cambio del 25 %, banda de coherencia física kWh/V·I·FP,
  ventana de baseline de 7 días) están calibrados para el dataset original de 12 medidores durante 14 días. Con un dataset muy
  distinto pueden necesitar ajustes. Las constantes están al inicio de cada detector en `backend/internal/analysis/engine`.

## Desarrollo

```bash
# Backend (Go 1.24)
cd backend
go test ./...                     # los tests de integración se saltan sin TEST_DATABASE_URL
TEST_DATABASE_URL="postgres://postgres:test@127.0.0.1:55433/postgres?sslmode=disable" go test ./...

# Frontend
cd frontend
npm install
npm run dev                       # http://localhost:5173, reenvía /api al backend (VITE_API_PROXY)
npm run build
```

Para los tests de integración levanta un Postgres desechable:
`docker run -d --name energy-ai-testdb -e POSTGRES_PASSWORD=test -p 127.0.0.1:55433:5432 postgres:16-alpine`.

## Documentación de la API (Swagger)

Cada endpoint está explicado en lenguaje sencillo, con ejemplos y botón **Try it out**:

- Con Docker Compose: http://localhost:5173/api/docs/ (o directo en la API: http://localhost:8080/docs/)
- La especificación OpenAPI 3 está en `backend/internal/apidocs/openapi.yaml` y también se sirve en `/openapi.yaml`.

Para probar: **POST /auth/login** → copia el `access_token` → botón **Authorize** → ya puedes ejecutar los demás.
La interfaz va embebida en el binario (sin CDN), así funciona sin internet. La documentación es pública; los endpoints siguen exigiendo token.

## API

Todas requieren `Authorization: Bearer <token>` salvo las dos primeras.

| Método | Ruta | Descripción |
|---|---|---|
| GET | `/health` | Estado de la API y la base |
| POST | `/auth/login` | `{username, password}` → JWT |
| GET | `/meters` | Medidores con consumo, baseline, variación y estado |
| GET | `/meters/{meterId}` | Detalle y eventos |
| GET | `/meters/{meterId}/readings?from=&to=` | Lecturas horarias |
| POST | `/ai/analyze` | Encola un análisis (202) |
| GET | `/ai/analysis/{id}` | Estado y paso actual |
| GET | `/anomalies?meter_id=&type=&severity=&status=` | Anomalías del último análisis, por prioridad |
| GET | `/anomalies/{id}` | Detalle con evidencia |
| PATCH | `/anomalies/{id}` | Cambia el estado (`OPEN`, `ACKNOWLEDGED`, `RESOLVED`) |
| GET | `/dashboard/summary` | KPIs |

## Limitaciones

- Un solo usuario demo; sin registro, roles ni refresh tokens (el token dura `JWT_TTL`, 8 h por defecto).
- El token vive en `localStorage`: práctico para una SPA, pero expuesto si hubiera XSS (la CSP lo mitiga).
- El análisis recorre todo el dataset (14 días, 12 medidores). Con datos en streaming habría que analizar
  por ventanas y guardar el baseline.
- Los umbrales se calibraron con estos datos; con otra flota habría que ajustarlos o aprenderlos.
- No se valida que los números del texto del LLM coincidan con la evidencia (solo forma y longitud);
  por eso el prompt le prohíbe inventar cifras, causas y equipos, y limita la acción a una verificación genérica por tipo; la plantilla es la referencia.
- El límite de intentos de login es en memoria (un solo proceso).

## Siguientes pasos

- Baselines por día de la semana y estacionalidad; umbrales adaptativos por medidor.
- Verificar automáticamente que las cifras del LLM coincidan con la evidencia.
- Notificaciones (correo/Slack) para anomalías de severidad alta.
- Usuarios y roles, y refresh tokens con cookies `HttpOnly`.
- Ingesta continua de lecturas y análisis programado.
