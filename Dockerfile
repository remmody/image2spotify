FROM golang:1.25.1-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o bot ./cmd/bot

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/bot .

RUN adduser -D -u 1000 botuser && \
    chown -R botuser:botuser /app

USER botuser

CMD ["./bot"]
