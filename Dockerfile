FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api && CGO_ENABLED=0 go build -o /out/nodo ./cmd/nodo

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/ /app/
COPY data/kmeans_clusters.json data/kmeans_puntos_reales.csv /app/data/
