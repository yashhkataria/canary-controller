FROM golang:1.27.2 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/demo-app ./cmd/demo-app
COPY internal/demoapp ./internal/demoapp
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/demo-app ./cmd/demo-app

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG ERROR_RATE=0
ARG EXTRA_LATENCY_MS=0
ENV VERSION=${VERSION} \
    ERROR_RATE=${ERROR_RATE} \
    EXTRA_LATENCY_MS=${EXTRA_LATENCY_MS}
COPY --from=build /out/demo-app /demo-app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/demo-app"]
