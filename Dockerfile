FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/gamifykit ./cmd/gamifykit-server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /out/gamifykit /app/gamifykit
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/gamifykit"]

