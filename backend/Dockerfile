# Build multi-etapa: se compila en una imagen con Go y se ejecuta en una imagen mínima.
# Contexto de build: la raíz del repo (para incluir /data). Ver docker-compose.yml.

FROM golang:1.24-alpine AS build
WORKDIR /src
# Primero solo go.mod/go.sum: la capa de dependencias queda en caché si el código cambia.
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# distroless: sin shell ni gestor de paquetes (menor superficie de ataque), usuario no-root.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/api /app/api
COPY data/ /app/data/
ENV DATA_DIR=/app/data PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
