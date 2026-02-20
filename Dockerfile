FROM golang:alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mock-api-server ./main.go

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/mock-api-server /app/mock-api-server
COPY config.yaml /app/config.yaml
COPY config /app/config
COPY mocks /app/mocks
COPY recorded /app/recorded
COPY logs /app/logs

EXPOSE 8080

# CMD ["/app/mock-api-server", "-config", "/app/config.yaml"]
