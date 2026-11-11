# syntax=docker/dockerfile:1
FROM golang:1.27.0-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata postgresql-client \
    && addgroup -S dikasa && adduser -S -G dikasa -u 10001 dikasa
WORKDIR /app
COPY --from=build /out/api /app/api
COPY migrations/*.up.sql /app/migrations/
COPY deploy/migrate.sh /app/deploy/migrate.sh
USER 10001:10001
ENV GIN_MODE=release PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/api"]
