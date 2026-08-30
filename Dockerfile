# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Download dependencies first for caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/server

# Run stage
FROM alpine:3.20

WORKDIR /app

# Add ca-certificates and tzdata
RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/server /app/server
COPY --from=builder /app/migrations /app/migrations

EXPOSE 8080

CMD ["/app/server"]

