# Multi-stage build for the antwort OpenResponses gateway.
#
# Build: podman build -t antwort -f Containerfile .
# Run:   podman run -p 8080:8080 -e ANTWORT_BACKEND_URL=http://host:8000 antwort

FROM docker.io/library/golang:1.25 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /antwort ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /antwort /antwort

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/antwort"]
