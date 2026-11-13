# syntax=docker/dockerfile:1
FROM node:24-alpine AS web-build
WORKDIR /build/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run check && npm run build && node scripts/precompress.mjs

FROM golang:1.27-alpine AS api-build
WORKDIR /build/api
COPY api/go.mod api/go.sum ./
RUN go mod download
COPY api/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tnr ./cmd/server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S -g 10001 tnr && adduser -S -u 10001 -G tnr tnr && mkdir -p /app/data && chown tnr:tnr /app/data
WORKDIR /app
COPY --from=api-build /out/tnr /app/tnr
COPY --from=web-build /build/web/dist /app/web
ENV FRONTEND_DIR=/app/web DATABASE_PATH=/app/data/tnr.db LISTEN_ADDR=:8081
USER tnr
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/app/tnr", "--healthcheck"]
ENTRYPOINT ["/app/tnr"]
