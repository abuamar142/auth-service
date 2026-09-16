FROM golang:1.26-alpine AS builder

RUN apk --no-cache add git
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /auth ./cmd/auth

FROM alpine:3.20

RUN apk --no-cache add ca-certificates
RUN adduser -D -u 1001 appuser
COPY --from=builder /auth /auth
USER appuser
EXPOSE 8080
ENTRYPOINT ["/auth"]
