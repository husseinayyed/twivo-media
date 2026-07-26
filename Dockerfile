# --- Builder Stage ---
FROM golang:1.26.5-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/main .

# --- Development Stage ---
FROM golang:1.26.5-alpine AS dev
WORKDIR /app
RUN apk --no-cache add git bash curl
COPY go.mod go.sum ./
RUN go mod download
COPY . .
EXPOSE 8020

CMD ["tail", "-f", "/dev/null"]

# --- Runtime / Production Stage ---
FROM alpine:3.21
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/main .
RUN adduser -D -u 1001 appuser && chown -R appuser:appuser /app
USER appuser

EXPOSE 8020

CMD ["./app"]