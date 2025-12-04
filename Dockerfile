# FROM golang:1.24

# WORKDIR /app

# COPY go.mod go.sum ./

# RUN go mod download

# COPY . .

# RUN CGO_ENABLED=0 GOOS=linux go build -o bot

# CMD ["/app/bot"]

FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bot .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/bot .

ENTRYPOINT ["/app/bot"]