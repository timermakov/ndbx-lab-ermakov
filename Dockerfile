FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY go.mod ./
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app cmd/app/main.go

FROM scratch
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
