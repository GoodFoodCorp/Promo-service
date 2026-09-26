FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o promo-service ./cmd/main.go

FROM gcr.io/distroless/static-debian12
WORKDIR /
COPY --from=builder /app/promo-service /promo-service
USER nonroot:nonroot
EXPOSE 8092
ENTRYPOINT ["/promo-service"]
