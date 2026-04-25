# ==================== BUILD STAGE ====================
FROM golang:1.23-alpine AS builder

# Install git & ca-certs (diperlukan untuk go mod download & HTTPS calls)
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go.mod & go.sum dulu (layer caching untuk dependencies)
COPY go.mod go.sum ./
RUN go mod download

# Copy seluruh source code
COPY . .

# Build binary — static linked, tanpa CGO
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /app/attesta-be .

# ==================== RUNTIME STAGE ====================
FROM alpine:3.21

# ca-certs diperlukan untuk HTTPS calls ke API external (GitHub, Anthropic, Monad RPC, dll.)
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binary dari build stage
COPY --from=builder /app/attesta-be .

# Port default sesuai config
EXPOSE 4010

# Jalankan binary
ENTRYPOINT ["./attesta-be"]
