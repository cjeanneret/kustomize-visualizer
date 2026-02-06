# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /kustomap .

# Run stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /kustomap .

EXPOSE 3000
ENV PORT=3000

USER nobody
ENTRYPOINT ["./kustomap"]
