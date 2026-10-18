FROM golang:1.27.2 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/controller ./cmd/controller
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/controller ./cmd/controller

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/controller /controller
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/controller"]
