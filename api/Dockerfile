# La API y el bicho virtual (los dos usan el núcleo de C++ por cgo).
# Se construye desde la raíz del repo: docker build -f api/Dockerfile .
FROM golang:1.25-bookworm AS compilar
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY firmware/nucleo firmware/nucleo
COPY firmware/api firmware/api
COPY shared shared
COPY api api
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /out/sintetica-api ./api/cmd/sintetica-api && \
    CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /out/bicho-virtual ./api/cmd/bicho-virtual

FROM debian:bookworm-slim AS base
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libstdc++6 && rm -rf /var/lib/apt/lists/*
RUN useradd --system --uid 10001 --home /nonexistent sintetica

FROM base AS api
COPY --from=compilar /out/sintetica-api /usr/local/bin/sintetica-api
USER sintetica
EXPOSE 8080 8081
HEALTHCHECK --interval=10s --timeout=3s --start-period=20s CMD ["sintetica-api", "salud"]
ENTRYPOINT ["sintetica-api"]

FROM base AS bicho-virtual
COPY --from=compilar /out/bicho-virtual /usr/local/bin/bicho-virtual
RUN mkdir /datos && chown sintetica /datos
USER sintetica
VOLUME /datos
EXPOSE 8090
ENTRYPOINT ["bicho-virtual"]
