# ---- 前端构建 ----
FROM node:22-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ---- 后端编译 ----
FROM golang:1.26-alpine AS backend-build
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/guangyanji-server ./cmd/server

# ---- 运行时 ----
FROM alpine:3.20
RUN apk add --no-cache tzdata ca-certificates
WORKDIR /app
COPY --from=backend-build /out/guangyanji-server ./guangyanji-server
COPY --from=frontend-build /app/frontend/dist ./dist
ENV PORT=8082 \
    FRONTEND_DIST=/app/dist \
    UPLOAD_DIR=/app/uploads \
    GIN_MODE=release
EXPOSE 8082
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8082/healthz >/dev/null 2>&1 || exit 1
CMD ["./guangyanji-server"]
