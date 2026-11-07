# La web (SvelteKit). Se construye desde la raíz del repo: necesita el núcleo
# (lo compila a WebAssembly con clang) y el protocolo compartido (sim/web).
FROM node:22-bookworm AS compilar
RUN apt-get update && apt-get install -y --no-install-recommends clang lld && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY web/package.json web/package-lock.json web/
RUN cd web && npm ci --no-audit --no-fund
COPY firmware/nucleo firmware/nucleo
COPY firmware/perfiles/sim firmware/perfiles/sim
COPY firmware/herramientas/fuentes firmware/herramientas/fuentes
COPY sim sim
COPY web web
RUN cd web && npm run build

FROM node:22-bookworm-slim
WORKDIR /app
COPY --from=compilar /src/web/build build
COPY --from=compilar /src/web/package.json package.json
ENV PORT=3000 HOST=0.0.0.0 API_INTERNA=http://api:8080 BODY_SIZE_LIMIT=10M
USER node
EXPOSE 3000
HEALTHCHECK --interval=10s --timeout=3s CMD ["node", "-e", "fetch('http://127.0.0.1:3000/entrar').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"]
CMD ["node", "build"]
