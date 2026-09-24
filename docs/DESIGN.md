# Diseño — AI Energy Management Platform

## 1. Problema

El producto es una **herramienta de triage**, no un CRUD. Un operador de energía necesita saber
en segundos **qué medidor revisar primero, por qué y qué hacer**.

Ciclo a demostrar: `DATOS → ANÁLISIS → ANOMALÍA → EXPLICACIÓN → PRIORIZACIÓN → ACCIÓN`.

### Criterios de aceptación (de la sección 9 del enunciado)

| Medidor | Tipo esperado        | Severidad | Evidencia clave                                                        |
|---------|----------------------|-----------|------------------------------------------------------------------------|
| M-109   | `REAL_ANOMALY`       | HIGH (1.º)| +100% desde 12-sep 14:00, corriente ×2, FP 0,94→0,74, sin evento que lo explique |
| M-112   | `DATA_QUALITY`       | HIGH      | kWh estable, pero V/I/FP físicamente incoherentes desde 13-sep         |
| M-104   | `EXPLAINABLE_ANOMALY`| MEDIUM    | +47% desde 11-sep 00:00, coincide con `OPERATIONAL_CHANGE`             |
| M-106   | `FALSE_POSITIVE`     | LOW       | Caída de 12 h el 8-sep, coincide con `SCHEDULED_OUTAGE`                |
| Resto (8)| —                   | —         | **Cero anomalías** (prueba de falsos positivos)                        |

## 2. Hallazgos de los datos

- 12 medidores × 336 horas (1–14 sep 2026), sin nulos ni duplicados.
- Hay un **patrón horario** (noche ≈ 23 kWh, día ≈ 36 kWh), así que el baseline se calcula **por hora del día**.
- La columna `status` siempre dice `OK`, incluso cuando los datos son malos. **No se usa como señal.**
- Física: `kWh ≈ V·I·FP / 1000`. En medidores sanos, la razón está en **~1,06 ± 0,06**. En M-112 va de 0,58 a 4,2.
- `events.csv` tiene un evento `UNKNOWN` ("No operational event reported") para M-109. **No explica nada.**
- Los números de ejemplo del enunciado son ilustrativos: no coinciden exactamente con los datos.
- Supuesto: los timestamps del CSV están en hora local de la planta y se guardan como `TIMESTAMP` sin zona.

## 3. Decisiones de arquitectura (ADR resumidos)

| # | Decisión | Por qué | Alternativa descartada |
|---|----------|---------|------------------------|
| 1 | Monolito modular en Go, organizado por funcionalidad | El volumen es mínimo, y un solo despliegue es simple de operar y demostrar | Microservicios: complejidad sin beneficio |
| 2 | `net/http` de la librería estándar (Go 1.22+ enruta por método y ruta) | Cero dependencias y nada de magia | Gin/Echo: aportan poco aquí |
| 3 | PostgreSQL + `pgx` con SQL plano, sin ORM | Consultas explícitas y parametrizadas (evitan inyección SQL) | GORM: oculta el SQL |
| 4 | **Cola de trabajos en PostgreSQL** (`analysis_runs` + `FOR UPDATE SKIP LOCKED`) | Desacopla, es transaccional, sobrevive reinicios y soporta varios workers, sin infraestructura extra | RabbitMQ/SQS: se justifican a otra escala; el worker se puede cambiar sin tocar el motor |
| 5 | Motor de análisis = **funciones puras** (sin BD ni HTTP) | 100% testeable y determinista | Lógica dentro de handlers |
| 6 | Única interfaz: `Explainer` (LLM o plantilla) — patrón Strategy | Existen dos implementaciones reales y la demo nunca depende de la red | Abstraer todo "por si acaso" |
| 7 | El LLM solo **redacta** a partir de la evidencia; no calcula ni decide la severidad | Evita alucinaciones; la clasificación queda auditable | LLM como clasificador |
| 8 | Detección estadística robusta + reglas explícitas, **sin ML supervisado** | 12 medidores y sin etiquetas: un modelo supervisado sobreajustaría; además se premia la explicabilidad | Isolation Forest / clasificador |
| 9 | Frontend: Vue 3 + TS + Vite + PrimeVue + ECharts | Tablas con filtro, orden y búsqueda listas; aspecto de SaaS rápido | UI hecha a mano |
| 10 | Auth: un usuario demo (hash bcrypt por variable de entorno) + JWT | Cumple el flujo de login con buenas prácticas y sin un sistema de usuarios | OAuth / tabla de usuarios |

## 4. Componentes

```
Vue SPA ──HTTP/JSON──► API Go (net/http)
                          ├── meter      (lectura de medidores y lecturas)
                          ├── analysis   (API + worker de la cola + motor puro)
                          ├── llm        (Explainer: OpenAI-compatible | plantilla)
                          └── db         (pgx pool, migraciones embebidas, seed)
                                   │
                                   ▼
                              PostgreSQL (datos + cola analysis_runs)
```

## 5. Modelo de datos

- `meters(id, meter_id UNIQUE, name, location, status, created_at)`
- `readings(id, meter_id FK, ts, consumption_kwh, voltage_v, current_a, power_factor, status)` — `UNIQUE(meter_id, ts)`
- `events(id, meter_id FK, ts, type, description)`
- `analysis_runs(id UUID, status PENDING|RUNNING|COMPLETED|FAILED, current_step, error, summary JSONB, created_at, started_at, finished_at)` ← **es la cola**
- `anomalies(id UUID, analysis_id FK, meter_id FK, detected_at, type, severity, confidence, priority_score, reason, recommended_action, evidence JSONB, status OPEN|ACKNOWLEDGED|RESOLVED, created_at)`

## 6. Pipeline de análisis (sección 13)

1. **Baseline:** por medidor y hora del día, la mediana y el MAD de los días 1–7.
2. **Detección** de hallazgos: cambio persistente (24 h, ±25%), desviación transitoria (z robusto alto que luego vuelve a lo normal), cambio eléctrico (FP, corriente, voltaje) y calidad de datos (razón física y valores imposibles o repetidos).
3. **Correlación** con eventos en una ventana de ±2 h del inicio. `OPERATIONAL_CHANGE` y `SCHEDULED_OUTAGE` explican el cambio; `UNKNOWN` no.
4. **Clasificación** con reglas en orden: DATA_QUALITY → FALSE_POSITIVE → EXPLAINABLE → REAL.
5. **Confianza y prioridad:** fórmulas deterministas; prioridad = severidad × magnitud × confianza.
6. **Explicación:** el `Explainer` devuelve `reason` y `recommended_action`, validados antes de guardarse.

## 7. API

`GET /health` · `GET /meters` · `GET /meters/{meterId}` · `GET /meters/{meterId}/readings?from&to` ·
`GET /anomalies` · `GET /anomalies/{id}` · `POST /ai/analyze` (202) · `GET /ai/analysis/{id}` ·
`GET /dashboard/summary` · `POST /auth/login`

## 8. Seguridad

Secretos solo por variables de entorno · SQL parametrizado · validación de parámetros de ruta ·
CORS con lista blanca · timeouts del servidor HTTP · límite de tamaño del body · la key del LLM nunca llega al
frontend · la salida del LLM se valida · el contenedor corre como usuario no-root (distroless).
