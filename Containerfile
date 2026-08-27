# =====================================================================
# STAGE 1: Build Frontend Assets via Vite
# =====================================================================
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend

# Copy lockfiles first into the exact subfolder to leverage caching
COPY frontend/package.json frontend/yarn.lock* frontend/package-lock.json* ./
RUN yarn install --frozen-lockfile

COPY frontend/ ./
RUN yarn build

# =====================================================================
# STAGE 2: Build Go Backend Binary
# =====================================================================
FROM golang:1.26-alpine AS backend-builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY sonary.go ./
COPY .env .env.prod ./
COPY internal/ ./internal/
COPY templates/ ./templates/
COPY utils/ ./utils/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o sonary sonary.go

# =====================================================================
# STAGE 3: Final Lightweight Runtime Environment
# =====================================================================
FROM alpine:latest
WORKDIR /app

RUN apk add --no-cache tzdata ffmpeg

COPY --from=backend-builder /app/sonary .
COPY --from=backend-builder /app/.env /app/.env.prod .
COPY --from=frontend-builder /app/static ./static
COPY templates/ ./templates/

EXPOSE 3101
ENV APP_ENV=prod
CMD ["./sonary"]
