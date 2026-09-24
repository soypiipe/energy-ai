# AI Energy Management Platform

MVP que convierte lecturas de medidores eléctricos en decisiones operativas: detecta anomalías,
las explica con evidencia, las prioriza y recomienda una acción.

> En construcción. Diseño y decisiones: [`docs/DESIGN.md`](docs/DESIGN.md).

## Arranque rápido

```bash
cp .env.example .env          # y cambia POSTGRES_PASSWORD
docker compose up --build
curl http://localhost:8080/meters
```

Al primer arranque, la API aplica las migraciones y carga `data/*.csv` (solo si la base está vacía).

## Desarrollo del backend

```bash
cd backend
go mod tidy                   # la primera vez: descarga dependencias y genera go.sum
go test ./...
DATABASE_URL="postgres://energy:<clave>@localhost:5432/energy?sslmode=disable" go run ./cmd/api
```

## API disponible

| Método | Ruta | Descripción |
|---|---|---|
| GET | `/health` | Estado de la API y la base de datos |
| GET | `/meters` | Medidores con consumo actual, baseline, variación y estado |
| GET | `/meters/{meterId}` | Detalle del medidor y sus eventos |
| GET | `/meters/{meterId}/readings?from=&to=` | Lecturas horarias (formato `2026-09-12T14:00:00`) |
